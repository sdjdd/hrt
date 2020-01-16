package main

type Transferer struct {
	Request, Response *BlockedBuffer
}

func NewTransferer() Transferer {
	return Transferer{
		Request:  NewBlockedBuffer(),
		Response: NewBlockedBuffer(),
	}
}
