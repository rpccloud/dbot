package dbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/robertkrimen/otto"
	"golang.org/x/term"
	"gopkg.in/yaml.v2"
)

type Context struct {
	parent         *Context
	runnerGroupMap map[string][]string
	runnerMap      map[string]Runner
	job            *Job
	rawCmd         *Command
	runCmd         *Command
	runners        []Runner
	path           string
	file           string
}

// NewContext create the root context
func NewContext(file string, jobName string) *Context {
	vCtx := &Context{
		runnerGroupMap: map[string][]string{
			"local": {"local"},
		},
		runnerMap: map[string]Runner{
			"local": &LocalRunner{},
		},
		path:    jobName,
		file:    "",
		runners: []Runner{&LocalRunner{}},
		runCmd:  &Command{Env: Env{}},
	}

	ret := vCtx.subContext(&Command{Tag: "job", Exec: jobName, File: file})
	ret.parent = nil
	return ret
}

// subContext create sub Context
func (p *Context) subContext(rawCmd *Command) *Context {
	cmdEnv := p.runCmd.Env.Merge(p.runCmd.Env.ParseEnv(rawCmd.Env))
	// Notice: if rawCmd tag is job, then runCmd.Env will change in init func
	runCmd := &Command{
		Tag:   cmdEnv.ParseString(rawCmd.Tag, "cmd", true),
		Exec:  cmdEnv.ParseString(rawCmd.Exec, "", false),
		On:    cmdEnv.ParseString(rawCmd.On, "", true),
		Stdin: cmdEnv.ParseStringArray(rawCmd.Stdin),
		Env:   cmdEnv,
		Args:  cmdEnv.ParseEnv(rawCmd.Args),
		File:  cmdEnv.ParseString(rawCmd.File, "", true),
	}

	file := p.file
	runners := p.runners
	path := p.path
	job := p.job

	switch runCmd.Tag {
	case "cmd", "script":
		if len(rawCmd.Args) > 0 {
			p.Clone("%s.args", p.path).LogError(
				"unsupported args on tag \"%s\"", runCmd.Tag,
			)
		}

		if rawCmd.File != "" {
			p.Clone("%s.file", p.path).LogError(
				"unsupported file on tag \"%s\"", runCmd.Tag,
			)
		}
	case "job":
		if len(rawCmd.Stdin) > 0 {
			p.Clone("%s.stdin", p.path).LogError(
				"unsupported stdin on tag \"%s\"", runCmd.Tag,
			)
		}

		// Load config
		config := make(map[string]*Job)

		if runCmd.File != "" {
			file = runCmd.File
		}

		if v, ok := p.Clone("%s.file", file).loadConfig(file, &config); ok {
			file = v
		} else {
			return nil
		}

		// Check is the job exist
		if v, ok := config[runCmd.Exec]; ok {
			job = v
		} else {
			p.Clone("%s.exec", p.path).LogError(
				"could not find job \"%s\" in \"%s\"", runCmd.Exec, file,
			)
			return nil
		}

		path = runCmd.Exec
		runCmd.Env = nil
	default:
		p.Clone("%s.tag", p.path).LogError("unsupported tag \"%s\"", runCmd.Tag)
		return nil
	}

	// Set runners
	if runCmd.On != "" {
		if v := p.getRunners(runCmd.On); len(v) > 0 {
			runners = v
		} else {
			return nil
		}
	}

	// clone runnerGroupMap
	runnerGroupMap := make(map[string][]string)
	for key, value := range p.runnerGroupMap {
		runnerGroupMap[key] = value
	}

	ret := &Context{
		runnerGroupMap: runnerGroupMap,
		runnerMap:      p.runnerMap,
		job:            job,
		parent:         p,
		rawCmd:         rawCmd,
		runCmd:         runCmd,
		runners:        runners,
		path:           path,
		file:           file,
	}

	if runCmd.Tag == "job" {
		if !ret.initJob() {
			return nil
		}
	}

	return ret
}

func (p *Context) initJob() bool {
	// init jobEnv
	rootEnv := p.getRootEnv()
	jobEnv := rootEnv.
		Merge(rootEnv.ParseEnv(p.job.Env)).
		Merge(p.runCmd.Args)
	tmpEnv := jobEnv.Merge(Env{})
	for key, it := range p.job.Inputs {
		itDesc := tmpEnv.ParseString(it.Desc, "input "+key+": ", false)
		itType := tmpEnv.ParseString(it.Type, "text", true)
		value, ok := p.Clone("%s.inputs.%s", p.path, key).
			GetUserInput(itDesc, itType)
		if !ok {
			return false
		}
		jobEnv[key] = tmpEnv.ParseString(value, "", false)
	}
	p.runCmd.Env = jobEnv

	// Load imports
	for key, it := range p.job.Imports {
		itName := jobEnv.ParseString(it.Name, "", true)
		itFile := jobEnv.ParseString(it.File, "", true)
		config := make(map[string][]*Remote)

		if absFile, ok := p.Clone("%s.imports.%s", p.path, key).
			loadConfig(itFile, config); !ok {
			return false
		} else if item, ok := config[itName]; !ok {
			return false
		} else {
			sshGroup := (&Context{
				parent:         p,
				runnerGroupMap: p.runnerGroupMap,
				runnerMap:      p.runnerMap,
				path:           itName,
				file:           absFile,
				runners:        p.runners,
			}).loadSSHGroup(item, Env{})

			if sshGroup == nil {
				return false
			}

			p.runnerGroupMap[key] = sshGroup
		}
	}

	// Load remotes
	for key, list := range p.job.Remotes {
		sshGroup := p.Clone("%s.remotes.%s", p.path, key).
			loadSSHGroup(list, jobEnv)

		if sshGroup == nil {
			return false
		}

		p.runnerGroupMap[key] = sshGroup
	}

	return true
}

func (p *Context) loadSSHGroup(list []*Remote, env Env) []string {
	if len(list) == 0 {
		p.LogError("list is empty")
		return nil
	}

	ret := make([]string, 0)
	for idx, it := range list {
		host := env.ParseString(it.Host, "", true)
		user := env.ParseString(it.User, os.Getenv("USER"), true)
		port := env.ParseString(it.Port, "22", true)

		id := fmt.Sprintf("%s@%s:%s", user, host, port)

		if _, ok := p.runnerMap[id]; !ok {
			ssh := NewSSHRunner(
				p.Clone("%s[%d]", p.path, idx), port, user, host,
			)

			if ssh == nil {
				return nil
			}

			p.runnerMap[id] = ssh
		}

		ret = append(ret, id)
	}

	return ret
}

func (p *Context) getRootEnv() Env {
	return Env{
		"KeyESC":   "\033",
		"KeyEnter": "\n",
	}.Merge(Env{
		"ConfigDir": filepath.Dir(p.file),
	})
}

func (p *Context) getRunners(runOn string) []Runner {
	ret := make([]Runner, 0)

	for _, groupName := range strings.Split(runOn, ",") {
		if groupName = strings.TrimSpace(groupName); groupName != "" {
			runnersName, ok := p.runnerGroupMap[groupName]

			if !ok {
				p.LogError("could not find group \"%s\"", groupName)
				return nil
			}

			for _, runnerName := range runnersName {
				runner, ok := p.runnerMap[runnerName]
				if !ok {
					p.LogError("could not find runner \"%s\"", runnerName)
					return nil
				}
				ret = append(ret, runner)
			}
		}
	}

	if len(ret) == 0 {
		p.LogError("could not find any runners")
		return nil
	}

	return ret
}

func (p *Context) Clone(format string, a ...interface{}) *Context {
	return &Context{
		runnerGroupMap: p.runnerGroupMap,
		runnerMap:      p.runnerMap,
		job:            p.job,
		parent:         p.parent,
		rawCmd:         p.rawCmd,
		runCmd:         p.runCmd,
		runners:        p.runners,
		path:           fmt.Sprintf(format, a...),
		file:           p.file,
	}
}

func (p *Context) Run() bool {
	if len(p.runners) == 0 {
		// Check
		p.Clone("kernel error: runners must be checked in previous call")
		return false
	} else if len(p.runners) == 1 {
		// If len(p.runners) == 1. Run it
		switch p.runCmd.Tag {
		case "job":
			return p.runJob()
		case "cmd":
			return p.runCommand()
		case "script":
			return p.runScript()
		default:
			p.Clone("kernel error: type must be checked in previous call")
			return false
		}
	} else {
		// If len(p.runners) > 1, Split the context by runners.
		for _, runner := range p.runners {
			ctx := p.Clone(p.path)
			ctx.runners = []Runner{runner}
			if !ctx.Run() {
				return false
			}
		}

		return true
	}
}

func (p *Context) runJob() bool {
	// If the commands are run in sequence, run them one by one and return
	if !p.job.Async {
		for i := 0; i < len(p.job.Commands); i++ {
			ctx := p.Clone("%s.commands[%d]", p.runCmd.Exec, i).
				subContext(p.job.Commands[i])

			if ctx == nil {
				return false
			}

			if !ctx.Run() {
				return false
			}
		}

		return true
	}

	// The commands are run async
	waitCH := make(chan bool, len(p.job.Commands))

	for i := 0; i < len(p.job.Commands); i++ {
		go func(idx int) {
			ctx := p.Clone("jobs.%s.commands[%d]", p.runCmd.Exec, idx).
				subContext(p.job.Commands[idx])

			if ctx == nil {
				waitCH <- false
			} else {
				waitCH <- ctx.Run()
			}
		}(i)
	}

	// Wait for all commands to complete
	ret := true
	for i := 0; i < len(p.job.Commands); i++ {
		if !<-waitCH {
			ret = false
		}
	}

	return ret
}

func (p *Context) runScript() bool {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	vm := otto.New()
	_ = vm.Set("dbot", &DbotObject{
		vm:     vm,
		stdout: stdout,
		stderr: stderr,
		ctx:    p,
		seed:   0,
	})
	_, e := vm.Run(p.runCmd.Exec)

	p.Log(stdout.String(), stderr.String())

	if e != nil {
		p.LogError(e.Error())
	}

	return e == nil
}

func (p *Context) runCommand() bool {
	return p.runners[0].Run(p)
}

func (p *Context) getRunnersName() string {
	nameArray := make([]string, 0)

	for _, runner := range p.runners {
		nameArray = append(nameArray, runner.Name())
	}

	return strings.Join(nameArray, ",")
}

func (p *Context) loadConfig(path string, v interface{}) (string, bool) {
	var fnUnmarshal (func(data []byte, v interface{}) error)

	ret := ""
	if filepath.IsAbs(path) {
		ret = path
	} else if p.file == "" {
		v, e := filepath.Abs(path)
		if e != nil {
			p.LogError(e.Error())
		}
		ret = v
	} else {
		ret = filepath.Join(filepath.Dir(p.file), path)
	}

	// If config file is a directory, we try to find default config file
	if IsDir(ret) {
		yamlFile := filepath.Join(ret, "main.yaml")
		ymlFile := filepath.Join(ret, "main.yml")
		jsonFile := filepath.Join(ret, "main.json")

		if IsFile(yamlFile) {
			ret = yamlFile
		} else if IsFile(ymlFile) {
			ret = ymlFile
		} else if IsFile(jsonFile) {
			ret = jsonFile
		} else {
			p.LogError(
				"could not find main.yaml or main.yml or main.json "+
					"in directory \"%s\"\n",
				ret,
			)
			return "", false
		}
	}

	// Check the file extension, and set corresponding unmarshal func
	ext := filepath.Ext(ret)
	switch ext {
	case ".json":
		fnUnmarshal = json.Unmarshal
	case ".yml":
		fnUnmarshal = yaml.Unmarshal
	case ".yaml":
		fnUnmarshal = yaml.Unmarshal
	default:
		p.LogError("unsupported file extension \"%s\"", ret)
		return "", false
	}

	// Read the config file, and unmarshal it to config structure
	if b, e := ioutil.ReadFile(ret); e != nil {
		p.LogError(e.Error())
		return "", false
	} else if e := fnUnmarshal(b, v); e != nil {
		p.LogError(e.Error())
		return "", false
	} else {
		return ret, true
	}
}

func (p *Context) GetUserInput(desc string, mode string) (string, bool) {
	switch mode {
	case "password":
		p.LogInfo("")
		p.logRawInfo(desc)
		b, e := term.ReadPassword(int(syscall.Stdin))
		if e != nil {
			p.logRawError(e.Error() + "\n")
			return "", false
		}

		p.logRawInfo("\n")
		return string(b), true
	case "text":
		p.LogInfo("")
		p.logRawInfo(desc)
		ret := ""
		if _, e := fmt.Scanf("%s", &ret); e != nil {
			p.logRawError(e.Error() + "\n")
			return "", false
		}
		return ret, true
	default:
		p.LogError("unsupported mode %s", mode)
		return "", false
	}
}

func (p *Context) logRawInfo(format string, a ...interface{}) {
	log(fmt.Sprintf(format, a...), color.FgBlue)
}

func (p *Context) logRawError(format string, a ...interface{}) {
	log(fmt.Sprintf(format, a...), color.FgRed)
}

func (p *Context) LogInfo(format string, a ...interface{}) {
	p.Log(fmt.Sprintf(format, a...), "")
}

func (p *Context) LogError(format string, a ...interface{}) {
	p.Log("", fmt.Sprintf(format, a...))
}

func (p *Context) Log(outStr string, errStr string) {
	logItems := []interface{}{}

	logItems = append(logItems, p.getRunnersName())
	logItems = append(logItems, color.FgYellow)

	logItems = append(logItems, " > ")
	logItems = append(logItems, color.FgGreen)
	logItems = append(logItems, p.file)
	logItems = append(logItems, color.FgYellow)

	if p.path != "" {
		logItems = append(logItems, " > ")
		logItems = append(logItems, color.FgGreen)
		logItems = append(logItems, p.path)
		logItems = append(logItems, color.FgYellow)
	}

	logItems = append(logItems, "\n")
	logItems = append(logItems, color.FgGreen)

	if p.runCmd != nil {
		if p.runCmd.Tag != "job" && p.runCmd.Exec != "" {
			logItems = append(logItems, GetStandradOut(p.runCmd.Exec))
			logItems = append(logItems, color.FgBlue)
		}
	}

	if outStr != "" {
		logItems = append(logItems, GetStandradOut(outStr))
		logItems = append(logItems, color.FgGreen)
	}

	if errStr != "" {
		logItems = append(logItems, GetStandradOut(errStr))
		logItems = append(logItems, color.FgRed)
	}

	log(logItems...)
}
