package pgdriver_go

import (
	"database/sql"
	"database/sql/driver"
	"net"
	"encoding/binary"
	"fmt"
	"log"
	"bytes"
	"errors"
	"io"
)

const (
	success    = iota //0
	_                 //1
	KerberosV5
	PlaintText
	_           //4
	Md5
	SCM
	GSSAPI
	xSSxData    //GSSAPI & SSPI data
	SSPI
)

type PGDriver struct {
	user, password, database string
}

func (d *PGDriver) Open(name string) (driver.Conn, error) {
	dial, err := net.Dial("tcp", name)
	if err != nil {
		log.Fatal(err)
	}

	var buf bytes.Buffer
	buf.Write([]byte{0x00, 0x03, 0x00, 0x00})
	buf.Write([]byte("user"))
	buf.WriteByte(0x00)
	buf.Write([]byte(d.user))
	buf.WriteByte(0x00)
	buf.Write([]byte("database"))
	buf.WriteByte(0x00)
	buf.Write([]byte(d.database))
	buf.WriteByte(0x00)
	buf.Write([]byte("application_name"))
	buf.WriteByte(0x00)
	buf.Write([]byte("jinPGDriver"))
	buf.WriteByte(0x00)
	buf.WriteByte(0x00)

	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(buf.Len()+4))
	var payload bytes.Buffer
	payload.Write(length)
	payload.Write(buf.Bytes())
	dial.Write(payload.Bytes())

	var result = make([]byte, 1024)
	n, err := dial.Read(result)
	if err != nil {
		return nil, err
	}

	if result[0] == 0x52 { //auth response 'R'
		le := binary.BigEndian.Uint32(result[1:5])
		code := binary.BigEndian.Uint32(result[5:le+1])
		if code != success { //auth failed
			return nil, errors.New("auth failed.")
		}
		fmt.Println("auth success.")
	}
	//todo other authtication type

	conn := new(PGConn)
	conn.conn = dial
	point := 0
	var pid, key uint32
	var flen uint32
	for point < n {
		flen = binary.BigEndian.Uint32(result[point+1:point+1+4])
		content := result[point+1+4:point+1+int(flen)]

		// property messages
		/*		for index, v := range content {
					if result[point] == 0x52 {
						fmt.Println("auth success.")
						break
					}
					if v == 0x00 {
						if content[index] == 0x00 {
							fmt.Printf("%s: %s\n", string(content[:index]), string(content[index+1:flen-5]))
							break
						}
					}
				}*/

		if result[point] == 0x4b { //information about pid and key
			pid = binary.BigEndian.Uint32(content[:4])
			key = binary.BigEndian.Uint32(content[4:])
			for i := 0; i < 4; i++ {
				conn.pid[i] = content[i]
				conn.key[i] = content[4+i]
			}
			fmt.Printf("pid = %d, key = %d\n", pid, key)
		}
		if result[point] == 0x5a { //ready for query
			conn.ready = true
		}

		point += 1 + int(flen)

	}
	return conn, nil
}

type PGConn struct {
	conn     net.Conn
	pid, key [4]byte
	ready    bool
}

func (conn *PGConn) Prepare(query string) (driver.Stmt, error) {
	stmt := new(PGStmt)
	var parse bytes.Buffer

	for _, v := range query {
		if v == '$' {
			stmt.parameters++
		}
	}

	//parse
	parse.WriteByte(0x50) //parse
	sql := []byte(query)
	lt := make([]byte, 4)
	binary.BigEndian.PutUint32(lt, uint32(8+len(sql))+4*uint32(stmt.parameters)) // le:4 statment:1 sql:sql+1 parameters:2 params*4
	parse.Write(lt)                                                              //length
	parse.WriteByte(0x00)
	parse.Write(sql)
	parse.WriteByte(0x00)

	params := make([]byte, 2)
	binary.BigEndian.PutUint16(params, stmt.parameters)
	parse.Write(params) //parameters

	for i := uint16(0); i < stmt.parameters; i++ {
		parse.Write([]byte{0x00, 0x00, 0x00, 0x17})
	}
	stmt.parse = parse.Bytes()

	stmt.conn = conn

	return stmt, nil
}

func (conn *PGConn) Close() error {
	return conn.conn.Close()
}

func (conn *PGConn) Begin() (driver.Tx, error) {
	tx := new(PGTx)
	tx.conn = conn

	conn.conn.Write(getTemplate("BEGIN"))

	//result todo

	return tx, nil
}

type PGTx struct {
	conn *PGConn
}

func (tx *PGTx) Commit() error {
	tx.conn.conn.Write(getTemplate("COMMIT"))
	//result todo
	return nil
}
func (tx *PGTx) Rollback() error {
	tx.conn.conn.Write(getTemplate("ROLLBACK"))
	//result todo
	return nil
}

type PGStmt struct {
	conn       *PGConn
	parameters uint16 //number of parameters
	parse      []byte //parse message
	bind       []byte //bind message
	describe   []byte //describe message
	ready      bool   //if ready for next statment
}

func (stmt *PGStmt) Close() error {
	var err error
	if !stmt.ready { //busy
		err = stmt.cancel()
	} else {
		err = nil
	}
	stmt = nil
	return err
}
func (stmt *PGStmt) NumInput() int {
	return 0
}
func (stmt *PGStmt) prepar(args []driver.Value) ([]byte, error) { //for EXEC & Query
	var bind, describe bytes.Buffer
	//bind
	bind.WriteByte(0x42)

	parameters := stmt.parameters
	var bindLen uint32
	var format, value bytes.Buffer
	format.Write([]byte{0x00, 0x01})
	format.Write([]byte{0x00, 0x01})

	l := make([]byte, 2)
	binary.BigEndian.PutUint16(l, parameters)
	value.Write(l)
	for i := uint16(0); i < parameters; i++ {
		arg := args[i]
		var buf bytes.Buffer
		err := binary.Write(&buf, binary.BigEndian, arg)
		if err != nil {
			return nil, err
		}
		bytes := buf.Bytes()
		le := make([]byte, 4)
		binary.BigEndian.PutUint32(le, uint32(len(bytes)))

		value.Write(le)
		value.Write(bytes)

		bindLen += uint32(len(bytes))
	}
	bindLen += 14
	le := make([]byte, 4)
	binary.BigEndian.PutUint32(le, bindLen)

	bind.Write(le)             //length
	bind.WriteByte(0x00)       //portal
	bind.WriteByte(0x00)       //statment
	bind.Write(format.Bytes()) //format(mats,mat)
	bind.Write(value.Bytes())  //values

	bind.Write([]byte{0x00, 0x00})

	stmt.bind = bind.Bytes()

	//describe
	describe.WriteByte(0x44)
	describe.Write([]byte{0x00, 0x00, 0x00, 0x06}) //length
	describe.WriteByte(0x50)                       //50
	describe.WriteByte(0x00)                       //portal
	stmt.describe = describe.Bytes()

	var exec bytes.Buffer
	exec.WriteByte(0x45)                       //execute
	exec.Write([]byte{0x00, 0x00, 0x00, 0x09}) //length
	exec.WriteByte(0x00)                       //portal
	exec.Write([]byte{0x00, 0x00, 0x00, 0x00}) // all rows

	var sync bytes.Buffer
	sync.WriteByte(0x53) //sync
	sync.Write([]byte{0x00, 0x00, 0x00, 0x04})

	var payload bytes.Buffer
	payload.Write(stmt.parse)
	payload.Write(stmt.bind)
	payload.Write(stmt.describe)
	payload.Write(exec.Bytes())
	payload.Write(sync.Bytes())

	return payload.Bytes(), nil
}

func (stmt *PGStmt) Exec(args []driver.Value) (driver.Result, error) {
	if len(args) != int(stmt.parameters) {
		return nil, errors.New("parameters are ivalide.")
	}
	result := new(PGResult)

	payload, err := stmt.prepar(args)
	if err != nil {
		return nil, err
	}
	stmt.conn.conn.Write(payload)
	//todo
	return result, nil
}
func (stmt *PGStmt) Query(args []driver.Value) (driver.Rows, error) {
	rows := new(PGRows)

	payload, err := stmt.prepar(args)
	if err != nil {
		return nil, err
	}

	conn := stmt.conn.conn
	conn.Write(payload)

	var result bytes.Buffer

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		result.Reset()
	} else {
		result.Write(buf[:n])
	}

	response := result.Bytes()
	offset := 0
	for offset < n {
		le := binary.BigEndian.Uint32(response[offset+1:offset+5])
		switch response[offset] {
		case 0x31: //parse completion
			fallthrough
		case 0x32: //bind completion
			fallthrough
		case 0x43: //command completion //todo tag
			fallthrough
		case 0x5a: //ready for query 0x49:idle 0x54:is transaction
			fallthrough
		case 0x6e: //no data
			offset += 1 + int(le)
		case 0x54: //row description
			num := binary.BigEndian.Uint16(response[offset+5:offset+7])
			fmt.Println("colomns:", num)
			var columns = make([]string, num)
			content := response[offset+7:offset+int(le)]
			start, end := offset+7, offset+len(content)
			i := uint16(0)
			for start < end && i < num {
				index := bytes.IndexByte(response[start:end], 0x00)
				index += start
				columns[i] = string(response[start:index])
				/*fmt.Printf("name: %s, tid:%d, cid:%d, typeid:%d, len:%d, modi:%d, format:%d\n",
					string(response[start:index]),
					binary.BigEndian.Uint32(response[index+1:index+1+4]),
					binary.BigEndian.Uint16(response[index+1+4:index+1+6]),
					binary.BigEndian.Uint32(response[index+1+6:index+1+10]),
					binary.BigEndian.Uint16(response[index+1+10:index+1+12]),
					binary.BigEndian.Uint32(response[index+1+12:index+1+16]),
					binary.BigEndian.Uint16(response[index+1+16:index+1+18]))*/
				start = index + 1 + 18
				i++
			}
			offset = start
			rows.columns = columns
		case 0x44: //data row
			//print data
			num := binary.BigEndian.Uint16(response[offset+5:offset+7])
			var data = make([]driver.Value, num)
			pt := offset + 7
			for i := 0; i < int(num); i++ {
				t := response[pt:pt+4]
				clen := binary.BigEndian.Uint32(t)
				if clen == 4294967295 { //clen = 0xff 0xff 0xff 0xff (-1)
					pt += 4
					continue
				}
				x := pt + 4 + int(clen)
				data[i] = string(response[pt+4:x])
				pt = x
			}
			offset = pt
			rows.Data = append(rows.Data, data)

		}

	}

	return rows, nil
}

func (stmt *PGStmt) cancel() error {
	var payload bytes.Buffer
	payload.WriteByte('F')                        //cancel request
	payload.Write([]byte{0x00, 0x00, 0x00, 0x10}) //length
	payload.Write([]byte{0x04, 0xd2, 0x16, 0x2e})
	payload.Write(stmt.conn.pid[:])
	payload.Write(stmt.conn.key[:])

	stmt.conn.conn.Write(payload.Bytes())
	return nil
}

func getTemplate(command string) []byte {
	//parse
	var parse bytes.Buffer
	parse.WriteByte(0x50)                       //parse
	parse.Write([]byte{0x00, 0x00, 0x00, 0x0d}) //length
	parse.WriteByte(0x00)                       //statement
	parse.Write([]byte(command))
	parse.WriteByte(0x00)
	parse.WriteByte(0x00)
	parse.WriteByte(0x00)

	var bind, describe bytes.Buffer
	//bind
	bind.WriteByte(0x42)

	var bindLen uint32
	var format, value bytes.Buffer
	format.Write([]byte{0x00, 0x01})
	value.Write([]byte{0x00, 0x01})

	bindLen += 14
	le := make([]byte, 4)
	binary.BigEndian.PutUint32(le, bindLen)

	bind.Write(le)             //length
	bind.WriteByte(0x00)       //portal
	bind.WriteByte(0x00)       //statment
	bind.Write(format.Bytes()) //format(mats,mat)
	bind.Write(value.Bytes())  //values

	bind.Write([]byte{0x00, 0x00}) //result format

	//describe
	describe.WriteByte(0x44)
	describe.Write([]byte{0x00, 0x00, 0x00, 0x06}) //length
	describe.WriteByte(0x50)                       //50
	describe.WriteByte(0x00)                       //portal

	var exec bytes.Buffer
	exec.WriteByte(0x45)                       //execute
	exec.Write([]byte{0x00, 0x00, 0x00, 0x09}) //length
	exec.WriteByte(0x00)                       //portal
	exec.Write([]byte{0x00, 0x00, 0x00, 0x00}) // all rows

	var sync bytes.Buffer
	sync.WriteByte(0x53) //sync
	sync.Write([]byte{0x00, 0x00, 0x00, 0x04})

	var payload bytes.Buffer
	payload.Write(parse.Bytes())
	payload.Write(bind.Bytes())
	payload.Write(describe.Bytes())
	payload.Write(exec.Bytes())
	payload.Write(sync.Bytes())

	return payload.Bytes()
}

type PGResult struct {
}

func (r *PGResult) LastInsertId() (int64, error) {
	return 0, nil
}
func (r *PGResult) RowsAffected() (int64, error) {
	return 0, nil
}

type PGRows struct {
	columns []string
	counts  uint32

	Data  []([]driver.Value)
	index int
}

func (r *PGRows) Columns() []string {
	return r.columns
}
func (r *PGRows) Close() error {
	//todo
	return nil
}
func (r *PGRows) Next(dest []driver.Value) error {
	if r.index >= len(r.Data) {
		return errors.New("has no more data.")
	}
	src := r.Data[r.index]
	for i, v := range src {
		dest[i] = v
	}
	r.index++
	return nil
}

//init
func init() {
	sql.Register("postgres", &PGDriver{})
}
