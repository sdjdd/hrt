package main

import (
	"bufio"
	"errors"
	"fmt"
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
	for {
		msg, err := agent.ReadMessage(0)
		if err != nil {
			break
		}

		switch m := msg.(type) {
		case DataMessage:
			b.ev.DispatchResponse <- BEvDispatchMessage{
				Agent: agent,
				Msg:   m,
			}
		case TextMessage:
			log.Debugf("text message from %s: %s", agent, m.Content)
		case ErrorMessage:
			log.Debugf("error message from %s: %s", agent, m.Content)
		case CloseMessage:
			log.Debugf("transferer %s closed: %s", m.TID, m.Reason)
		default:
			log.Infof("received a unknown message from %s", agent)
		}
	}

	b.ev.AgentOffline <- agent
}

func (b *Broker) eh_AgentOnline(agent *Agent) {
	log.Infof("agent %s online", agent)
	b.agents[agent.ID] = agent
	go b.recvAgentMessage(agent)
}

func (b *Broker) eh_AgentOffline(agent *Agent) {
	log.Infof("agent %s offline", agent)
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

	tid := strconv.FormatUint(agent.gentid(), 10)
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
				Serial: serial,
				TID:    tid,
				Host:   route.Host,
				Data:   buf[:n],
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
	var respReader *bufio.Reader
	var tf *Transferer
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

	for {
		req, err = http.ReadRequest(reqReader)
		if err != nil {
			return
		}

		if tf == nil {
			tf, err = b.CreateTransferer(req.Host)
			if err != nil {
				return
			}
			respReader = bufio.NewReader(tf.Response)
		}

		// FIXME: race condition
		req.Host = b.route[req.Host].Host

		req.Write(tf.Request)
		resp, err = http.ReadResponse(respReader, req)
		if err != nil {
			return
		}
		resp.Write(conn)

		if resp.Close {
			return
		}
	}

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

func (b *Broker) acceptAgent(lsn net.Listener) {
	for {
		conn, err := lsn.Accept()
		if err != nil {
			break
		}
		agent := &Agent{
			conn: conn,
			msgr: NewMessageReader(conn),
			tfs:  make(map[string]Transferer),
		}
		go b.auth(agent)
	}
}
