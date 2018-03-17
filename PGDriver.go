package pgdriver_go

import (
	"database/sql"
	"database/sql/driver"
	"net"
)

type PGDriver struct {
}

func (d *PGDriver) Open(name string) (driver.Conn, error) {
	conn := new(PGConn)
	return conn, nil
}

type PGConn struct {
	conn net.Conn
}

func (conn *PGConn) Prepare(query string) (driver.Stmt, error) {
	stmt := new(PGStmt)
	return stmt, nil
}

func (conn *PGConn) Close() error {
	return nil
}

func (conn *PGConn) Begin() (driver.Tx, error) {
	tx := new(PGTx)
	return tx, nil
}

type PGTx struct {
}

func (tx *PGTx) Commit() error {
	return nil
}
func (tx *PGTx) Rollback() error {
	return nil
}

type PGStmt struct {
}

func (stmt *PGStmt) Close() error {
	return nil
}
func (stmt *PGStmt) NumInput() int {
	return 0
}
func (stmt *PGStmt) Exec(args []driver.Value) (driver.Result, error) {
	result := new(PGResult)
	return result, nil
}
func (stmt *PGStmt) Query(args []driver.Value) (driver.Rows, error) {
	rows := new(PGRows)
	return rows, nil
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
}

func (r *PGRows) Columns() []string {
	return nil
}
func (r *PGRows) Close() error {
	return nil
}
func (r *PGRows) Next(dest []driver.Value) error {
	return nil
}

//init
func init() {
	sql.Register("postgres", &PGDriver{})
}
