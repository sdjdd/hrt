package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unsafe"
)

type Transferable interface {
	Bytes() []byte
}

type (
	AuthMessage struct {
		ID, Token string
	}
	TextMessage struct {
		Content string
	}
	ErrorMessage struct {
		Content string
	}
	DataMessage struct {
		TID  string
		Data []byte
	}
	FirstDataMessage struct {
		DataMessage
		Host string
	}
	LastDataMessage struct {
		DataMessage
		Err string
	}
)

type MessageReader struct {
	rd *bufio.Reader
}

var ErrInvalidMessage = errors.New("invalid message format")

func NewMessageReader(rd io.Reader) *MessageReader {
	return &MessageReader{rd: bufio.NewReader(rd)}
}

func str(p []byte) string { return *(*string)(unsafe.Pointer(&p)) }

func (r *MessageReader) Read() (Transferable, error) {
	firstch, err := r.rd.ReadByte()
	if err != nil {
		return nil, err
	}

	switch firstch {
	case '@':
		line, err := r.rd.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		sp := bytes.Split(line[:len(line)-1], []byte{' '})
		if len(sp) != 2 {
			return nil, ErrInvalidMessage
		}
		return AuthMessage{ID: str(sp[0]), Token: str(sp[1])}, nil

	case '+':
		text, err := r.rd.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		return TextMessage{Content: str(text[:len(text)-1])}, nil

	case '-':
		text, err := r.rd.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		return ErrorMessage{Content: str(text[:len(text)-1])}, nil

	case '=':
		return r.readDataMessage()

	case '[':
		host, err := r.rd.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		dm, err := r.readDataMessage()
		if err != nil {
			return nil, err
		}
		return FirstDataMessage{
			Host:        str(host[:len(host)-1]),
			DataMessage: dm,
		}, nil

	case ']':
		errstr, err := r.rd.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		dm, err := r.readDataMessage()
		if err != nil {
			return nil, err
		}
		return LastDataMessage{
			Err:         str(errstr[:len(errstr)-1]),
			DataMessage: dm,
		}, nil

	default:
		return nil, errors.New("unknown message type")
	}
}

func (r *MessageReader) readDataMessage() (m DataMessage, err error) {
	line, err := r.rd.ReadBytes('\n')
	if err != nil {
		return
	}
	sp := bytes.Split(line[:len(line)-1], []byte{' '})
	if len(sp) != 2 {
		err = ErrInvalidMessage
		return
	}
	m.TID = str(sp[0])
	dlenn, err := strconv.Atoi(str(sp[1]))
	if err != nil {
		err = fmt.Errorf("parse len(data) of data message: %s", err)
		return
	}
	if dlenn > 0 {
		m.Data = make([]byte, dlenn)
		_, err = io.ReadFull(r.rd, m.Data)
		if err != nil {
			return
		}
	}
	return
}

// '@' agent-id SP token LF
func (m AuthMessage) Bytes() []byte {
	bytes := make([]byte, len(m.ID)+len(m.Token)+3)
	var n int

	bytes[n] = '@'
	n++

	n += copy(bytes[n:], m.ID)
	bytes[n] = ' '
	n++

	n += copy(bytes[n:], m.Token)
	bytes[n] = '\n'
	return bytes
}

// '+' text LF
func (m TextMessage) Bytes() []byte {
	bytes := make([]byte, len(m.Content)+2)
	bytes[0] = '+'
	copy(bytes[1:], m.Content)
	bytes[len(bytes)-1] = '\n'
	return bytes
}

// '-' error LF
func (m ErrorMessage) Bytes() []byte {
	bytes := make([]byte, len(m.Content)+2)
	bytes[0] = '-'
	copy(bytes[1:], m.Content)
	bytes[len(bytes)-1] = '\n'
	return bytes
}

// '=' tid SP data-length LF data
func (m DataMessage) Bytes() []byte {
	dlen := strconv.Itoa(len(m.Data))
	bytes := make([]byte, len(m.TID)+len(dlen)+len(m.Data)+3)
	var i int

	bytes[i] = '='
	i++

	i += copy(bytes[i:], m.TID)
	bytes[i] = ' '
	i++

	i += copy(bytes[i:], dlen)
	bytes[i] = '\n'
	i++

	copy(bytes[i:], m.Data)
	return bytes
}

// '[' host LF tid SP data-length LF data
func (m FirstDataMessage) Bytes() []byte {
	dlen := strconv.Itoa(len(m.Data))
	bytes := make([]byte, len(m.Host)+len(m.TID)+len(dlen)+len(m.Data)+4)
	var i int

	bytes[i] = '['
	i++

	i += copy(bytes[i:], m.Host)
	bytes[i] = '\n'
	i++

	i += copy(bytes[i:], m.TID)
	bytes[i] = ' '
	i++

	i += copy(bytes[i:], dlen)
	bytes[i] = '\n'
	i++

	copy(bytes[i:], m.Data)
	return bytes
}

// ']' error LF tid SP data-length LF data
func (m LastDataMessage) Bytes() []byte {
	dlen := strconv.Itoa(len(m.Data))
	bytes := make([]byte, len(m.Err)+len(m.TID)+len(dlen)+len(m.Data)+4)
	var i int

	bytes[i] = ']'
	i++

	i += copy(bytes[i:], m.Err)
	bytes[i] = '\n'
	i++

	i += copy(bytes[i:], m.TID)
	bytes[i] = ' '
	i++

	i += copy(bytes[i:], dlen)
	bytes[i] = '\n'
	i++

	copy(bytes[i:], m.Data)
	return bytes
}
