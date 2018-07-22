package pgdriver_go

import (
	"bytes"
	"database/sql/driver"
	"net"
)

type PGConn struct {
	conn     net.Conn
	pid, key [4]byte
	ready    bool // if ready for next statement
	closed   bool // if been closed
	in_tx    bool

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

	//
	payload []byte
}

func (conn *PGConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	return nil, nil //todo
}

func (conn *PGConn) Prepare(query string) (driver.Stmt, error) {
	stmt := new(PGStmt)

	for _, v := range query {
		if v == '$' {
			stmt.parameters++
		}
	}
	//stmt.parse = getParseStr("", query)
	conn.payload = append(conn.payload, getParseStr("", query)...)

	stmt.conn = conn

	return stmt, nil
}

func (conn *PGConn) Close() error {
	return conn.conn.Close()
}

func (conn *PGConn) Begin() (driver.Tx, error) {
	tx := new(PGTx)
	tx.conn = conn

	var payload bytes.Buffer
	payload.Write(getParse("", "BEGIN", 0))
	payload.Write(getBind("", "", nil))
	//payload.Write(getDesc(""))
	payload.Write(getExec("", 0))
	//payload.Write(getSync())

	//conn.conn.Write(payload.Bytes())
	conn.payload = append(conn.payload, payload.Bytes()...)

	//conn.conn.Write(getTemplate("BEGIN"))

	//result todo

	return tx, nil
}
