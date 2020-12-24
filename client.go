package gortmp

import (
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	flv "github.com/zhangpeihao/goflv"
)

type Handler struct {
	obConn        OutboundConn
	StreamChan    chan OutboundStream
	exit          chan struct{}
	videoDataSize int64
	audioDataSize int64
	status        uint
	filename      string
	rurl          RtmpURL
}

func NewWithConn(url, filename string, c net.Conn) (*Handler, error) {
	h := &Handler{
		StreamChan: make(chan OutboundStream),
		exit:       make(chan struct{}),
		filename:   filename,
	}

	rurl, err := ParseURL(url)
	if err != nil {
		return nil, err
	}

	h.rurl = rurl

	switch rurl.Protocol {
	case "rtmp":
		obConn, err := NewOutbounConn(c, rurl, h, 1000)
		if err != nil {
			return nil, err
		}

		if err = obConn.Connect(); err != nil {
			return nil, err
		}

		h.obConn = obConn
	default:
		return nil, errors.New(fmt.Sprintf("Unsupport protocol %s", rurl.Protocol))
	}

	return h, nil
}

func New(url, filename string) (*Handler, error) {
	h := &Handler{
		StreamChan: make(chan OutboundStream),
		exit:       make(chan struct{}),
		filename:   filename,
	}

	rurl, err := ParseURL(url)
	if err != nil {
		return nil, err
	}

	h.rurl = rurl

	switch rurl.Protocol {
	case "rtmp":
		c, err := net.Dial("tcp", fmt.Sprintf("%s:%d", rurl.Host, rurl.Port))
		if err != nil {
			return nil, err
		}

		obConn, err := NewOutbounConn(c, rurl, h, 1000)
		if err != nil {
			return nil, err
		}

		if err = obConn.Connect(); err != nil {
			return nil, err
		}

		h.obConn = obConn
	default:

		return nil, errors.New(fmt.Sprintf("Unsupport protocol %s", rurl.Protocol))
	}

	return h, nil
}

func (h *Handler) Run() {
	for {
		select {
		case stream := <-h.StreamChan:
			if stream == nil {
				log.Println("StreamChan Closed")
				break
			}

			if h.filename != "" {
				stream.Attach(h)

				if err := stream.Publish(h.rurl.Name, h.rurl.App); err != nil {
					log.Printf("Publish error: %v\n", err)

					return
				}

				continue
			}

			if err := stream.Play(h.rurl.Name, nil, nil, nil); err != nil {
				log.Printf("Play error: %v\n", err)

				return
			}

		case <-time.After(5 * time.Second):

		case <-h.exit:
			log.Println("finished exit")

			return
		}
	}
}

func (h *Handler) Close() {
	if h.obConn == nil {
		return
	}

	h.obConn.Close()
}

func (h *Handler) OnStatus(conn OutboundConn) {
	if h.obConn == nil {
		return
	}

	var err error

	h.status, err = h.obConn.Status()
	if err != nil {
		log.Printf("status: %d, err: %v\n", h.status, err)
	}
}

func (h *Handler) OnClosed(conn Conn) {
	conn.Close()
	close(h.exit)
	close(h.StreamChan)

	log.Println("Closed")
}

func (h *Handler) OnReceived(conn Conn, message *Message) {
	switch message.Type {
	case VIDEO_TYPE:
		h.videoDataSize += int64(message.Buf.Len())

	case AUDIO_TYPE:
		h.audioDataSize += int64(message.Buf.Len())
	}

	message.Buf.Reset()
}

func (h *Handler) OnReceivedRtmpCommand(conn Conn, command *Command) {
}

func (h *Handler) OnStreamCreated(conn OutboundConn, stream OutboundStream) {
	h.StreamChan <- stream
}

func (h *Handler) OnPlayStart(stream OutboundStream) {
}

func (h *Handler) OnPublishStart(stream OutboundStream) {
	go func(stream OutboundStream) {
		flvFile, err := flv.OpenFile(h.filename)
		if err != nil {
			log.Printf("Open FLV dump file error: %v", err)
			return
		}

		defer flvFile.Close()
		startTS := uint32(0)
		startAt := time.Now().UnixNano()
		preTS := uint32(0)

		for h.status == OUTBOUND_CONN_STATUS_CREATE_STREAM_OK {
			if flvFile.IsFinished() {
				log.Println("@File finished")
				flvFile.LoopBack()
				startAt = time.Now().UnixNano()
				startTS = uint32(0)
				preTS = uint32(0)
			}

			header, data, err := flvFile.ReadTag()
			if err != nil {
				log.Printf("flvFile.ReadTag() error: %v", err)
				break
			}

			switch header.TagType {
			case flv.VIDEO_TAG:
				h.videoDataSize += int64(len(data))
			case flv.AUDIO_TAG:
				h.audioDataSize += int64(len(data))
			}

			if startTS == uint32(0) {
				startTS = header.Timestamp
			}
			diff1 := uint32(0)
			if header.Timestamp > startTS {
				diff1 = header.Timestamp - startTS
			}

			if diff1 > preTS {
				preTS = diff1
			}

			if err = stream.PublishData(header.TagType, data, diff1); err != nil {
				log.Printf("PublishData() error: %v", err)

				break
			}

			diff2 := uint32((time.Now().UnixNano() - startAt) / 1000000)
			if diff1 > diff2+100 {
				time.Sleep(time.Millisecond * time.Duration(diff1-diff2))
			}
		}
	}(stream)
}
