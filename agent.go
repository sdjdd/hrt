package main

import (
	"fmt"
	"net"
	"time"
)

type Agent struct {
	conn    *Connection
	events  *AgentEvent
	session map[string]net.Conn
}

func NewAgent() *Agent {
	return &Agent{
		events:  NewAgentEvent(),
		session: make(map[string]net.Conn),
	}
}

func (a *Agent) Connect(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	a.conn = NewConnection(conn)

	if err = a.auth("", "test-agent"); err != nil {
		log.Error("auth to broker:", err)
	}

	go a.EventLoop()
	go func() {
		for {
			msg, err := a.conn.ReadMessage(0)
			if err != nil {
				log.Error("read message: ", err)
				break
			}
			a.events.RecvMsg <- msg
		}
		a.conn.Close()
	}()

	return nil
}

func (a *Agent) auth(token, id string) error {
	msg := Message{
		Type: MsgAuth,
		Attr: map[string]string{"token": token, "id": id},
	}
	if err := a.conn.SendMessage(msg); err != nil {
		return err
	}

	msg, err := a.conn.ReadMessage(time.Second * 10)
	if err != nil {
		return err
	} else if msg.Type != MsgOK {
		return fmt.Errorf("expect OK message, got: %s", msg.Type)
	}

	return nil
}

func (a *Agent) EventLoop() {
	for {
		select {
		case msg := <-a.events.RecvMsg:
			a.handleRecvMsg(msg)
		case msg := <-a.events.SendMsg:
			a.conn.SendMessage(msg)
		}
	}
}

func (a *Agent) handleRecvMsg(msg Message) {
	switch msg.Type {
	case MsgData:
		serial := msg.Attr["serial"]
		conn, ok := a.session[serial]
		if !ok {
			var err error
			conn, err = net.Dial("tcp", msg.Attr["host"])
			if err != nil {
				log.Errorf("connect to %s: %s", msg.Attr["host"], err)
				return
			}
			log.Info("dial to ", msg.Attr["host"])
			a.session[serial] = conn
			go func() {
				buf := make([]byte, 16*1024)
				for {
					n, err := conn.Read(buf)
					if err != nil {
						break
					}
					data := make([]byte, n)
					copy(data, buf)
					log.Info("receive: ", string(data))
					a.events.SendMsg <- Message{
						Type: MsgData,
						Len:  n,
						Data: data,
						Attr: map[string]string{
							"serial": serial,
						},
					}
				}
			}()
		}
		log.Info("send: ", string(msg.Data))
		conn.Write(msg.Data)
	default:
		log.Error("unsupported message type: ", msg.Type)
	}
}
