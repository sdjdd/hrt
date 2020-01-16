package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
)

type Transferable interface {
	Bytes() []byte
}

type (
	AuthMessage struct {
		Token, ID string
	}
	TextMessage struct {
		Content string
	}
	ErrorMessage struct {
		Content string
	}
	DataMessage struct {
		Serial int
		TID    string
		Host   string
		Data   []byte
	}
	CloseMessage struct {
		TID    string
		Reason string
	}
)

type MessageReader struct {
	rd *bufio.Reader
}

var ErrInvalidMessage = errors.New("unknown message type")

func NewMessageReader(rd io.Reader) *MessageReader {
	return &MessageReader{
		rd: bufio.NewReader(rd),
	}
}

func (r *MessageReader) Read() (msg Transferable, err error) {
	firstch, err := r.rd.ReadByte()
	if err != nil {
		return
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
		return AuthMessage{
			Token: string(sp[0]),
			ID:    string(sp[1]),
		}, nil

	case '+':
		text, err := r.rd.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		return TextMessage{Content: string(text[:len(text)-1])}, nil

	case '-':
		text, err := r.rd.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		return ErrorMessage{Content: string(text[:len(text)-1])}, nil

	case '=':
		line, err := r.rd.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		sp := bytes.Split(line[:len(line)-1], []byte{' '})
		if len(sp) < 4 {
			return nil, ErrInvalidMessage
		}
		serial, err := strconv.Atoi(string(sp[0]))
		if err != nil {
			return nil, ErrInvalidMessage
		}
		dlenn, err := strconv.Atoi(string(sp[2]))
		if err != nil {
			return nil, fmt.Errorf("parse len(data) of data message: %s", err)
		}
		data := make([]byte, dlenn)
		if _, err := io.ReadFull(r.rd, data); err != nil {
			return nil, err
		}
		return DataMessage{
			Serial: serial,
			TID:    string(sp[1]),
			Host:   string(bytes.Join(sp[3:], nil)),
			Data:   data,
		}, nil

	case '~':
		tid, err := r.rd.ReadBytes(' ')
		if err != nil {
			return nil, err
		}
		reason, err := r.rd.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		return CloseMessage{
			TID:    string(tid[:len(tid)-1]),
			Reason: string(reason[:len(reason)-1]),
		}, nil

	default:
		return nil, errors.New("unknown message type")
	}
}

func (m AuthMessage) Bytes() []byte {
	bytes := make([]byte, len(m.Token)+len(m.ID)+3)
	bytes[0] = '@'
	copy(bytes[1:], m.Token)
	bytes[len(m.Token)+1] = ' '
	copy(bytes[len(m.Token)+2:], m.ID)
	bytes[len(bytes)-1] = '\n'
	return bytes
}

func (m TextMessage) Bytes() []byte {
	bytes := make([]byte, len(m.Content)+2)
	bytes[0] = '+'
	copy(bytes[1:], m.Content)
	bytes[len(bytes)-1] = '\n'
	return bytes
}

// '=' serial SP tid SP data-length SP host LF data
func (m DataMessage) Bytes() []byte {
	serial := strconv.Itoa(m.Serial)
	dlen := strconv.Itoa(len(m.Data))
	bytes := make([]byte, len(serial)+len(m.TID)+len(dlen)+len(m.Host)+len(m.Data)+5)
	var i int

	bytes[i] = '='
	i++

	i += copy(bytes[i:], serial)
	bytes[i] = ' '
	i++

	i += copy(bytes[i:], m.TID)
	bytes[i] = ' '
	i++

	i += copy(bytes[i:], dlen)
	bytes[i] = ' '
	i++

	i += copy(bytes[i:], m.Host)
	bytes[i] = '\n'
	i++
	copy(bytes[i:], m.Data)
	return bytes
}

func (m CloseMessage) Bytes() []byte {
	bytes := make([]byte, len(m.TID)+len(m.Reason)+3)
	bytes[0] = '~'
	copy(bytes[1:], m.TID)
	bytes[len(m.TID)+1] = ' '
	copy(bytes[len(m.TID)+2:], m.Reason)
	bytes[len(bytes)-1] = '\n'
	return bytes
}

func (m ErrorMessage) Bytes() []byte {
	bytes := make([]byte, len(m.Content)+2)
	bytes[0] = '-'
	copy(bytes[1:], m.Content)
	bytes[len(bytes)-1] = '\n'
	return bytes
}
