package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeAuthMessage(t *testing.T) {
	m := AuthMessage{
		ID:    "test-ID",
		Token: "test-token",
	}
	bytes := []byte("@" + m.ID + " " + m.Token + "\n")
	assert.Equal(t, bytes, m.Bytes())
}

func TestEncodeTextMessage(t *testing.T) {
	m := TextMessage{Content: "blahblahblah"}
	bytes := []byte("+" + m.Content + "\n")
	assert.Equal(t, bytes, m.Bytes())
}

func TestEncodeErrorMessage(t *testing.T) {
	m := ErrorMessage{Content: "something bad!"}
	assert.Equal(t, []byte("-"+m.Content+"\n"), m.Bytes())
}

func TestEncodeDataMessage(t *testing.T) {
	m := DataMessage{
		TID:  "test-tid",
		Data: []byte("test-data"),
	}
	bytes := []byte(fmt.Sprintf("=%s %d\n%s", m.TID, len(m.Data), m.Data))
	assert.Equal(t, bytes, m.Bytes())
}

func TestEncodeFirstDataMessage(t *testing.T) {
	m := FirstDataMessage{
		Host: "114.51.41.191",
		DataMessage: DataMessage{
			TID:  "test-tid",
			Data: []byte("sodayo!"),
		},
	}
	bytes := []byte(fmt.Sprintf("[%s\n%s %d\n%s", m.Host, m.TID, len(m.Data), m.Data))
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
		TID:  "test-tid",
		Data: []byte{114, 5, 14, 191, 98, 10},
	}
	r := NewMessageReader(bytes.NewReader(msg.Bytes()))
	msg2, err := r.Read()
	assert.Nil(t, err)
	assert.Equal(t, msg, msg2)
}

func TestReadFirstDataMessage(t *testing.T) {
	assert := assert.New(t)
	msg := FirstDataMessage{
		Host: "www.114514.com",
		DataMessage: DataMessage{
			TID:  "test-tid",
			Data: []byte{114, 5, 14, 191, 98, 10},
		},
	}
	r := NewMessageReader(bytes.NewReader(msg.Bytes()))
	msg2, err := r.Read()
	assert.Nil(err)
	assert.IsType(FirstDataMessage{}, msg2)
	assert.Equal(msg, msg2)
}
