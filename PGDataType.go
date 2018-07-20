package pgdriver_go

var PGDataType map[string]int32

func init() {
	PGDataType = make(map[string] int32)
	PGDataType["bool"] = 16
	PGDataType["bytea"] = 17
	PGDataType["int8"] = 20
	PGDataType["serial8"] = 20
	PGDataType["int2"] = 21
	PGDataType["serial2"] = 21
	PGDataType["int4"] = 23
	PGDataType["serial4"] = 23
	PGDataType["text"] = 25
	PGDataType["json"] = 114
	PGDataType["xml"] = 142
	PGDataType["cidr"] = 650
	PGDataType["float4"] = 700
	PGDataType["float8"] = 701
	PGDataType["money"] = 790
	PGDataType["inet"] = 869
	PGDataType["char"] = 1042
	PGDataType["varchar"] = 1043
	PGDataType["date"] = 1082
	PGDataType["time"] = 1083
	PGDataType["timestamp"] = 1114
	PGDataType["timestamptz"] = 1184
	PGDataType["uuid"] = 2950
}
