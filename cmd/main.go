package main

import (
	"flag"

	"github.com/rpccloud/dbot"
)

func main() {
	cfgFile := ""
	flag.StringVar(
		&cfgFile,
		"config",
		"",
		"set config file",
	)

	jobName := ""
	flag.StringVar(
		&jobName,
		"job",
		"default",
		"set the name of the job to run",
	)

	flag.Parse()

	ctx := dbot.NewContext(cfgFile, jobName)
	if ctx != nil {
		ctx.Run()
	}
}
