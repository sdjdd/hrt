package main

import (
	"fmt"
	"os"

	"go.uber.org/zap"
)

var log *zap.SugaredLogger

func init() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Fprintf(os.Stderr, "initialize logger failed: %s", err)
		os.Exit(1)
	}
	log = logger.Sugar()
}

func main() {

	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage:\n  hrt [command]\n")
		os.Exit(1)
	}

	switch cmd := os.Args[1]; cmd {
	case "serve":
		StartBroker()
	case "connect":
		StartAgent()
	}
}

func StartBroker() {
	brk := NewBroker()
	err := brk.Serve("127.0.0.1:8081", "127.0.0.1:80")
	if err != nil {
		log.Fatal("connect to broker: ", err)
	}
}

func StartAgent() {
	agent := NewAgent("gtmdc3p1")
	go func() {
		err := agent.Connect("127.0.0.1:8081", "")
		if err != nil {
			log.Error("connect to broker: ", err)
			return
		}
	}()
	agent.EventLoop()
}
