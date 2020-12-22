

package gortmp

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	amf "github.com/zhangpeihao/goamf"
)

const (
	OUTBOUND_CONN_STATUS_CLOSE            = uint(0)
	OUTBOUND_CONN_STATUS_HANDSHAKE_OK     = uint(1)
	OUTBOUND_CONN_STATUS_CONNECT          = uint(2)
	OUTBOUND_CONN_STATUS_CONNECT_OK       = uint(3)
	OUTBOUND_CONN_STATUS_CREATE_STREAM    = uint(4)
	OUTBOUND_CONN_STATUS_CREATE_STREAM_OK = uint(5)
)

// A Handler for outbound connection
type OutboundConnHandler interface {
	ConnHandler
	// When connection status changed
	OnStatus(obConn OutboundConn)
	// On stream created
	OnStreamCreated(obConn OutboundConn, stream OutboundStream)
}

type OutboundConn interface {
	// Connect an appliction on FMS after handshake.
	Connect(extendedParameters ...interface{}) (err error)
	// Create a stream
	CreateStream() (err error)
	// Close a connection
	Close()
	// Connection status
	Status() (uint, error)
	// Send a message
	Send(message *Message) error
	// Calls a command or method on Flash Media Server
	// or on an application server running Flash Remoting.
	Call(name string, customParameters ...interface{}) (err error)
	// Get network connect instance
	Connection() Conn
}

// High-level interface
//
// A RTMP connection(based on TCP) to RTMP server(FMS or crtmpserver).
// In one connection, we can create many chunk Streams.
type OBConn struct {
	URL          string
	RURL         RtmpURL
	status       uint
	err          error
	Handler      OutboundConnHandler
	Conn         Conn
	Transactions map[uint32]string
	Streams      map[uint32]OutboundStream
}

// Connect to FMS server, and finish handshake process
func Dial(url string, Handler OutboundConnHandler, maxChannelNumber int) (OutboundConn, error) {
	rurl, err := ParseURL(url)
	if err != nil {
		return nil, err
	}

	switch rurl.Protocol {
	case "rtmp":
		c, err := net.Dial("tcp", fmt.Sprintf("%s:%d", rurl.Host, rurl.Port))
		if err != nil {
			return nil, err
		}

		return NewOutbounConn(c, rurl, Handler, maxChannelNumber)

	default:

		return nil, errors.New(fmt.Sprintf("Unsupport protocol %s", rurl.Protocol))
	}
}

// Connect to FMS server, and finish handshake process
func NewOutbounConn(c net.Conn, rurl RtmpURL, Handler OutboundConnHandler, maxChannelNumber int) (OutboundConn, error) {
	ipConn, ok := c.(*net.TCPConn)
	if ok {
		ipConn.SetWriteBuffer(128 * 1024)
	}

	br := bufio.NewReader(c)
	bw := bufio.NewWriter(c)

	timeout := time.Duration(10 * time.Second)

	if err := Handshake(c, br, bw, timeout); err != nil {
		return nil, err
	}

	obConn := &OBConn{
		URL:          rurl.URL,
		RURL:         rurl,
		Handler:      Handler,
		status:       OUTBOUND_CONN_STATUS_HANDSHAKE_OK,
		Transactions: make(map[uint32]string),
		Streams:      make(map[uint32]OutboundStream),
	}

	obConn.Handler.OnStatus(obConn)
	obConn.Conn = NewConn(c, br, bw, obConn, maxChannelNumber)
	return obConn, nil
}

// Connect an appliction on FMS after handshake.
func (obConn *OBConn) Connect(extendedParameters ...interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
			if obConn.err == nil {
				obConn.err = err
			}
		}
	}()
	// Create connect command
	buf := new(bytes.Buffer)
	// Command name
	_, err = amf.WriteString(buf, "connect")
	CheckError(err, "Connect() Write name: connect")
	transactionID := obConn.Conn.NewTransactionID()
	obConn.Transactions[transactionID] = "connect"
	_, err = amf.WriteDouble(buf, float64(transactionID))
	CheckError(err, "Connect() Write transaction ID")
	_, err = amf.WriteObjectMarker(buf)
	CheckError(err, "Connect() Write object marker")

	_, err = amf.WriteObjectName(buf, "app")
	CheckError(err, "Connect() Write app name")
	_, err = amf.WriteString(buf, obConn.RURL.Domain)
	CheckError(err, "Connect() Write app value")

	_, err = amf.WriteObjectName(buf, "flashVer")
	CheckError(err, "Connect() Write flashver name")
	_, err = amf.WriteString(buf, FLASH_PLAYER_VERSION_STRING)
	CheckError(err, "Connect() Write flashver value")

	//	_, err = amf.WriteObjectName(buf, "swfUrl")
	//	CheckError(err, "Connect() Write swfUrl name")
	//	_, err = amf.WriteString(buf, SWF_URL_STRING)
	//	CheckError(err, "Connect() Write swfUrl value")

	_, err = amf.WriteObjectName(buf, "tcUrl")
	CheckError(err, "Connect() Write tcUrl name")

	tcUrl := strings.Split(obConn.URL, "/")
	_, err = amf.WriteString(buf, strings.Join(tcUrl[:len(tcUrl)-1], "/"))
	CheckError(err, "Connect() Write tcUrl value")

	_, err = amf.WriteObjectName(buf, "fpad")
	CheckError(err, "Connect() Write fpad name")
	_, err = amf.WriteBoolean(buf, false)
	CheckError(err, "Connect() Write fpad value")

	_, err = amf.WriteObjectName(buf, "capabilities")
	CheckError(err, "Connect() Write capabilities name")
	_, err = amf.WriteDouble(buf, DEFAULT_CAPABILITIES)
	CheckError(err, "Connect() Write capabilities value")

	_, err = amf.WriteObjectName(buf, "audioCodecs")
	CheckError(err, "Connect() Write audioCodecs name")
	_, err = amf.WriteDouble(buf, DEFAULT_AUDIO_CODECS)
	CheckError(err, "Connect() Write audioCodecs value")

	_, err = amf.WriteObjectName(buf, "videoCodecs")
	CheckError(err, "Connect() Write videoCodecs name")
	_, err = amf.WriteDouble(buf, DEFAULT_VIDEO_CODECS)
	CheckError(err, "Connect() Write videoCodecs value")

	_, err = amf.WriteObjectName(buf, "videoFunction")
	CheckError(err, "Connect() Write videoFunction name")
	_, err = amf.WriteDouble(buf, float64(1))
	CheckError(err, "Connect() Write videoFunction value")

	_, err = amf.WriteObjectEndMarker(buf)
	CheckError(err, "Connect() Write ObjectEndMarker")

	// extended parameters
	for _, param := range extendedParameters {
		_, err = amf.WriteValue(buf, param)
		CheckError(err, "Connect() Write extended parameters")
	}
	connectMessage := &Message{
		ChunkStreamID: CS_ID_COMMAND,
		Type:          COMMAND_AMF0,
		Size:          uint32(buf.Len()),
		Buf:           buf,
	}

	obConn.status = OUTBOUND_CONN_STATUS_CONNECT
	return obConn.Conn.Send(connectMessage)
}

// Close a connection
func (obConn *OBConn) Close() {
	for _, stream := range obConn.Streams {
		stream.Close()
	}
	obConn.status = OUTBOUND_CONN_STATUS_CLOSE
	go func() {
		time.Sleep(time.Second)
		obConn.Conn.Close()
	}()
}

// Connection status
func (obConn *OBConn) Status() (uint, error) {
	return obConn.status, obConn.err
}

// Callback when recieved message. Audio & Video data
func (obConn *OBConn) OnReceived(Conn Conn, message *Message) {
	stream, found := obConn.Streams[message.StreamID]
	if found {
		if !stream.Received(message) {
			obConn.Handler.OnReceived(Conn, message)
		}
	} else {
		obConn.Handler.OnReceived(Conn, message)
	}
}

// Callback when recieved message.
func (obConn *OBConn) OnReceivedRtmpCommand(Conn Conn, command *Command) {
	switch command.Name {
	case "_result":
		transaction, found := obConn.Transactions[command.TransactionID]
		if found {
			switch transaction {
			case "connect":
				if command.Objects != nil && len(command.Objects) >= 2 {
					information, ok := command.Objects[1].(amf.Object)
					if ok {
						code, ok := information["code"]
						if ok && code == RESULT_CONNECT_OK {
							// Connect OK
							//time.Sleep(time.Duration(200) * time.Millisecond)
							obConn.Conn.SetWindowAcknowledgementSize()
							obConn.status = OUTBOUND_CONN_STATUS_CONNECT_OK
							obConn.Handler.OnStatus(obConn)
							obConn.status = OUTBOUND_CONN_STATUS_CREATE_STREAM
							obConn.CreateStream()
						}
					}
				}
			case "createStream":
				if command.Objects != nil && len(command.Objects) >= 2 {
					streamID, ok := command.Objects[1].(float64)
					if ok {
						newChunkStream, err := obConn.Conn.CreateMediaChunkStream()
						if err != nil {
							log.Printf("OBConn::ReceivedRtmpCommand() CreateMediaChunkStream err:", err)
							return
						}

						stream := &outboundStream{
							id:            uint32(streamID),
							conn:          obConn,
							chunkStreamID: newChunkStream.ID,
						}

						obConn.Streams[stream.ID()] = stream
						obConn.status = OUTBOUND_CONN_STATUS_CREATE_STREAM_OK
						obConn.Handler.OnStatus(obConn)
						obConn.Handler.OnStreamCreated(obConn, stream)
					}
				}
			}
			delete(obConn.Transactions, command.TransactionID)
		}
	case "_error":
		transaction, found := obConn.Transactions[command.TransactionID]
		log.Println(transaction, found)

	case "onBWCheck":
	}

	obConn.Handler.OnReceivedRtmpCommand(obConn.Conn, command)
}

// Connection closed
func (obConn *OBConn) OnClosed(Conn Conn) {
	obConn.status = OUTBOUND_CONN_STATUS_CLOSE
	obConn.Handler.OnStatus(obConn)
	obConn.Handler.OnClosed(Conn)
}

// Create a stream
func (obConn *OBConn) CreateStream() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
			if obConn.err == nil {
				obConn.err = err
			}
		}
	}()
	// Create createStream command
	transactionID := obConn.Conn.NewTransactionID()
	cmd := &Command{
		IsFlex:        false,
		Name:          "createStream",
		TransactionID: transactionID,
		Objects:       make([]interface{}, 1),
	}
	cmd.Objects[0] = nil
	buf := new(bytes.Buffer)
	err = cmd.Write(buf)
	CheckError(err, "createStream() Create command")
	obConn.Transactions[transactionID] = "createStream"

	message := &Message{
		ChunkStreamID: CS_ID_COMMAND,
		Type:          COMMAND_AMF0,
		Size:          uint32(buf.Len()),
		Buf:           buf,
	}

	return obConn.Conn.Send(message)
}

// Send a message
func (obConn *OBConn) Send(message *Message) error {
	return obConn.Conn.Send(message)
}

// Calls a command or method on Flash Media Server
// or on an application server running Flash Remoting.
func (obConn *OBConn) Call(name string, customParameters ...interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
			if obConn.err == nil {
				obConn.err = err
			}
		}
	}()
	// Create command
	transactionID := obConn.Conn.NewTransactionID()
	cmd := &Command{
		IsFlex:        false,
		Name:          name,
		TransactionID: transactionID,
		Objects:       make([]interface{}, 1+len(customParameters)),
	}
	cmd.Objects[0] = nil
	for index, param := range customParameters {
		cmd.Objects[index+1] = param
	}
	buf := new(bytes.Buffer)
	err = cmd.Write(buf)
	CheckError(err, "Call() Create command")
	obConn.Transactions[transactionID] = name

	message := &Message{
		ChunkStreamID: CS_ID_COMMAND,
		Type:          COMMAND_AMF0,
		Size:          uint32(buf.Len()),
		Buf:           buf,
	}

	return obConn.Conn.Send(message)

}

// Get network connect instance
func (obConn *OBConn) Connection() Conn {
	return obConn.Conn
}
