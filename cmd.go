package main

import (
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use: "hrt",
	}
	brokerCmd = &cobra.Command{
		Use:   "serve",
		Short: "Start a hrt broker",
		Run:   brokerCmdHandler,
	}
	agentCmd = &cobra.Command{
		Use:   "connect [address]",
		Short: "Connect to hrt broker",
		Args:  cobra.MinimumNArgs(1),
		Run:   agentCmdHandler,
	}
)

func init() {
	bflags := brokerCmd.Flags()
	bflags.StringP("listen", "l", ":9090", "hrt broker listening address")
	bflags.String("http", ":8080", "http service listening address")
	bflags.String("route", "", "route file path")
	bflags.String("token", "", "")

	aflags := agentCmd.Flags()
	aflags.String("token", "", "")
	aflags.String("id", "", "agent id")
	rootCmd.AddCommand(brokerCmd, agentCmd)
}

func brokerCmdHandler(cmd *cobra.Command, args []string) {
	var conf BrokerConf
	flags := cmd.Flags()
	conf.listen, _ = flags.GetString("listen")
	conf.http, _ = flags.GetString("http")
	conf.token, _ = flags.GetString("token")
	conf.route, _ = flags.GetString("route")
	StartBroker(conf)
}

func agentCmdHandler(cmd *cobra.Command, args []string) {
	var conf AgentConf
	flags := cmd.Flags()
	conf.addr = args[0]
	conf.token, _ = flags.GetString("token")
	conf.id, _ = flags.GetString("id")
	StartAgent(conf)
}
