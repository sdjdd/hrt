package main

import (
	"bytes"
	"errors"
	"io"
	"sync/atomic"
)

type EventBuffer struct {
	rbuf bytes.Buffer
	wbuf bytes.Buffer

	closed uint32
	size   int

	onRead  func() (data []byte, err error)
	onFlush func(p []byte) (n int, err error)
	onClose func() error
}

type EventBufferConf struct {
	Size    int
	OnRead  func() (data []byte, err error)
	OnFlush func(p []byte) (n int, err error)
	OnClose func() error
}

var (
	ErrEvBufferClosed = errors.New("event buffer has been closed")
)

func NewEventBuffer(conf EventBufferConf) (b *EventBuffer, err error) {
	b = new(EventBuffer)
	if conf.OnRead == nil || conf.OnClose == nil {
		err = errors.New("must provide OnRead, OnWrite and OnClose handler")
		return
	}
	b.onRead, b.onClose = conf.OnRead, conf.OnClose

	if conf.Size < 0 {
		err = errors.New("buffer size < 0")
		return
	}

	if conf.Size > 0 {
		if conf.OnFlush == nil {
			err = errors.New("must provide OnFlush handler when buffer size > 0")
			return
		}
		b.size, b.onFlush = conf.Size, conf.OnFlush
	}
	return
}

func (b *EventBuffer) Read(p []byte) (n int, err error) {
	if b.rbuf.Len() > 0 {
		return b.rbuf.Read(p)
	}
	if atomic.LoadUint32(&b.closed) != 0 {
		err = io.EOF
		return
	}

	data, err := b.onRead()
	if n = copy(p, data); n < len(data) {
		b.rbuf.Write(data[n:])
	}
	return
}

func (b *EventBuffer) Write(p []byte) (n int, err error) {
	if b.closed != 0 {
		return 0, ErrEvBufferClosed
	}

	overflow := b.wbuf.Len() + len(p) - b.size
	if overflow < 0 {
		return b.wbuf.Write(p)
	}

	b.wbuf.Write(p[:len(p)-overflow])
	n, err = b.Flush()
	if err == nil && overflow > 0 {
		b.wbuf.Write(p[len(p)-overflow:])
	}
	return
}

func (b *EventBuffer) Flush() (n int, err error) {
	if b.wbuf.Len() > 0 {
		n, err = b.onFlush(b.wbuf.Bytes())
		b.wbuf.Truncate(n)
	}
	return
}

func (b *EventBuffer) Close() (err error) {
	if atomic.CompareAndSwapUint32(&b.closed, 0, 1) {
		err = b.onClose()
	}
	return
}
