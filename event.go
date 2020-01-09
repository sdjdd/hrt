package main

type BrokerEvent struct {
	Online  chan AgentInfo
	Offline chan AgentInfo
	Recv    chan BrokerMsgEvent
	Send    chan BrokerMsgEvent
	Request chan httpRequest
}

type AgentEvent struct {
	RecvMsg chan Message
	SendMsg chan Message
}

type BrokerMsgEvent struct {
	AgentID string
	Msg     Message
}

func NewBrokerEvent() *BrokerEvent {
	return &BrokerEvent{
		Online:  make(chan AgentInfo),
		Offline: make(chan AgentInfo),
		Recv:    make(chan BrokerMsgEvent, 16),
		Send:    make(chan BrokerMsgEvent, 16),
		Request: make(chan httpRequest),
	}
}

func NewAgentEvent() *AgentEvent {
	return &AgentEvent{
		RecvMsg: make(chan Message),
		SendMsg: make(chan Message),
	}
}
