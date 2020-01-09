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
		route   map[string]RouteInfo
		agents  map[string]AgentInfo
		events  *BrokerEvent
		serial  uint64
		session map[string]*RequestTransferer

		Token string
	}
	AgentInfo struct {
		ID   string
		Conn *Connection
	}
	RouteInfo struct {
		AgentID, Host string
	}
	httpRequest struct {
		*http.Request
		Conn net.Conn
	}
)

func NewBroker() *Broker {
	return &Broker{
		route: map[string]RouteInfo{
			"127.0.0.1": {"test-agent", "www.baidu.com:80"},
			"hrt.test":  {"test-agent", "127.0.0.1:9001"},
		},
		agents:  make(map[string]AgentInfo),
		events:  NewBrokerEvent(),
		session: make(map[string]*RequestTransferer),
	}
}

func SendHTTPError(conn net.Conn, status int, format string, a ...interface{}) {
	msg := fmt.Sprintf("hrt error: "+format, a...)
	bufw := bufio.NewWriter(conn)
	fmt.Fprintf(bufw, "HTTP/1.1 %d \r\n", status)
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
			err = fmt.Errorf("accept agent: %w", er)
			break
		}
		go b.auth(NewConnection(conn))
	}
	lsn.Close()
	cancel()

	return
}

func (b *Broker) auth(conn *Connection) {
	var err error
	defer func() {
		if err != nil {
			log.Error("auth agent: ", err)
			conn.Close()
		}
	}()

	msg, err := conn.ReadMessage(time.Second * 10)
	if err != nil {
		return
	} else if msg.Type != MsgAuth {
		err = fmt.Errorf("first message from %s is not AUTH", conn)
		return
	} else if msg.Attr["token"] != b.Token {
		err = fmt.Errorf("invalid token from %s", conn)
		return
	}

	id := msg.Attr["id"]
	if id == "" {
		err = fmt.Errorf("empty id from %s", conn)
		return
	} else if err = conn.SendOK(); err != nil {
		err = fmt.Errorf("send OK message: %s", err)
		return
	}

	b.events.Online <- AgentInfo{id, conn}
}

func (b *Broker) EventLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case agent := <-b.events.Online:
			if _, ok := b.agents[agent.ID]; ok {
				agent.Conn.Close()
				log.Errorf("agent id %s already exists", agent.ID)
			} else {
				b.agents[agent.ID] = agent
				go b.HandleAgent(agent)
				log.Infof("agent %s@%s online", agent.ID, agent.Conn)
			}

		case e := <-b.events.Recv:
			rt, ok := b.session[e.Msg.Attr["serial"]]
			if !ok {
				log.Error("request not exists")
				return
			}
			rt.rch <- e.Msg.Data
			log.Info("receive: ", string(e.Msg.Data))

		case e := <-b.events.Send:
			agent, ok := b.agents[e.AgentID]
			if !ok {
				log.Error("agent is offline")
				continue
			}
			agent.Conn.SendMessage(e.Msg)

		case agent := <-b.events.Offline:
			agent.Conn.Close()
			delete(b.agents, agent.ID)
			log.Infof("agent %s@%s offline", agent.ID, agent.Conn)

		case req := <-b.events.Request:
			route, ok := b.route[req.Host]
			if !ok {
				SendHTTPError(req.Conn, 404, "no route record for this host")
				continue
			}
			_, ok = b.agents[route.AgentID]
			if !ok {
				SendHTTPError(req.Conn, 503, "agent for this host is offline")
				continue
			}
			serialstr := strconv.FormatUint(b.serial, 32)
			b.serial++

			reqw := b.NewRequestTransferer(serialstr, route.AgentID, route.Host)
			b.session[serialstr] = reqw
			req.Host = route.Host
			go func() {
				req.Write(reqw)
				reqw.Flush()
				log.Info("start to read resp")
				resp, err := http.ReadResponse(bufio.NewReader(reqw), req.Request)
				if err != nil {
					log.Error("read http response: ", err)
					return
				}

				resp.Write(req.Conn)
				req.Conn.Close()
			}()
		}
	}
}

func (b *Broker) HandleAgent(agent AgentInfo) {
	for {
		msg, err := agent.Conn.ReadMessage(0)
		if err != nil {
			break
		}
		b.events.Recv <- BrokerMsgEvent{agent.ID, msg}
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
	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		log.Error("read http request: ", err)
		return
	}
	b.events.Request <- httpRequest{req, conn}
}

func (b *Broker) NewRequestTransferer(serial, agentID, host string) *RequestTransferer {
	return &RequestTransferer{
		wch:     b.events.Send,
		rch:     make(chan []byte),
		buf:     make([]byte, 16*1024),
		Serial:  serial,
		AgentID: agentID,
		Host:    host,
	}
}

func (b *Broker) CloseRequestTransferer(rt *RequestTransferer) {

}

type RequestTransferer struct {
	wch      chan BrokerMsgEvent
	rch      chan []byte
	buf      []byte
	wp       int
	lastRead []byte

	Serial  string
	AgentID string
	Host    string
}

func (w *RequestTransferer) Write(p []byte) (n int, err error) {
	var rp int
	for rp < len(p) {
		copied := copy(w.buf[w.wp:], p[rp:])
		w.wp += copied
		rp += copied
		if w.wp == len(w.buf) {
			w.Flush()
		}
	}
	return len(p), nil
}

func (w *RequestTransferer) Flush() error {
	data := append(w.buf[:0:0], w.buf[:w.wp]...)
	msg := Message{
		Type: MsgData,
		Len:  w.wp,
		Data: data,
		Attr: map[string]string{
			"serial": w.Serial,
			"host":   w.Host,
		},
	}
	w.wch <- BrokerMsgEvent{
		AgentID: w.AgentID,
		Msg:     msg,
	}
	w.wp = 0
	return nil
}

func (r *RequestTransferer) Read(p []byte) (n int, err error) {
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
