package pgdriver_go

var (
	UrlErr      = "Invalid url format, please use format postgres://user:pass@ip:port/db"
	ProtocolErr = "Protocol must be postgres or postgresql."
	AuthErr     = "Invalid password."
	CmdErr      = "Unsupported command."
	NoMoreData  = "Has no more data."
	ParamErr    = "Parameters are invalid."
	QueryErr    = "Error returns when query."
)
