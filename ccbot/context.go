package ccbot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"golang.org/x/term"
	"gopkg.in/yaml.v2"
)

type Context struct {
	runnerGroupMap map[string][]string
	runnerMap      map[string]Runner
	jobConfig      *Job
	jobEnv         Env
	parent         *Context
	rawCmd         *Command
	runCmd         *Command
	runners        []Runner
	path           string
	file           string
	upEnv          Env
}

func NewContext(file string, jobName string) *Context {
	vCtx := &Context{
		runnerGroupMap: map[string][]string{
			"local": {"local"},
		},
		runnerMap: map[string]Runner{
			"local": &LocalRunner{},
		},
		path:    "dbot.init",
		file:    "",
		runners: []Runner{&LocalRunner{}},
	}

	if currentDir, e := os.Getwd(); e == nil {
		vCtx.file = currentDir
	} else {
		vCtx.LogError(e.Error())
		return nil
	}

	absFile, ok := vCtx.absPath(file)
	if !ok {
		return nil
	}

	ret := vCtx.subContext(
		&Command{Tag: "job", Exec: jobName, File: absFile},
		Env{},
	)

	ret.parent = nil

	return ret
}

func (p *Context) init() bool {
	// init jobEnv
	rootEnv := p.getRootEnv()
	useEnv := rootEnv.
		Merge(rootEnv.ParseEnv(p.jobConfig.Env)).
		Merge(p.runCmd.Args)
	jobEnv := useEnv.Merge(Env{})
	for key, it := range p.jobConfig.Inputs {
		itDesc := useEnv.ParseString(it.Desc, "input "+key+": ", false)
		itType := useEnv.ParseString(it.Type, "text", true)
		value, ok := p.Clone("%s.inputs.%s", p.path, key).
			GetUserInput(itDesc, itType)
		if !ok {
			return false
		}
		jobEnv[key] = useEnv.ParseString(value, "", false)
	}
	p.jobEnv = jobEnv

	// Load imports
	for key, it := range p.jobConfig.Imports {
		itName := jobEnv.ParseString(it.Name, "", true)
		itFile := jobEnv.ParseString(it.File, "", true)
		importConfig := make(map[string][]*Remote)

		if absFile, ok := p.Clone("%s.imports.%s", p.path, key).
			absPath(itFile); !ok {
			return false
		} else if _, ok := p.Clone("%s.imports.%s", p.path, key).
			loadConfig(absFile, importConfig); !ok {
			return false
		} else if importItem, ok := importConfig[itName]; !ok {
			return false
		} else {
			ctx := &Context{
				runnerGroupMap: p.runnerGroupMap,
				runnerMap:      p.runnerMap,
				path:           itName,
				file:           absFile,
				runners:        p.runners,
			}

			sshGroup := ctx.loadSSHGroup(importItem)
			if sshGroup == nil {
				return false
			}
			p.runnerGroupMap[key] = sshGroup
		}
	}

	// Load remotes
	for key, list := range p.jobConfig.Remotes {
		sshGroup := p.Clone("%s.remotes.%s", p.path, key).loadSSHGroup(list)
		if sshGroup == nil {
			return false
		}
		p.runnerGroupMap[key] = sshGroup
	}

	return true
}

func (p *Context) loadSSHGroup(list []*Remote) []string {
	if len(list) == 0 {
		p.LogError("list is empty")
		return nil
	}

	ret := make([]string, 0)
	for idx, it := range list {
		host := Env{}.ParseString(it.Host, "", true)
		user := Env{}.ParseString(it.User, os.Getenv("USER"), true)
		port := Env{}.ParseString(it.Port, "22", true)

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

func (p *Context) subContext(rawCmd *Command, upEnv Env) *Context {
	runCmd := (&Context{rawCmd: rawCmd, upEnv: upEnv}).parseCommand()
	file := p.file
	runners := p.runners
	path := p.path
	// config := make(map[string]*Job)
	jobConfig := p.jobConfig
	needInitJob := false

	switch runCmd.Tag {
	case "cmd", "script":
		if len(rawCmd.Args) > 0 {
			p.Clone("%s.args", p.path).LogError(
				"unsupported args on tag \"%s\"",
				runCmd.Tag,
			)
		}

		if rawCmd.File != "" {
			p.Clone("%s.file", p.path).LogError(
				"unsupported file on tag \"%s\"",
				runCmd.Tag,
			)
		}
	case "job":
		if len(rawCmd.Stdin) > 0 {
			p.Clone("%s.stdin", p.path).LogError(
				"unsupported stdin on tag \"%s\"",
				runCmd.Tag,
			)
		}

		if runCmd.File != "" {
			if v, ok := p.absPath(runCmd.File); ok {
				file = v
			} else {
				return nil
			}
		}

		// Load config
		config := make(map[string]*Job)
		if configFile, ok := p.Clone("%s.file", p.path).
			loadConfig(file, &config); ok {
			file = configFile
		} else {
			return nil
		}

		// Check is the job exist
		if v, ok := config[runCmd.Exec]; ok {
			jobConfig = v
			needInitJob = true
		} else {
			p.Clone("%s.exec", p.path).
				LogError("could not find job \"%s\"", runCmd.Exec)
			return nil
		}

		path = runCmd.Exec
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
		jobConfig:      jobConfig,
		jobEnv:         p.jobEnv,
		parent:         p,
		rawCmd:         rawCmd,
		runCmd:         runCmd,
		runners:        runners,
		path:           path,
		file:           file,
		upEnv:          upEnv,
	}

	if needInitJob {
		if !ret.init() {
			return nil
		}
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
		jobConfig:      p.jobConfig,
		jobEnv:         p.jobEnv,
		parent:         p.parent,
		rawCmd:         p.rawCmd,
		runCmd:         p.runCmd,
		runners:        p.runners,
		path:           fmt.Sprintf(format, a...),
		file:           p.file,
		upEnv:          p.upEnv,
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
	if !p.jobConfig.Async {
		for i := 0; i < len(p.jobConfig.Commands); i++ {
			ctx := p.Clone("%s.commands[%d]", p.runCmd.Exec, i).
				subContext(p.jobConfig.Commands[i], p.jobEnv)

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
	waitCH := make(chan bool, len(p.jobConfig.Commands))

	for i := 0; i < len(p.jobConfig.Commands); i++ {
		go func(idx int) {
			ctx := p.Clone("jobs.%s.commands[%d]", p.runCmd.Exec, idx).
				subContext(p.jobConfig.Commands[idx], p.jobEnv)
			waitCH <- ctx.Run()
		}(i)
	}

	// Wait for all commands to complete
	ret := true
	for i := 0; i < len(p.jobConfig.Commands); i++ {
		if !<-waitCH {
			ret = false
		}
	}

	return ret
}

func (p *Context) runScript() bool {
	return false
}

func (p *Context) runCommand() bool {
	return p.runners[0].Run(p)
}

func (p *Context) getRunnersName() string {
	nameArray := make([]string, 0)
	for _, runner := range p.runners {
		nameArray = append(nameArray, runner.Name())
	}

	if len(nameArray) == 0 {
		return ""
	}

	return strings.Join(nameArray, ",")
}

func (p *Context) absPath(path string) (string, bool) {
	dir := p.file
	if IsFile(p.file) {
		dir = filepath.Dir(p.file)
	}

	if filepath.IsAbs(path) {
		return path, true
	} else if ret, e := filepath.Abs(filepath.Join(dir, path)); e != nil {
		p.LogError(e.Error())
		return "", false
	} else {
		return ret, true
	}
}

func (p *Context) loadConfig(absPath string, v interface{}) (string, bool) {
	var fnUnmarshal (func(data []byte, v interface{}) error)
	ret := absPath

	// If config file is a directory, we try to find default config file
	if IsDir(absPath) {
		yamlFile := filepath.Join(absPath, "main.yaml")
		ymlFile := filepath.Join(absPath, "main.yml")
		jsonFile := filepath.Join(absPath, "main.json")

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
				absPath,
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

func (p *Context) parseCommand() *Command {
	cmdEnv := p.upEnv.ParseEnv(p.rawCmd.Env)
	useEnv := p.upEnv.Merge(cmdEnv)

	return &Command{
		Tag:   p.upEnv.ParseString(p.rawCmd.Tag, "cmd", true),
		Exec:  useEnv.ParseString(p.rawCmd.Exec, "", false),
		On:    useEnv.ParseString(p.rawCmd.On, "", true),
		Stdin: useEnv.ParseStringArray(p.rawCmd.Stdin),
		Env:   cmdEnv,
		Args:  useEnv.ParseEnv(p.rawCmd.Args),
		File:  useEnv.ParseString(p.rawCmd.File, "", true),
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
		if p.runCmd.Tag == "cmd" && p.runCmd.Exec != "" {
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
