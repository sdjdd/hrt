package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessageReader_Read(t *testing.T) {
	assert := assert.New(t)
	str := "type:test-message\nkey1:val1\nkey2:val2\nlength:6\n\nhello!"
	r := NewMessageReader(strings.NewReader(str))
	msg, err := r.Read()
	assert.Nil(err)
	assert.Equal("test-message", msg.Type)
	assert.Len(msg.Attr, 2)
	assert.Equal("val1", msg.Attr["key1"])
	assert.Equal("val2", msg.Attr["key2"])
	assert.Equal(6, msg.Len)
	assert.Equal([]byte("hello!"), msg.Data)
}
