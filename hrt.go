package main

import (
	"fmt"
	"os"

	"go.uber.org/zap"
)

type (
	BrokerConf struct {
		listen string
		http   string
		route  string
		token  string
	}
	AgentConf struct {
		addr  string
		token string
		id    string
	}
)

var log *zap.SugaredLogger

// var exit = make(chan struct{})

func init() {
	// exch := make(chan os.Signal, 1)
	// go func() {
	// 	<-exch
	// 	signal.Stop(exch)
	// 	close(exch)
	// 	close(exit)
	// }()
	// signal.Notify(exch, syscall.SIGINT)

	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Fprintf(os.Stderr, "initialize logger failed: %s", err)
		os.Exit(1)
	}
	log = logger.Sugar()
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func StartBroker(conf BrokerConf) {
	b := Broker{Token: conf.token}
	b.Init()

	if conf.route != "" {
		route, err := ReadJsonRoute(conf.route)
		if err != nil {
			log.Fatal("read route file: ", err)
		}
		b.route = route
		log.Info("successfully loaded the routing information from ", conf.route)
		for host, record := range route {
			log.Debugf("route: %s => %s[%s]", host, record.AgentID, record.Host)
		}
	}

	err := b.Serve(conf.listen, conf.http)
	if err != nil {
		log.Error("start broker: ", err)
	}
}

func StartAgent(conf AgentConf) {
	agent := NewAgent(conf.id)
	err := agent.Connect(conf.addr, conf.token)
	if err != nil {
		log.Error("connect to broker: ", err)
		return
	}
}
