package pgdriver_go

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os/user"
	"strings"
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
const (
	ZeroByte = 0x00
)

type PGDriver struct {
	properties map[string]string
	cons       []*PGConn
	protocol   []byte
	minCon     int
	maxCon     int
	parsed     bool
}

func (d *PGDriver) parseUrl(name string) {
	url, err := url.Parse(name)
	if err != nil {
		log.Fatal(UrlErr)
	}
	if protocal := url.Scheme; protocal == "postgres" || protocal == "postgresql" {

	} else {
		log.Fatal(ProtocolErr)
	}
	prp := make(map[string]string)
	//username & password
	if users := url.User; users == nil { //user current user
		cu, err := user.Current() //only for *nux
		if err != nil {
			log.Fatal("Get postgres username failed.")
		}
		prp["user"] = cu.Username
	} else {
		prp["user"] = users.Username()
		pass, set := users.Password()
		if set {
			prp["pass"] = pass
		}
	}
	//host & ip
	if host := url.Host; strings.Contains(host, ":") {
		remoteAdd := strings.Split(host, ":")
		prp["ip"] = remoteAdd[0]
		prp["port"] = remoteAdd[1]
	} else {
		prp["ip"] = host
		prp["port"] = "5432"
	}
	// database if not set then postgres
	if database := url.Path; database != "" {
		prp["database"] = strings.TrimPrefix(database, "/")
	} else {
		prp["database"] = "postgres"
	}

	// set default properties
	prp["application_name"] = "pgdriver-go"

	//other properties
	prps := url.Query()
	for key, v := range prps {
		prp[key] = v[0]
	}

	d.properties = prp
	d.parsed = true

}

func (d *PGDriver) Open(name string) (driver.Conn, error) {

	//use connection pool
	if d.parsed {
		for _, conn := range d.cons {
			if conn.closed {
				return conn, nil
			}
		}
	}

	//set connection properties
	d.parseUrl(name)

	pro := d.properties
	fmt.Println(pro)
	addr := d.properties["ip"] + ":" + d.properties["port"]
	dial, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	d.protocol = []byte{0x00, 0x03, 0x00, 0x00}

	var buf bytes.Buffer
	buf.Write(d.protocol)
	buf.Write([]byte("user"))
	buf.WriteByte(ZeroByte)
	buf.Write([]byte(d.properties["user"]))
	buf.WriteByte(ZeroByte)
	buf.Write([]byte("database"))
	buf.WriteByte(ZeroByte)
	buf.Write([]byte(d.properties["database"]))
	buf.WriteByte(ZeroByte)
	buf.Write([]byte("application_name"))
	buf.WriteByte(ZeroByte)
	buf.Write([]byte("jinPGDriver"))
	//optional TimeZone client_encoding etc.
	buf.WriteByte(ZeroByte)
	buf.WriteByte(ZeroByte)

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
		code := binary.BigEndian.Uint32(result[5: le+1])
		if code != success { //auth failed
			return nil, errors.New(AuthErr)
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
		flen = binary.BigEndian.Uint32(result[point+1: point+1+4])
		content := result[point+1+4: point+1+int(flen)]

		//set connection properties
		if result[point] == 0x53 {
			prp := d.properties
			for index, v := range content {
				if v == ZeroByte {
					prp[string(content[:index])] = string(content[index+1: flen-5])
					break
				}
			}
		}

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

	//add to connection poll
	d.cons = append(d.cons, conn)
	return conn, nil
}

// start up message is the first message to postgresql server when application start up
// it contains some client properties such as client_encoding datestyle ticolumn_2meZone
func (dr *PGDriver) startUp() {

}
func isBool(s string) bool {
	if "on" == s || "true" == s || "1" == s {
		return true
	}
	return false
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

func getParseStr(stmt,query string) []byte{
	var parameters uint16
	for _, v := range query {
		if v == '$' {
			parameters++
		}
	}
	return getParse(stmt,query,parameters)
}

func getParse(stmt, query string, params uint16) []byte {
	//parse
	var parse, stmtBuf, queryBuf,paraTypes bytes.Buffer
	var parseLen uint32 = 6 //length+params
	parse.WriteByte(0x50)   //parse

	if stmt != "" {
		stmtBuf.Write([]byte(stmt))
	}
	stmtBuf.WriteByte(ZeroByte)
	parseLen += uint32(stmtBuf.Len())

	if query != "" {
		queryBuf.Write([]byte(query))
	} else{

	}
	queryBuf.WriteByte(ZeroByte)

	for i := uint16(0); i < params; i++ {
		paraTypes.Write([]byte{0x00, 0x00, 0x00, 0x17})
	}

	parseLen += uint32(queryBuf.Len())
	parseLen += uint32(paraTypes.Len())

	parse.Write(getUint32Byte(parseLen)) //length
	parse.Write(stmtBuf.Bytes())         //statement
	parse.Write(queryBuf.Bytes())        //query
	parse.Write(getUint16Byte(params))   //parameters
	parse.Write(paraTypes.Bytes())

	return parse.Bytes()
}

func getBind(portal, stmt string) []byte {
	//bind
	var bind bytes.Buffer
	bind.WriteByte(0x42)

	var bindLen uint32 = 4 //length`s length
	var portalBuf, stmtBuf, format, value bytes.Buffer
	format.Write([]byte{ZeroByte, 0x01})
	value.Write([]byte{ZeroByte, 0x01})
	bindLen += 4 //format+value

	if portal != "" {
		portalBuf.Write([]byte(portal))
	} else {
		portalBuf.WriteByte(ZeroByte)
	}

	if stmt != "" {
		stmtBuf.Write([]byte(stmt))
	} else {
		stmtBuf.WriteByte(ZeroByte)
	}

	bindLen += uint32(portalBuf.Len())
	bindLen += uint32(stmtBuf.Len())
	bindLen += 2 //length of format

	bind.Write(getUint32Byte(bindLen)) //length
	bind.Write(portalBuf.Bytes())      //portal
	bind.Write(stmtBuf.Bytes())        //statment
	bind.Write(format.Bytes())         //format(mats,mat)
	bind.Write(value.Bytes())          //values

	bind.Write([]byte{ZeroByte, ZeroByte}) //result format
	return bind.Bytes()
}

func getExec(portal string, rows uint32) []byte {
	var exec, portalBuf bytes.Buffer
	var execLen uint32 = 8 //length + returns

	le := make([]byte, 4)
	exec.WriteByte(0x45) //execute

	if portal != "" {
		portalBuf.Write([]byte(portal))
		portalBuf.WriteByte(ZeroByte)
		execLen += uint32(portalBuf.Len())
	} else {
		portalBuf.WriteByte(ZeroByte)
	}
	binary.BigEndian.PutUint32(le, execLen)
	exec.Write(getUint32Byte(execLen)) //length
	exec.Write(portalBuf.Bytes())
	exec.WriteByte(ZeroByte)                   //portal end
	exec.Write([]byte{ZeroByte, ZeroByte, ZeroByte, ZeroByte}) // all rows
	return exec.Bytes()
}

func getDesc(portal string) []byte {
	//describe
	var describe, portalBuf bytes.Buffer
	var length uint32 = 6 //length + 0x50 +zeroByte
	le := make([]byte, 4)

	portalBuf.WriteByte(0x50)
	if portal != "" {
		portalBuf.Write([]byte(portal))
		length += uint32(portalBuf.Len())
	}

	describe.WriteByte(0x44)
	describe.Write(le)                //length
	describe.Write(portalBuf.Bytes()) //portal
	describe.WriteByte(ZeroByte)
	return describe.Bytes()
}

func getSync() []byte {
	var sync bytes.Buffer
	sync.WriteByte(0x53) //sync
	sync.Write([]byte{0x00, 0x00, 0x00, 0x04})
	return sync.Bytes()
}

func getTemplate(command string) []byte {
	//parse
	var parse bytes.Buffer
	parse.WriteByte(0x50)                                   //parse
	parse.Write([]byte{ZeroByte, ZeroByte, ZeroByte, 0x0d}) //length
	parse.WriteByte(ZeroByte)                               //statement
	parse.Write([]byte(command))
	parse.WriteByte(ZeroByte)
	parse.WriteByte(ZeroByte)
	parse.WriteByte(ZeroByte)

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

//init
func init() {
	sql.Register("postgres", &PGDriver{})
}
