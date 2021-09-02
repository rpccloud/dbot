package main

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/rpccloud/dbot"
)

func main() {
	redColor := color.New(color.FgRed, color.Bold)
	entryPath := ""
	flag.StringVar(
		&entryPath,
		"config",
		"",
		"set config path",
	)
	flag.Parse()

	if entryPath == "" {
		if _, e := os.Stat("./main.yaml"); e == nil {
			entryPath = "./main.yaml"
		} else if _, e := os.Stat("./main.yml"); e == nil {
			entryPath = "./main.yml"
		} else if _, e := os.Stat("./main.json"); e == nil {
			entryPath = "./main.json"
		} else {
			absPath, _ := filepath.Abs(".")
			_, _ = redColor.Printf(
				"could not find main.yaml or main.yml or main.json in dir %s\n",
				absPath,
			)
			return
		}
	}

	absEntryPath, e := filepath.Abs(entryPath)
	if e != nil {
		_, _ = redColor.Printf("could not load %s\n", entryPath)
		return
	}

	rootCtx := dbot.NewRootContext(absEntryPath)
	if rootCtx.Prepare() {
		rootCtx.Run()
	}
}
