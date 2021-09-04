package context

import (
	"strings"
)

type CmdContext struct {
	BaseContext
}

func (p *CmdContext) Clone(format string, a ...interface{}) Context {
	return &CmdContext{
		BaseContext: *p.BaseContext.copy(format, a...),
	}
}

func (p *CmdContext) getRunners(runOn string) []Runner {
	rootContext := p.GetRootContext()
	if rootContext == nil {
		p.LogError("kernel error: rootContext is nil")
		return nil
	}

	ret := make([]Runner, 0)

	for _, groupName := range strings.Split(runOn, ",") {
		if groupName = strings.TrimSpace(groupName); groupName != "" {
			if groupName == "local" {
				ret = append(ret, rootContext.runnerMap["local"])
			} else {
				sshGroup, ok := rootContext.runnerGroupMap[groupName]

				if !ok {
					p.LogError("could not find SSHGroup \"%s\"", groupName)
					return nil
				}

				for _, runnerName := range sshGroup {
					runner, ok := rootContext.runnerMap[runnerName]
					if !ok {
						p.LogError("could not find runner \"%s\"", runnerName)
						return nil
					}
					ret = append(ret, runner)
				}
			}
		}
	}

	if len(ret) == 0 {
		p.LogError("could not find any runners")
		return nil
	}

	return ret
}

func (p *CmdContext) newCommandContext(cmd *Command, parseEnv Env) Context {
	ret := &CmdContext{
		BaseContext: BaseContext{
			parent:   p,
			cmd:      cmd,
			runners:  append([]Runner{}, p.runners...),
			path:     p.path,
			config:   p.config,
			parseEnv: parseEnv,
		},
	}

	// Parse cmd.On
	cmdOn := ret.ParseCommand().On

	// Use default runners
	if cmdOn == "" {
		return ret
	}

	// Use defined runners
	runners := p.getRunners(cmdOn)
	if runners == nil {
		return nil
	}

	ret.runners = runners
	return ret
}

func (p *CmdContext) Run() bool {
	if len(p.runners) == 0 {
		p.Clone("kernel error: runners must be checked in previous call")
		return false
	} else if len(p.runners) == 1 {
		switch p.cmd.Type {
		case "job":
			return p.runJob()
		case "cmd":
			return p.runCmd()
		case "script":
			return p.runScript()
		default:
			p.Clone("kernel error: type must be checked in previous call")
			return false
		}
	} else {
		for _, runner := range p.runners {
			ctx := p.Clone(p.GetPath()).(*CmdContext)
			ctx.runners = []Runner{runner}
			if !ctx.Run() {
				return false
			}
		}

		return true
	}
}

func (p *CmdContext) runJob() bool {
	// Parse command
	cmd := p.ParseCommand()
	if cmd == nil {
		return false
	}

	// Load config
	config := &JobConfig{}
	if !p.LoadConfig(config) {
		return false
	}

	// Get job
	job, ok := config.Jobs[cmd.Exec]
	if !ok {
		p.Clone("jobs.%s", cmd.Exec).
			LogError("could not find job \"%s\"", cmd.Exec)
		return false
	}

	// Make jobEnv
	rootEnv := p.GetRootEnv()
	jobEnv := rootEnv.Merge(rootEnv.ParseEnv(job.Env)).Merge(cmd.Env)

	// If the commands are run in sequence, run them one by one and return
	if !job.Async {
		for i := 0; i < len(job.Commands); i++ {
			ctx := p.Clone("jobs.%s.commands[%d]", cmd.Exec, i).(*CmdContext).
				newCommandContext(job.Commands[i], jobEnv)
			if !ctx.Run() {
				return false
			}
		}

		return true
	}

	// The commands are run async
	waitCH := make(chan bool, len(job.Commands))

	for i := 0; i < len(job.Commands); i++ {
		go func(idx int) {
			ctx := p.Clone("jobs.%s.commands[%d]", cmd.Exec, idx).(*CmdContext).
				newCommandContext(job.Commands[idx], jobEnv)
			waitCH <- ctx.Run()
		}(i)
	}

	// Wait for all commands to complete
	ret := true
	for i := 0; i < len(job.Commands); i++ {
		if !<-waitCH {
			ret = false
		}
	}

	return ret
}

func (p *CmdContext) runScript() bool {
	return false
}

func (p *CmdContext) runCmd() bool {
	return p.runners[0].Run(p)
}
