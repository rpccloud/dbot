package context

import "fmt"

type CommandContext struct {
	BaseContext
}

func (p *CommandContext) Run() bool {
	fmt.Println(p.cmd)
	return false
}
