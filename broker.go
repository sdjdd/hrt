package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

type (
	Broker struct {
		route  Route
		agents map[string]*Agent
		ev     BrokerEvent
		done   <-chan struct{}

		Token string
	}
)

type BrokerEvent struct {
	AgentOnline      chan *Agent
	AgentOffline     chan *Agent
	CreateTransferer chan BrokerEvCreateTransferer
	DispatchRequest  chan BEvDispatchMessage
	DispatchResponse chan BEvDispatchMessage
}

type BrokerEvCreateTransferer struct {
	Host   string
	future *Future
}

type BEvDispatchMessage struct {
	Agent *Agent
	Msg   DataMessage
}

func (e *BrokerEvent) Init() {
	e.AgentOnline = make(chan *Agent)
	e.AgentOffline = make(chan *Agent)
	e.CreateTransferer = make(chan BrokerEvCreateTransferer)
	e.DispatchRequest = make(chan BEvDispatchMessage)
	e.DispatchResponse = make(chan BEvDispatchMessage)
}

func (b *Broker) Init() {
	b.agents = make(map[string]*Agent)
	b.ev.Init()
}

func (b *Broker) Serve(addr, httpAddr string) (err error) {
	agentListener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("start broker: %s", err)
	}

	httpListener, err := net.Listen("tcp", httpAddr)
	if err != nil {
		return fmt.Errorf("start http service: %s", err)
	}

	go b.acceptAgent(agentListener)
	go b.acceptHTTPRequest(httpListener)

	log.Info("hrt broker listen on ", agentListener.Addr())

	for {
		select {
		case <-b.done:
			agentListener.Close()
			httpListener.Close()
			return
		case agent := <-b.ev.AgentOnline:
			b.eh_AgentOnline(agent)
		case agent := <-b.ev.AgentOffline:
			b.eh_AgentOffline(agent)
		case e := <-b.ev.CreateTransferer:
			b.eh_CreateTransferer(e)
		case e := <-b.ev.DispatchRequest:
			b.eh_DispatchRequest(e)
		case e := <-b.ev.DispatchResponse:
			b.eh_DispatchResponse(e)
		}
	}
}

func (b *Broker) acceptAgent(lsn net.Listener) {
	for {
		conn, err := lsn.Accept()
		if err != nil {
			break
		}
		agent := &Agent{
			conn: conn,
			msgr: NewMessageReader(conn),
		}
		go b.auth(agent)
	}
}

func (b *Broker) auth(agent *Agent) {
	var err error
	defer func() {
		if err != nil {
			log.Errorf("auth agent %s: %s", agent, err)
			agent.conn.Close()
		}
	}()

	msg, err := agent.ReadMessage(time.Second * 10)
	if err != nil {
		return
	}

	m, ok := msg.(AuthMessage)
	if !ok {
		err = errors.New("received a non-auth message")
		return
	}
	if agent.ID = m.ID; agent.ID == "" {
		err = errors.New("id is empty")
		return
	}
	if m.Token != b.Token {
		err = errors.New("token is not correct")
		return
	}

	err = agent.SendMessage(TextMessage{Content: "OK"})
	if err != nil {
		err = fmt.Errorf("send OK message: %s", err)
		return
	}

	b.ev.AgentOnline <- agent
}

func (b *Broker) recvAgentMessage(agent *Agent) {
	defer func() {
		b.ev.AgentOffline <- agent
	}()
	for {
		msg, err := agent.ReadMessage(0)
		if err != nil {
			log.Errorf("read message from %s: %s", agent, err)
			return
		}
		switch m := msg.(type) {
		case DataMessage:
			b.ev.DispatchResponse <- BEvDispatchMessage{
				Msg:   m,
				Agent: agent,
			}
		case TextMessage:
			log.Debugf("text message from %s: %s", agent, m.Content)
		case ErrorMessage:
			log.Debugf("error message from %s: %s", agent, m.Content)
		default:
			log.Infof("received an unsupported message from %s", agent)
			return
		}
	}
}

func (b *Broker) eh_AgentOnline(agent *Agent) {
	log.Infof("agent %s online", agent)
	b.agents[agent.ID] = agent
	go b.recvAgentMessage(agent)
}

func (b *Broker) eh_AgentOffline(agent *Agent) {
	log.Infof("agent %s offline", agent)
	agent.conn.Close()
	delete(b.agents, agent.ID)
}

func (b *Broker) eh_DispatchRequest(e BEvDispatchMessage) {

}

func (b *Broker) eh_DispatchResponse(e BEvDispatchMessage) {
	tf, ok := e.Agent.tfs[e.Msg.TID]
	if !ok {
		return
	}
	tf.Response.Write(e.Msg.Data)
}

func (b *Broker) eh_CreateTransferer(e BrokerEvCreateTransferer) {
	route, ok := b.route[e.Host]
	if !ok {
		e.future.Reject(HErrNoRouteRecort)
		return
	}

	agent, ok := b.agents[route.AgentID]
	if !ok {
		e.future.Reject(HErrAgentNotOnline)
		return
	}

	tid := strconv.FormatUint(114514, 10)
	tf := Transferer{
		Request:  NewBlockedBuffer(),
		Response: NewBlockedBuffer(),
	}
	agent.tfs[tid] = tf
	e.future.Resolve(&tf)

	go func() {
		buf := make([]byte, 16*1024)
		for serial := 0; ; serial++ {
			n, err := tf.Request.Read(buf)
			if err != nil {
				tf.Request.SetError(err)
				break
			}
			agent.SendMessage(DataMessage{
				TID:  tid,
				Data: buf[:n],
			})
		}
	}()
}

func (b *Broker) acceptHTTPRequest(lsn net.Listener) {
	for {
		conn, err := lsn.Accept()
		if err != nil {
			break
		}
		go b.handleHTTPRequest(conn)
	}
}

func (b *Broker) handleHTTPRequest(conn net.Conn) {
	var tunnel io.ReadWriteCloser
	var respReader *bufio.Reader
	var req *http.Request
	var resp *http.Response
	var err error
	reqReader := bufio.NewReader(conn)

	defer func() {
		if he, ok := err.(HTTPError); ok {
			he.Write(bufio.NewWriter(conn))
		}
		conn.Close()
	}()

	req, err = http.ReadRequest(reqReader)
	if err != nil {
		return
	}

	for {
		// FIXME: race condition
		req.Host = b.route[req.Host].Host

		req.Write(tunnel)
		resp, err = http.ReadResponse(respReader, req)
		if err != nil {
			break
		}
		resp.Write(conn)

		req, err = http.ReadRequest(reqReader)
		if err != nil {
			break
		}
	}

	tunnel.Close()
}

func (b *Broker) CreateTransferer(host string) (tf *Transferer, err error) {
	future := NewFuture()
	b.ev.CreateTransferer <- BrokerEvCreateTransferer{
		Host:   host,
		future: future,
	}
	val, err := future.Result()
	if err == nil {
		tf = val.(*Transferer)
	}
	return
}
