package gortmp

import (
	amf "github.com/zhangpeihao/goamf"
)

// Command
//
// Command messages carry the AMF encoded commands between the client
// and the server. A client or a server can request Remote Procedure
// Calls (RPC) over streams that are communicated using the command
// messages to the peer.
type Command struct {
	IsFlex        bool
	Name          string
	TransactionID uint32
	Objects       []interface{}
}

func (cmd *Command) Write(w Writer) (err error) {
	if cmd.IsFlex {
		err = w.WriteByte(0x00)
		if err != nil {
			return
		}
	}
	_, err = amf.WriteString(w, cmd.Name)
	if err != nil {
		return
	}
	_, err = amf.WriteDouble(w, float64(cmd.TransactionID))
	if err != nil {
		return
	}
	for _, object := range cmd.Objects {
		_, err = amf.WriteValue(w, object)
		if err != nil {
			return
		}
	}
	return
}
