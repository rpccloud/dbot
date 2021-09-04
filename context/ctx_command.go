package context

import "fmt"

type CommandContext struct {
	cmd *Command
	BaseContext
}

func (p *CommandContext) Clone(pathFormat string, a ...interface{}) Context {
	return &CommandContext{
		cmd:         p.cmd,
		BaseContext: *p.BaseContext.copy(pathFormat, a...),
	}
}

func (p *CommandContext) Run() bool {
	fmt.Println(p.cmd)
	return false
}
