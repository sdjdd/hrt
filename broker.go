package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

type (
	Broker struct {
		route  map[string]RouteInfo
		agents map[string]*Agent
		events *BrokerEvent

		Token string
	}
	RouteInfo struct {
		AgentID, Host string
	}
	httpRequest struct {
		*http.Request
		Conn net.Conn
	}
	HttpError struct {
		Status  int
		Msg     string
		Content string
	}
)

func (e HttpError) Error() string { return e.Content }

func NewBroker() *Broker {
	return &Broker{
		route: map[string]RouteInfo{
			"127.0.0.1": {"gtmdc3p1", "www.baidu.com:80"},
			"hrt.test":  {"gtmdc3p1", "tjjlmd.com:80"},
		},
		agents: make(map[string]*Agent),
		events: NewBrokerEvent(),
	}
}

func SendHTTPError(conn net.Conn, err HttpError) {
	msg := "hrt error: " + err.Content
	bufw := bufio.NewWriter(conn)
	fmt.Fprintf(bufw, "HTTP/1.1 %d %s\r\n", err.Status, err.Msg)
	fmt.Fprintf(bufw, "Content-Length: %d\r\n", len(msg))
	bufw.WriteString("\r\n")
	bufw.WriteString(msg)
	bufw.Flush()
	conn.Close()
}

func (b *Broker) Serve(addr, httpAddr string) (err error) {
	lsn, err := net.Listen("tcp", addr)
	if err != nil {
		return
	}
	log.Info("hrt broker listen on ", lsn.Addr())

	ctx, cancel := context.WithCancel(context.Background())
	go b.EventLoop(ctx)
	go b.ServeHTTP(httpAddr)

	for {
		conn, er := lsn.Accept()
		if er != nil {
			err = fmt.Errorf("accept agent: %s", er)
			break
		}

		agent := &Agent{
			conn: conn,
			msgr: NewMessageReader(conn),
			tfs:  make(map[string]io.ReadWriter),
		}
		go b.auth(agent)
	}
	lsn.Close()
	cancel()

	return
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
		err = fmt.Errorf("first message from %s is not AUTH", agent)
		return
	} else if m.Token != b.Token {
		err = fmt.Errorf("invalid token from %s", agent)
		return
	}

	if agent.ID = m.ID; agent.ID == "" {
		err = fmt.Errorf("empty id from %s", agent)
		return
	}
	if err = agent.SendOK(); err != nil {
		err = fmt.Errorf("send OK message: %s", err)
		return
	}

	b.events.Online <- agent
}

func (b *Broker) EventLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case agent := <-b.events.Online:
			if _, ok := b.agents[agent.ID]; ok {
				log.Errorf("agent id %s already exists", agent.ID)
				agent.conn.Close()
			} else {
				log.Infof("agent %s online", agent)
				b.agents[agent.ID] = agent
				go b.HandleAgent(agent)
			}

		case e := <-b.events.Recv:
			switch m := e.Msg.(type) {
			case DataMessage:
				if tf, ok := e.Agent.tfs[m.TID]; ok {
					tf.(*Transferer).rch <- m.Data
				} else {
					log.Errorf("transferer %s has been closed", m.TID)
				}
			}

		case e := <-b.events.Send:
			if agent, ok := b.agents[e.AgentID]; ok {
				agent.SendMessage(e.Msg)
			} else {
				log.Error("agent is offline")
			}

		case agent := <-b.events.Offline:
			agent.conn.Close()
			delete(b.agents, agent.ID)
			log.Infof("agent %s offline", agent)

		case e := <-b.events.CreateTransferer:
			b.handleCreateTrensferer(e)
		}
	}
}

func (b *Broker) SendToAgent(agentID string, msg Transferable) {
	b.events.Send <- BrokerSendMsgEvent{agentID, msg}
}

func (b *Broker) HandleAgent(agent *Agent) {
	for {
		msg, err := agent.ReadMessage(0)
		if err != nil {
			break
		}
		b.events.Recv <- BrokerRecvMsgEvent{agent, msg}
	}
	b.events.Offline <- agent
}

func (b *Broker) ServeHTTP(addr string) {
	lsn, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error("start http service: ", err)
		return
	}
	for {
		conn, err := lsn.Accept()
		if err != nil {
			log.Error("accept http connection: ", err)
			break
		}
		go b.handleHTTPRequest(conn)
	}
	lsn.Close()
}

func (b *Broker) handleHTTPRequest(conn net.Conn) {
	defer conn.Close()
	remote := conn.RemoteAddr().String()
	log.Debug("new http request come: ", remote)

	reqbuf := bufio.NewReader(conn)
	var reqHost string
	var tf io.ReadWriteCloser
	var respbuf *bufio.Reader
	for {
		req, err := http.ReadRequest(reqbuf)
		if err != nil {
			if err == io.EOF {
				log.Debug("http request read finish: ", remote)
			} else {
				log.Errorf("read http request from %s: %s", remote, err)
			}
			break
		}

		if tf == nil {
			if reqHost = req.Host; req.Host == "" {
				SendHTTPError(conn, HttpError{400, "Bad Request", "missing host request header"})
				break
			}

			if t, err := b.CreateTransferer(req.Host); err != nil {
				switch er := err.(type) {
				case HttpError:
					SendHTTPError(conn, er)
				default:
					log.Error(er)
				}
				break
			} else {
				tf = t
				respbuf = bufio.NewReader(tf)
			}
		} else if req.Host != reqHost {
			log.Error("request different hosts on the same TCP connection")
			break
		}

		req.Host = tf.(*Transferer).host
		req.Write(tf)
		resp, err := http.ReadResponse(respbuf, req)
		if err != nil {
			log.Error(err)
			break
		}
		resp.Write(conn)
	}
	if tf != nil {
		tf.Close()
	}
}

func (b *Broker) handleCreateTrensferer(e CreateTransfererEvent) {
	var result Result
	defer func() {
		e.ResultCh <- result
		close(e.ResultCh)
	}()

	route, ok := b.route[e.Host]
	if !ok {
		result.Err = HttpError{404, "Not Found", "no route record for this host"}
		return
	}
	agent, ok := b.agents[route.AgentID]
	if !ok {
		result.Err = HttpError{503, "Agent Offline", "agent for this host is offline"}
		return
	}

	tid := strconv.FormatUint(agent.gentid(), 10)
	tf := NewTransferer(tid, agent.ID, route.Host)
	tf.wch = b.events.Send
	agent.tfs[tid] = tf
	result.Val = tf
}

func (b *Broker) CreateTransferer(host string) (tf io.ReadWriteCloser, err error) {
	retch := make(chan Result)
	b.events.CreateTransferer <- CreateTransfererEvent{host, retch}
	result := <-retch
	if result.Err == nil {
		tf = result.Val.(io.ReadWriteCloser)
	} else {
		err = result.Err
	}
	return
}

type Transferer struct {
	id       string
	host     string
	agentID  string
	wch      chan BrokerSendMsgEvent
	rch      chan []byte
	lastRead []byte

	OnClose func(id string)
}

func NewTransferer(id, agentID, host string) *Transferer {
	return &Transferer{
		id:      id,
		host:    host,
		agentID: agentID,
		rch:     make(chan []byte),
	}
}

func (t *Transferer) Write(p []byte) (n int, err error) {
	data := make([]byte, len(p))
	n = copy(data, p)
	msg := Message{
		Type: MsgData,
		Data: data,
		Attr: map[string]string{
			"tid":  t.id,
			"host": t.host,
		},
	}
	t.wch <- BrokerSendMsgEvent{t.agentID, msg}
	return
}

func (r *Transferer) Read(p []byte) (n int, err error) {
	var data []byte
	if r.lastRead == nil {
		var ok bool
		if data, ok = <-r.rch; !ok {
			err = io.EOF
			return
		}
	} else {
		data = r.lastRead
		r.lastRead = nil
	}
	if len(data) > len(p) {
		r.lastRead = data[len(p):]
	}
	n = copy(p, data)
	return
}

func (t *Transferer) Close() error {
	msg := Message{
		Type: MsgData,
		Attr: map[string]string{
			"tid":   t.id,
			"close": "EOF",
		},
	}
	t.wch <- BrokerSendMsgEvent{t.agentID, msg}
	if t.OnClose != nil {
		t.OnClose(t.id)
	}
	return nil
}
