package main

import (
	"sync"
)

type Future struct {
	val  interface{}
	err  error
	done chan struct{}
	mut  sync.Mutex
}

func NewFuture() *Future {
	return &Future{done: make(chan struct{})}
}

func (f *Future) Resolve(val interface{}) (ok bool) {
	select {
	case <-f.done:
	default:
		f.mut.Lock()
		select {
		case <-f.done:
		default:
			f.val, ok = val, true
			close(f.done)
		}
		f.mut.Unlock()
	}
	return
}

func (f *Future) Reject(err error) (ok bool) {
	select {
	case <-f.done:
	default:
		f.mut.Lock()
		select {
		case <-f.done:
		default:
			f.err, ok = err, true
			close(f.done)
		}
		f.mut.Unlock()
	}
	return
}

func (f *Future) Result() (val interface{}, err error) {
	<-f.done
	return f.val, f.err
}
