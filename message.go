package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	MsgAuth = "auth"
	MsgOK   = "ok"
	MsgData = "data"
)

type Transferable interface {
	Bytes() []byte
}

type (
	Message struct {
		Type string
		Attr map[string]string
		Data []byte
	}
	AuthMessage struct {
		Token, ID string
	}
	DataMessage struct {
		TID   string
		Host  string
		Close string // explain whether and why the connection is closed
		Data  []byte
	}
	OKMessage struct{}
)

type MessageReader struct {
	rd *bufio.Reader
}

var (
	ErrInvalidMessage = errors.New("invalid Message")
)

func NewMessageReader(rd io.Reader) *MessageReader {
	return &MessageReader{
		rd: bufio.NewReader(rd),
	}
}

func (r *MessageReader) Read() (msg Transferable, err error) {
	attr := make(map[string]string)
	for {
		line, er := r.rd.ReadBytes('\n')
		if er != nil {
			err = er
			return
		} else if len(line) == 1 {
			break
		}

		if i := bytes.Index(line, []byte{':'}); i > 0 {
			attr[string(line[:i])] = string(line[i+1 : len(line)-1])
		} else {
			err = ErrInvalidMessage
			return
		}
	}

	switch attr["type"] {
	case "":
		err = ErrInvalidMessage
		return
	case MsgAuth:
		msg = AuthMessage{Token: attr["token"], ID: attr["id"]}
		return
	case MsgOK:
		msg = OKMessage{}
		return
	case MsgData:
		dataLen, ok := attr["length"]
		if !ok {
			err = ErrInvalidMessage
			return
		}
		length, er := strconv.Atoi(dataLen)
		if er != nil {
			err = ErrInvalidMessage
			return
		}
		data := make([]byte, length)
		_, err = io.ReadFull(r.rd, data)
		msg = DataMessage{
			TID:   attr["tid"],
			Host:  attr["host"],
			Close: attr["close"],
			Data:  data,
		}
	}

	return
}

func (m Message) Bytes() []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "type:%s\n", m.Type)
	if len(m.Data) > 0 {
		fmt.Fprintf(&buf, "length:%d\n", len(m.Data))
	}
	for k, v := range m.Attr {
		buf.WriteString(k + ":" + v + "\n")
	}
	buf.WriteByte('\n')
	buf.Write(m.Data)
	return buf.Bytes()
}

func (m Message) String() string {
	buf := new(strings.Builder)
	buf.WriteString("type:" + m.Type + "\n")
	for k, v := range m.Attr {
		buf.WriteString(k + ":" + v + "\n")
	}
	fmt.Fprintf(buf, "%v", m.Data)
	return buf.String()
}

func (m AuthMessage) Bytes() []byte {
	return Message{
		Type: MsgAuth,
		Attr: map[string]string{"token": m.Token, "id": m.ID},
	}.Bytes()
}

func (m OKMessage) Bytes() []byte {
	return []byte("type:ok\n\n")
}

func (m DataMessage) Bytes() []byte {
	attr := map[string]string{"tid": m.TID}
	if m.Host != "" {
		attr["host"] = m.Host
	}
	if m.Close != "" {
		attr["close"] = m.Close
	}
	return Message{
		Type: MsgAuth,
		Attr: attr,
		Data: m.Data,
	}.Bytes()
}
