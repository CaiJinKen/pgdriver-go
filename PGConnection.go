package pgdriver_go

import (
	"bytes"
	"database/sql/driver"
	"encoding/binary"
	"net"
)

type PGConn struct {
	conn     net.Conn
	pid, key [4]byte
	ready    bool // if ready for next statement
	closed   bool // if been closed

	//parameter status
	properties            map[string]string
	application_name      string
	client_encoding       string
	server_encoding       string
	server_version        string
	session_authorization string
	DateStyle             string
	TimeZone              string
	integer_datetimes     string
	IntervalStyle         string
	is_superuser          string
}

func (conn *PGConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	return nil, nil //todo
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
