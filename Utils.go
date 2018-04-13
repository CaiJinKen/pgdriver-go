package pgdriver_go

import "encoding/binary"

func getUint32Byte(len uint32) []byte{
	le := make([]byte, 4)
	binary.BigEndian.PutUint32(le, len)
	return le
}

func getUint16Byte(len uint16) []byte{
	le := make([]byte, 2)
	binary.BigEndian.PutUint16(le, len)
	return le
}
