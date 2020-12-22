package gortmp

import (
	"bytes"
)

// Message
//
// The different types of messages that are exchanged between the server
// and the client include audio messages for sending the audio data,
// video messages for sending video data, data messages for sending any
// user data, shared object messages, and command messages.
type Message struct {
	ChunkStreamID     uint32
	Timestamp         uint32
	Size              uint32
	Type              uint8
	StreamID          uint32
	Buf               *bytes.Buffer
	IsInbound         bool
	AbsoluteTimestamp uint32
}

func NewMessage(csi uint32, t uint8, sid uint32, ts uint32, data []byte) *Message {
	message := &Message{
		ChunkStreamID:     csi,
		Type:              t,
		StreamID:          sid,
		Timestamp:         ts,
		AbsoluteTimestamp: ts,
		Buf:               new(bytes.Buffer),
	}
	if data != nil {
		message.Buf.Write(data)
		message.Size = uint32(len(data))
	}
	return message
}

// The length of remain data to read
func (message *Message) Remain() uint32 {
	if message.Buf == nil {
		return message.Size
	}

	return message.Size - uint32(message.Buf.Len())
}
