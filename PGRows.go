package pgdriver_go

import (
	"database/sql/driver"
	"errors"
)

type PGRows struct {
	columns []string
	counts  uint32

	Data  [][]driver.Value
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
		return errors.New(NoMoreData)
	}
	src := r.Data[r.index]
	for i, v := range src {
		dest[i] = v
	}
	r.index++
	return nil
}
