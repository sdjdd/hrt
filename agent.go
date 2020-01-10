package main

import (
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"
)

type Agent struct {
	ID      string
	conn    net.Conn
	events  *AgentEvent
	tfs     map[string]io.ReadWriter
	msgr    *MessageReader
	nextTID uint64
}

func NewAgent(id string) *Agent {
	return &Agent{
		ID:     id,
		events: NewAgentEvent(),
		tfs:    make(map[string]io.ReadWriter),
	}
}

// ReadMessage try to read a Message from Agent.conn.
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

func (a *Agent) SendOK() error {
	_, err := a.conn.Write([]byte("type:ok\n\n"))
	return err
}

func (a Agent) String() string {
	id := a.ID
	if id == "" {
		id = "unknown"
	}
	return id + "@" + a.conn.RemoteAddr().String()
}

func (a *Agent) gentid() uint64 { return atomic.AddUint64(&a.nextTID, 1) }

func (a *Agent) Connect(addr, token string) (err error) {
	if a.conn, err = net.Dial("tcp", addr); err != nil {
		return
	}
	a.msgr = NewMessageReader(a.conn)

	if err = a.auth(token); err != nil {
		return fmt.Errorf("auth to broker: %s", err)
	}

	go func() {
		for {
			msg, err := a.ReadMessage(0)
			if err != nil {
				log.Error("read message: ", err)
				a.events.LostConn <- err
				break
			}
			a.events.RecvMsg <- msg
		}
		a.conn.Close()
	}()

	a.events.Connected <- struct{}{}
	return nil
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
	if _, ok := msg.(OKMessage); !ok {
		return fmt.Errorf("expect OK message")
	}

	return nil
}

func (a *Agent) EventLoop() {
	for {
		select {
		case <-a.events.Connected:
			log.Info("connect to broker successfully")
		case err := <-a.events.LostConn:
			log.Fatal("lost connection: ", err)
		case msg := <-a.events.RecvMsg:
			a.handleRecvMsg(msg)
		case msg := <-a.events.SendMsg:
			a.SendMessage(msg)
		}
	}
}

func (a *Agent) handleRecvMsg(msg Transferable) {
	switch m := msg.(type) {
	case DataMessage:
		conn, ok := a.tfs[m.TID]
		if !ok {
			var err error
			conn, err = net.Dial("tcp", m.Host)
			if err != nil {
				log.Errorf("connect to %s: %s", m.Host, err)
				return
			}
			log.Info("dial to ", m.Host)
			a.tfs[m.TID] = conn
			go func() {
				buf := make([]byte, 16*1024)
				for {
					n, err := conn.Read(buf)
					if err != nil {
						if err == io.EOF {
							log.Infof("connection to %s closed", m.Host)
						} else {
							log.Infof("connection to %s unexpectedly closed", m.Host)
						}
						break
					}
					data := make([]byte, n)
					copy(data, buf)
					a.events.SendMsg <- Message{
						Type: MsgData,
						Data: data,
						Attr: map[string]string{"tid": m.TID},
					}
				}
			}()
		}
		conn.Write(m.Data)
	default:
		log.Error("received an unsupported message type")
	}
}
