package main

import (
	"errors"
	"fmt"
	"net"
	"time"
)

type Agent struct {
	conn net.Conn
	msgr *MessageReader

	ev    AgentEvent
	tfs   map[string]Transferer
	lcons map[string]net.Conn

	ID string
}

type tunnelInfo struct {
}

type AgentEvent struct {
	GetLocalConn    chan AE_GetLocalConn
	CloseLocalConn  chan AE_CloseLocalConn
	DispatchRequest chan DataMessage
}

type (
	AE_GetLocalConn struct {
		TID    string
		Host   string
		Future *Future
	}
	AE_CloseLocalConn struct {
		TID, Host string
		Err       error
	}
)

func NewAgent(id string) *Agent {
	a := &Agent{
		ID:    id,
		tfs:   make(map[string]Transferer),
		lcons: make(map[string]net.Conn),
	}
	a.ev.GetLocalConn = make(chan AE_GetLocalConn)
	a.ev.CloseLocalConn = make(chan AE_CloseLocalConn)
	a.ev.DispatchRequest = make(chan DataMessage)
	return a
}

func (a *Agent) ReadMessage(timeout time.Duration) (Transferable, error) {
	if timeout > 0 {
		a.conn.SetReadDeadline(time.Now().Add(timeout))
		defer a.conn.SetReadDeadline(time.Time{})
	}
	return a.msgr.Read()
}

func (a *Agent) SendMessage(msg Transferable) error {
	_, err := a.conn.Write(msg.Bytes())
	return err
}

func (a Agent) String() string {
	id := a.ID
	if id == "" {
		id = "unknown"
	}
	return id + "@" + a.conn.RemoteAddr().String()
}

func (a *Agent) Connect(addr, token string) (err error) {
	if a.conn, err = net.Dial("tcp", addr); err != nil {
		return
	}
	defer a.conn.Close()
	a.msgr = NewMessageReader(a.conn)

	if err = a.auth(token); err != nil {
		return fmt.Errorf("auth to broker: %s", err)
	}

	log.Info("connect to broker successfully")

	var msg Transferable
	for {
		msg, err = a.ReadMessage(0)
		if err != nil {
			break
		}
		switch m := msg.(type) {
		case TextMessage:
			log.Info("message from broker: ", m.Content)

		default:

		}
	}

	return
}

func (a *Agent) auth(token string) error {
	err := a.SendMessage(AuthMessage{Token: token, ID: a.ID})
	if err != nil {
		return err
	}

	msg, err := a.ReadMessage(time.Second * 10)
	if err != nil {
		return err
	}

	switch m := msg.(type) {
	case TextMessage:
		if m.Content != "OK" {
			return errors.New(m.Content)
		}
	case ErrorMessage:
		return errors.New(m.Content)
	}

	return nil
}

func (a *Agent) getLocalConn(host, tid string) (conn net.Conn, err error) {
	future := NewFuture()
	a.ev.GetLocalConn <- AE_GetLocalConn{
		TID:    tid,
		Host:   host,
		Future: future,
	}
	val, err := future.Result()
	if err == nil {
		conn = val.(net.Conn)
	}
	return
}

func (a *Agent) Close() error {
	return nil
}

func (a *Agent) eh_GetLocalConn(e AE_GetLocalConn) {
	s := e.TID + e.Host
	conn, ok := a.lcons[s]
	if ok {
		e.Future.Resolve(conn)
		return
	}

	conn, err := net.Dial("tcp", e.Host)
	if err != nil {
		log.Debugf("fail to create local connection to %s: %s", e.Host, err)
		e.Future.Reject(err)
		return
	}
	a.lcons[s] = conn
	e.Future.Resolve(conn)
	log.Debugf("local connection to %s created", e.Host)

	go func() {
		buf := make([]byte, 16*1024)
		for serial := 0; ; serial++ {
			_, err := conn.Read(buf)
			if err != nil {
				log.Error("read data from local connection: ", err)
				break
			}
		}
		a.ev.CloseLocalConn <- AE_CloseLocalConn{TID: e.TID, Host: e.Host}
	}()
}

func (a *Agent) eh_CloseLocalConn(tid, host string) {
	s := tid + host
	conn, ok := a.lcons[s]
	if !ok {
		return
	}
	conn.Close()
	delete(a.lcons, s)
}
