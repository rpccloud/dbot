package main

import (
	"flag"

	"github.com/rpccloud/dbot"
)

func main() {
	configPath := ""
	jobName := ""
	flag.StringVar(
		&configPath,
		"config",
		"./config.json",
		"set config path",
	)
	flag.StringVar(
		&jobName,
		"job",
		"install",
		"set job name",
	)
	flag.Parse()

	mgr := dbot.NewManager()
	mgr.Run(configPath, jobName)
	mgr.Close()
}
