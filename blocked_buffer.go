package main

import (
	"bytes"
	"io"
	"sync"
)

type BlockedBuffer struct {
	data  bytes.Buffer
	rcond *sync.Cond
	wcond *sync.Cond
	err   error
}

func NewBlockedBuffer() *BlockedBuffer {
	mu := new(sync.Mutex)
	return &BlockedBuffer{
		rcond: sync.NewCond(mu),
		wcond: sync.NewCond(mu),
	}
}

func (t *BlockedBuffer) Write(p []byte) (n int, err error) {
	t.wcond.L.Lock()
	for t.err == nil && t.data.Len() > 0 {
		t.wcond.Wait()
	}
	if err = t.err; err == nil {
		n, err = t.data.Write(p)
		defer t.rcond.Signal()
	}
	t.wcond.L.Unlock()
	return
}

func (t *BlockedBuffer) Read(p []byte) (n int, err error) {
	t.rcond.L.Lock()
	for t.err == nil && t.data.Len() == 0 {
		t.rcond.Wait()
	}
	if t.data.Len() > 0 {
		n, err = t.data.Read(p)
		if t.data.Len() == 0 {
			defer t.wcond.Signal()
		} else {
			defer t.rcond.Signal()
		}
	} else {
		err = t.err
	}
	t.rcond.L.Unlock()
	return
}

func (t *BlockedBuffer) Close() (err error) {
	return t.SetError(io.EOF)
}

func (t *BlockedBuffer) SetError(e error) (err error) {
	t.wcond.L.Lock()
	if err = t.err; err == nil {
		t.err = e
		defer func() {
			t.wcond.Broadcast()
			t.rcond.Broadcast()
		}()
	}
	t.wcond.L.Unlock()
	return
}
