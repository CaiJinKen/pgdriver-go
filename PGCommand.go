package pgdriver_go

type msgType struct {
	Name string
	Code uint8
	Desc string
}
type Messages struct {
	Msgs map[uint8] *msgType
}
func (msg *Messages)New(){
	 msg.Msgs = make(map[uint8] *msgType)
	msg.Msgs['R']=&msgType{"Authentication",0x52,"Authentication message"}
	msg.Msgs['E']=&msgType{"Execute",0x45,"Execute"}// send
	msg.Msgs['E']=&msgType{"Error",0x45,""}//receive

	msg.Msgs['P'] = &msgType{"Parse",0x50,"Parse"}
	msg.Msgs['K']=&msgType{"Cancel request",0x4b,"Cancel request"}
	msg.Msgs['B']=&msgType{"Bind",0x42,"Bind"}
	msg.Msgs['S']=&msgType{"Sync",0x53,"Sync"}// send
	msg.Msgs['S']=&msgType{"Parameter status",0x53,"Sync"}// receive
	msg.Msgs['C']=&msgType{"Command completion",0x43,"Command completion"}// receive
	msg.Msgs['C']=&msgType{"Close",0x43,"Close"}// send close prepar statement('S':statement,'P':入口)

	msg.Msgs[0x31]=&msgType{"Parse completion",0x31,"Parse completion"}
	msg.Msgs[0x32]=&msgType{"Bind completion",0x32,"Bind completion"}
	msg.Msgs[0x33]=&msgType{"Close completion",0x33,"Close completion"}
	msg.Msgs['Z']=&msgType{"Ready for query",0x5a,"Ready for query"}
	msg.Msgs['D']=&msgType{"Describe",0x44,"Describe"}// send
	msg.Msgs['D']=&msgType{"Data row",0x44,"Data row"}//receive
	msg.Msgs['T']=&msgType{"Row description",0x54,"Row description"}
	msg.Msgs['E']=&msgType{"Error",0x45,"Error"}//receive
	msg.Msgs['H']=&msgType{"Flush",0x48,"Flush"}//send
	msg.Msgs['F']=&msgType{"Function call",0x46,"Function call"}//send
	msg.Msgs['V']=&msgType{"Function call response",0x56,"Function call response"}//receive
	msg.Msgs['n']=&msgType{"No data",0x6e,"No data"}//receive
	msg.Msgs['N']=&msgType{"Notice response",0x4e,"Notice response"}//receive
	msg.Msgs['A']=&msgType{"Notification response",0x41,"Notification response"}//receive
	msg.Msgs['t']=&msgType{"Parameter description",0x74,"Parameter description"}//receive
	msg.Msgs['p']=&msgType{"Password message",0x70,"Password message"}//send
	msg.Msgs['s']=&msgType{"Portal suspended",0x73,"Portal suspended"}//receive
	msg.Msgs['Q']=&msgType{"Query",0x51,"Query"}//send
	msg.Msgs['X']=&msgType{"Terminate",0x58,"Terminate"}//send
}
