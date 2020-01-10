package main

type BrokerEvent struct {
	Online  chan *Agent
	Offline chan *Agent
	Recv    chan BrokerRecvMsgEvent
	Send    chan BrokerSendMsgEvent
	Request chan httpRequest

	CreateTransferer chan CreateTransfererEvent
}

type AgentEvent struct {
	Connected chan struct{}
	LostConn  chan error
	RecvMsg   chan Transferable
	SendMsg   chan Message
}

type BrokerRecvMsgEvent struct {
	Agent *Agent
	Msg   Transferable
}

type BrokerSendMsgEvent struct {
	AgentID string
	Msg     Transferable
}

type CreateTransfererEvent struct {
	Host     string
	ResultCh chan<- Result
}

type Result struct {
	Err error
	Val interface{}
}

func NewBrokerEvent() *BrokerEvent {
	return &BrokerEvent{
		Online:           make(chan *Agent),
		Offline:          make(chan *Agent),
		Recv:             make(chan BrokerRecvMsgEvent, 16),
		Send:             make(chan BrokerSendMsgEvent, 16),
		Request:          make(chan httpRequest),
		CreateTransferer: make(chan CreateTransfererEvent),
	}
}

func NewAgentEvent() *AgentEvent {
	return &AgentEvent{
		Connected: make(chan struct{}),
		LostConn:  make(chan error),
		RecvMsg:   make(chan Transferable),
		SendMsg:   make(chan Message),
	}
}
