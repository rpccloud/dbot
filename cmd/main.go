package main

import (
	"flag"

	"github.com/rpccloud/dbot/context"
)

func main() {
	entryPath := ""
	flag.StringVar(
		&entryPath,
		"config",
		"",
		"set config path",
	)
	flag.Parse()

	rootCtx := context.NewRootContext(entryPath)
	if rootCtx != nil {
		rootCtx.Run()
	}
}
