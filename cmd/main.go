package main

import (
	"flag"
	"os"

	"github.com/fatih/color"
	"github.com/rpccloud/dbot"
)

var (
	greenColor   = color.New(color.FgGreen, color.Bold)
	redColor     = color.New(color.FgRed, color.Bold)
	blueColor    = color.New(color.FgBlue, color.Bold)
	yellowColor  = color.New(color.FgYellow, color.Bold)
	magentaColor = color.New(color.FgMagenta, color.Bold)
)

const jsonUsage = `{
  "name": "sayHello demo",
  "jobs": {
    "sayHello": {
      "commands": [
        {"exec": "echo \"Hello World\""}
      ]
    }
  }
}`

const yamlUsage = `name: sayHello demo
jobs:
  sayHello:
    commands:
    - exec: echo "Hello World"`

func printUsage() {
	greenColor.Println("Usage: dbot config file support json or yaml(yml)")
	yellowColor.Println("For json: ")
	magentaColor.Println("Define config.json:")
	blueColor.Println(jsonUsage)
	magentaColor.Println("Run:")
	blueColor.Println("dbot -config=\"config.json\" job=\"sayHello\"")
	blueColor.Println()
	yellowColor.Println("For yaml: ")
	magentaColor.Println("Define config.yaml:")
	blueColor.Println(yamlUsage)
	magentaColor.Println("Run:")
	blueColor.Println("dbot -config=\"config.yaml\" job=\"sayHello\"")
	blueColor.Println()
	yellowColor.Println("For more detail, please visit github:")
	blueColor.Println("https://github.com/rpccloud/dbot")
}

func main() {
	configPath := ""
	jobName := ""
	flag.StringVar(
		&configPath,
		"config",
		"",
		"set config path",
	)
	flag.StringVar(
		&jobName,
		"job",
		"install",
		"set job name",
	)
	flag.Parse()

	if configPath == "" {
		if _, e := os.Stat("./config.yaml"); e == nil {
			configPath = "./config.yaml"
			greenColor.Printf("find config file \"%s\"\n", configPath)
		} else if _, e := os.Stat("./config.yml"); e == nil {
			configPath = "./config.yml"
			greenColor.Printf("find config file \"%s\"\n", configPath)
		} else if _, e := os.Stat("./config.json"); e == nil {
			configPath = "./config.json"
			greenColor.Printf("find config file \"%s\"\n", configPath)
		} else {
			redColor.Println("could not find config file")
			printUsage()
			return
		}
	}

	mgr := dbot.NewManager()
	mgr.Run(configPath, jobName)
}
