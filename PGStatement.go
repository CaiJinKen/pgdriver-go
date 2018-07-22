package pgdriver_go

import (
	"bytes"
	"database/sql/driver"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

type PGStmt struct {
	conn       *PGConn
	parameters uint16 //number of parameters
	parse      []byte //parse message
	bind       []byte //bind message
	describe   []byte //describe message
	ready      bool   //if ready for next statement
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
	format.Write([]byte{ZeroByte, 0x01})
	format.Write([]byte{ZeroByte, 0x01})

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
		bindLen += 4
		bindLen += uint32(len(bytes))
	}
	bindLen += 14
	le := make([]byte, 4)
	binary.BigEndian.PutUint32(le, bindLen)

	bind.Write(le)             //length
	bind.WriteByte(ZeroByte)   //portal
	bind.WriteByte(ZeroByte)   //statment
	bind.Write(format.Bytes()) //format(mats,mat)
	bind.Write(value.Bytes())  //values

	bind.Write([]byte{ZeroByte, ZeroByte})

	stmt.bind = bind.Bytes()

	//describe
	describe.WriteByte(0x44)
	describe.Write([]byte{ZeroByte, ZeroByte, ZeroByte, 0x06}) //length
	describe.WriteByte(0x50)                                   //50
	describe.WriteByte(ZeroByte)                               //portal
	stmt.describe = describe.Bytes()

	var exec bytes.Buffer
	exec.WriteByte(0x45)                                       //execute
	exec.Write([]byte{ZeroByte, ZeroByte, ZeroByte, 0x09})     //length
	exec.WriteByte(ZeroByte)                                   //portal
	exec.Write([]byte{ZeroByte, ZeroByte, ZeroByte, ZeroByte}) // all rows

	var sync bytes.Buffer
	sync.WriteByte(0x53) //sync
	sync.Write([]byte{ZeroByte, ZeroByte, ZeroByte, 0x04})

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
		return nil, errors.New(ParamErr)
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

	stmt.conn.payload = append(stmt.conn.payload, payload...)
	conn := stmt.conn.conn
	conn.Write(stmt.conn.payload)

	var result bytes.Buffer

	var n int
	for { //read all data rows
		buf := make([]byte, 4096)
		bLen, err := conn.Read(buf)
		if err != nil && err != io.EOF {
			result.Reset()
		} else {
			n += bLen
			result.Write(buf[:bLen])
			if buf[0] == 0x45 { //error
				err = errors.New(QueryErr)
			}
			if bLen > 5 && buf[bLen-6] == 0x5a { //ready for query
				break
			}
		}
	}
	if err != nil {
		conn.Close()
		fmt.Println(err)
		os.Exit(3)
	}

	response := result.Bytes()
	offset := 0
	var num uint16
	for offset < n {
		le := binary.BigEndian.Uint32(response[offset+1 : offset+5])
		if response[offset] != 0x5a {
			num = binary.BigEndian.Uint16(response[offset+5 : offset+7])
		}
		switch response[offset] {
		case 0x31: //parse completion
			fallthrough
		case 0x32: //bind completion
			fallthrough
		case 0x43: //command completion //todo tag
			fallthrough
		case 0x5a: //ready for query 0x49:idle 0x54:is transaction
			stmt.ready = true
			fallthrough
		case 0x6e: //no data
			offset += 1 + int(le)
		case 0x54: //row description
			fmt.Println("colomns:", num)
			var columns = make([]string, num)
			content := response[offset+7 : offset+int(le)]
			start, end := offset+7, offset+len(content)
			i := uint16(0)
			for start < end && i < num {
				index := bytes.IndexByte(response[start:end], ZeroByte)
				index += start
				columns[i] = string(response[start:index])
				start = index + 1 + 18
				i++
			}
			offset = start
			rows.columns = columns
		case 0x44: //data row
			//print data
			var data = make([]driver.Value, num)
			pt := offset + 7
			for i := 0; i < int(num); i++ {
				t := response[pt : pt+4]
				clen := binary.BigEndian.Uint32(t)
				if clen == 4294967295 { //clen = 0xff 0xff 0xff 0xff (-1)
					pt += 4
					continue
				}
				x := pt + 4 + int(clen)
				data[i] = string(response[pt+4 : x])
				pt = x
			}
			offset = pt
			rows.Data = append(rows.Data, data)

		case 0x45: //error
			return nil, errors.New(QueryErr)

		}

	}

	return rows, nil
}

func (stmt *PGStmt) cancel() error {
	var payload bytes.Buffer
	payload.WriteByte('F')                                    //cancel request
	payload.Write([]byte{ZeroByte, ZeroByte, ZeroByte, 0x10}) //length
	payload.Write([]byte{0x04, 0xd2, 0x16, 0x2e})
	payload.Write(stmt.conn.pid[:])
	payload.Write(stmt.conn.key[:])

	stmt.conn.conn.Write(payload.Bytes())
	return nil
}
