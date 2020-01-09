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
	MsgAuth = "AUTH"
	MsgOK   = "OK"
	MsgData = "DATA"
)

type Message struct {
	Type string
	Attr map[string]string
	Len  int
	Data []byte
}

type MessageReader struct {
	rd *bufio.Reader
}

var (
	ErrInvalidMessage = errors.New("Invalid Message")
)

func NewMessageReader(rd io.Reader) *MessageReader {
	return &MessageReader{
		rd: bufio.NewReader(rd),
	}
}

func (r *MessageReader) Read() (msg Message, err error) {
	msg.Attr = make(map[string]string)
	for {
		attr, er := r.rd.ReadBytes('\n')
		if er != nil {
			err = er
			return
		} else if len(attr) == 1 {
			break
		}

		kv := attr[:len(attr)-1]
		if i := bytes.Index(kv, []byte{':'}); i > 0 {
			msg.Attr[string(kv[:i])] = string(kv[i+1:])
		} else {
			err = ErrInvalidMessage
			return
		}
	}
	msg.Type = msg.Attr["type"]
	delete(msg.Attr, "type")

	dataLenStr, ok := msg.Attr["length"]
	if !ok {
		return
	}
	delete(msg.Attr, "length")

	msg.Len, err = strconv.Atoi(dataLenStr)
	if err != nil {
		err = ErrInvalidMessage
		return
	}

	msg.Data = make([]byte, msg.Len)
	_, err = io.ReadFull(r.rd, msg.Data)
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
	fmt.Fprintf(buf, "length:%d\n", m.Len)
	for k, v := range m.Attr {
		buf.WriteString(k + ":" + v + "\n")
	}
	fmt.Fprintf(buf, "%v", m.Data)
	return buf.String()
}
