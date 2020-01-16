package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeAuthMessage(t *testing.T) {
	m := AuthMessage{
		Token: "test-token",
		ID:    "test-ID",
	}
	bytes := []byte("@" + m.Token + " " + m.ID + "\n")
	assert.Equal(t, bytes, m.Bytes())
}

func TestEncodeTextMessage(t *testing.T) {
	m := TextMessage{Content: "blahblahblah"}
	bytes := []byte("+" + m.Content + "\n")
	assert.Equal(t, bytes, m.Bytes())
}

func TestEncodeDataMessage(t *testing.T) {
	m := DataMessage{
		Serial: 114514,
		TID:    "test-tid",
		Host:   "test-host",
		Data:   []byte("test-data"),
	}
	bytes := []byte(fmt.Sprintf("=%d %s %d %s\n%s",
		m.Serial, m.TID, len(m.Data), m.Host, m.Data))
	assert.Equal(t, bytes, m.Bytes())
}

func TestEncodeErrorMessage(t *testing.T) {
	m := ErrorMessage{Content: "something bad!"}
	assert.Equal(t, []byte("-"+m.Content+"\n"), m.Bytes())
}

func TestEncodeCloseMessage(t *testing.T) {
	m := CloseMessage{TID: "test-tid", Reason: "EOF"}
	bytes := []byte("~" + m.TID + " " + m.Reason + "\n")
	assert.Equal(t, bytes, m.Bytes())
}

func TestParseAuthMessage(t *testing.T) {
	msg := AuthMessage{Token: "test-token", ID: "test-id"}
	r := NewMessageReader(bytes.NewReader(msg.Bytes()))
	msg2, err := r.Read()
	assert.Nil(t, err)
	assert.Equal(t, msg, msg2)
}

func TestParseTextMessage(t *testing.T) {
	msg := TextMessage{Content: "something good!"}
	r := NewMessageReader(bytes.NewReader(msg.Bytes()))
	msg2, err := r.Read()
	assert.Nil(t, err)
	assert.Equal(t, msg, msg2)
}

func TestParseErrorMessage(t *testing.T) {
	msg := ErrorMessage{Content: "something bad!"}
	r := NewMessageReader(bytes.NewReader(msg.Bytes()))
	msg2, err := r.Read()
	assert.Nil(t, err)
	assert.Equal(t, msg, msg2)
}

func TestParseDataMessage(t *testing.T) {
	msg := DataMessage{
		Serial: 114514,
		TID:    "test-tid",
		Host:   "http://example.com",
		Data:   []byte{114, 5, 14, 191, 98, 10},
	}
	r := NewMessageReader(bytes.NewReader(msg.Bytes()))
	msg2, err := r.Read()
	assert.Nil(t, err)
	assert.Equal(t, msg, msg2)
}

func TestParseCloseMessage(t *testing.T) {
	msg := CloseMessage{TID: "test-tid", Reason: "i'd like to"}
	r := NewMessageReader(bytes.NewReader(msg.Bytes()))
	msg2, err := r.Read()
	assert.Nil(t, err)
	assert.Equal(t, msg, msg2)
}
