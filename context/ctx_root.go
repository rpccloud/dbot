package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func loadSSHGroup(ctx Context, key string, list []*Remote) bool {
	rootContext := ctx.GetRootContext()

	if rootContext == nil {
		ctx.LogError("kernel error: could not get RootContext")
		return false
	}

	runnerGroup := make([]string, 0)
	for idx, it := range list {
		host := Env{}.ParseString(it.Host, "", true)
		user := Env{}.ParseString(it.User, os.Getenv("USER"), true)
		port := Env{}.ParseString(it.Port, "22", true)

		id := fmt.Sprintf("%s@%s:%s", user, host, port)

		if _, ok := rootContext.runnerMap[id]; !ok {
			ssh := NewSSHRunner(
				ctx.Clone("%s[%d]", ctx.GetPath(), idx), port, user, host,
			)

			if ssh == nil {
				return false
			}

			rootContext.runnerMap[id] = ssh
		}

		runnerGroup = append(runnerGroup, id)
	}

	rootContext.runnerGroupMap[key] = runnerGroup
	return true
}

type RootContext struct {
	rootConfig     *RootConfig
	runnerGroupMap map[string][]string
	runnerMap      map[string]Runner
	runContexts    []*CommandContext
	BaseContext
}

func NewRootContext(config string) *RootContext {
	localRUnner := &LocalRunner{}

	ret := &RootContext{
		rootConfig:     &RootConfig{},
		runnerGroupMap: make(map[string][]string),
		runnerMap:      make(map[string]Runner),
		runContexts:    make([]*CommandContext, 0),
		BaseContext: BaseContext{
			parent:  nil,
			cmd:     nil,
			runners: []Runner{localRUnner},
			path:    "",
			config:  config,
			env:     Env{},
		},
	}

	// Add local runner
	ret.runnerMap["local"] = localRUnner

	// Make sure the BaseContext.config is an absolute path
	absConfig, e := filepath.Abs(config)
	if e != nil {
		ret.LogError(e.Error())
		return nil
	}
	ret.BaseContext.config = absConfig

	// Load
	if !ret.load() {
		return nil
	}

	return ret
}

func (p *RootContext) Clone(pathFormat string, a ...interface{}) Context {
	// override the BaseContext Clone method
	return &RootContext{
		rootConfig:     p.rootConfig,
		runnerGroupMap: p.runnerGroupMap,
		runnerMap:      p.runnerMap,
		runContexts:    append([]*CommandContext{}, p.runContexts...),
		BaseContext:    *(p.BaseContext.Clone(pathFormat, a...)).(*BaseContext),
	}
}

func (p *RootContext) newImportContext(
	path string, config string,
) *ImportContext {
	absConfig, ok := p.AbsPath(config)
	if !ok {
		return nil
	}

	return &ImportContext{
		BaseContext: BaseContext{
			parent:  p,
			cmd:     nil,
			runners: append([]Runner{}, p.runners...),
			path:    path,
			config:  absConfig,
			env:     Env{},
		},
	}
}

func (p *RootContext) newCommandContext(
	config string, jobName string, env Env,
) *CommandContext {
	absConfig, ok := p.AbsPath(config)
	if !ok {
		return nil
	}

	return &CommandContext{
		BaseContext: BaseContext{
			parent: p,
			cmd: &Command{
				Type:   "job",
				Exec:   jobName,
				On:     "local",
				Inputs: []string{},
				Env:    env.Merge(nil),
				Config: absConfig,
			},
			runners: append([]Runner{}, p.runners...),
			path:    "",
			config:  absConfig,
			env:     Env{},
		},
	}
}

func (p *RootContext) Run() bool {
	// Check
	if len(p.runContexts) == 0 {
		p.Clone("main").LogError("could not find any tasks")
		return false
	}

	// Run all the contexts
	for _, ctx := range p.runContexts {
		if !ctx.Run() {
			return false
		}
	}

	return true
}

func (p *RootContext) GetRootContext() *RootContext {
	return p
}

func (p *RootContext) load() bool {
	// Load Config
	if !p.LoadConfig(p.rootConfig) {
		return false
	}

	// Load imports
	for key, it := range p.rootConfig.Imports {
		name := strings.TrimSpace(it.Name)
		config := strings.TrimSpace(it.Config)
		ctx := p.Clone("imports.%s", key).
			GetRootContext().
			newImportContext(name, config)

		if sshGroup := ctx.GetSSHGroup(name); sshGroup == nil {
			return false
		} else if !loadSSHGroup(ctx, key, sshGroup) {
			return false
		} else {
			continue
		}
	}

	// Load remotes
	for key, sshGroup := range p.rootConfig.Remotes {
		if len(sshGroup) == 0 {
			p.Clone("imports.%s", key).LogError("SSHGroup \"%s\" is empty", key)
			return false
		} else if !loadSSHGroup(p.Clone("remotes.%s", key), key, sshGroup) {
			return false
		} else {
			continue
		}
	}

	// Load the tasks that will run on main
	for idx, taskName := range p.rootConfig.Main {
		taskName = strings.TrimSpace(taskName)

		// Get task
		task, ok := p.rootConfig.Tasks[taskName]
		if !ok {
			p.Clone("main[%d]", idx).LogError("task \"%s\" not found")
			return false
		}

		// Load task env
		taskEnv := p.GetRootEnv().ParseEnv(task.Env)
		contextEnv := p.GetRootEnv().Merge(taskEnv)
		for key, it := range task.Inputs {
			ctx := p.Clone("tasks.%s.inputs.%s", taskName, key)
			itDesc := contextEnv.ParseString(it.Desc, "input "+key+": ", false)
			itType := contextEnv.ParseString(it.Type, "text", true)
			value, ok := ctx.GetUserInput(itDesc, itType)
			if !ok {
				return false
			}
			taskEnv[key] = contextEnv.ParseString(value, "", false)
		}
		contextEnv = p.GetRootEnv().Merge(taskEnv)

		// Create a job command context, and add it to the run list
		p.runContexts = append(
			p.runContexts,
			p.Clone("tasks.%s", taskName).GetRootContext().newCommandContext(
				contextEnv.ParseString(task.Config, "", true),
				contextEnv.ParseString(task.Run, "", true),
				taskEnv,
			),
		)
	}

	return true
}
