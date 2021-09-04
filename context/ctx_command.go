package context

import "fmt"

type CommandContext struct {
	BaseContext
}

func (p *CommandContext) Clone(pathFormat string, a ...interface{}) Context {
	return &CommandContext{
		BaseContext: *p.BaseContext.copy(pathFormat, a...),
	}
}

func (p *CommandContext) Run() bool {
	fmt.Println(p.cmd)
	return false
}
