package main

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransferer(t *testing.T) {
	assert := assert.New(t)
	tf := NewBlockedBuffer()
	data := []byte("1145141919810")
	rbuf := make([]byte, len(data))

	n, err := tf.Write(data)
	assert.Nil(err)
	assert.Equal(len(data), n)

	n, err = tf.Read(rbuf)
	assert.Nil(err)
	assert.Equal(len(data), n)

	err = tf.Close()
	assert.Nil(err)

	n, err = tf.Read(rbuf)
	assert.Equal(io.EOF, err)
	assert.Equal(0, n)

	n, err = tf.Write(data)
	assert.Equal(io.EOF, err)
	assert.Equal(0, n)

	err = tf.Close()
	assert.Equal(io.EOF, err)
}

func TestTransferer_ReadUncomplete(t *testing.T) {
	assert := assert.New(t)
	tf := NewBlockedBuffer()
	data := []byte("1145141919810")
	rbuf := make([]byte, len(data)-1)

	tf.Write(data)
	tf.Close()

	n, err := tf.Read(rbuf)
	assert.Nil(err)
	assert.Equal(len(data)-1, n)

	n, err = tf.Read(rbuf)
	assert.Nil(err)
	assert.Equal(1, n)

	n, err = tf.Read(rbuf)
	assert.Equal(io.EOF, err)
	assert.Equal(0, n)
}
