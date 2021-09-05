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
	parent         *Context
	rawCmd         *Command
	runCmd         *Command
	runners        []Runner
	config         Config
	path           string
	file           string
	upEnv          Env
}

func NewContext(file string, jobName string) *Context {
	vCtx := &Context{
		runnerMap: make(map[string]Runner),
		path:      "dbot.init",
		file:      "",
		runners:   []Runner{&LocalRunner{}},
	}

	if currentDir, e := os.Getwd(); e == nil {
		vCtx.file = currentDir
	} else {
		vCtx.LogError(e.Error())
		return nil
	}

	absFile, ok := vCtx.AbsPath(file)
	if !ok {
		return nil
	}

	ret := vCtx.subContext(
		&Command{Tag: "job", Exec: jobName, File: absFile},
		Env{},
	)

	ret.parent = nil
	ret.runnerGroupMap["local"] = []string{"local"}
	ret.runnerMap["local"] = &LocalRunner{}

	return ret
}

func (p *Context) subContext(rawCmd *Command, upEnv Env) *Context {
	runCmd := (&Context{rawCmd: rawCmd, upEnv: upEnv}).ParseCommand()
	file := p.file
	runners := p.runners
	path := p.path
	config := Config{}

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

		config = p.config
	case "job":
		if len(rawCmd.Stdin) > 0 {
			p.Clone("%s.stdin", p.path).LogError(
				"unsupported stdin on tag \"%s\"",
				runCmd.Tag,
			)
		}

		// Set file and load config
		if runCmd.File == "" {
			config = p.config
		} else if absFile, ok := p.AbsPath(runCmd.File); !ok {
			return nil
		} else if retFile, ok := p.Clone("%s.file", p.path).
			LoadConfig(absFile, &config); !ok {
			return nil
		} else {
			file = retFile
			path = ""
		}

		// Check is the job exist
		if _, ok := config[runCmd.Exec]; !ok {
			p.Clone("%s.exec", p.path).
				LogError("could not find job \"%s\"", runCmd.Exec)
			return nil
		}
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

	return &Context{
		runnerGroupMap: make(map[string][]string),
		runnerMap:      p.runnerMap,
		config:         config,
		parent:         p,
		rawCmd:         rawCmd,
		runCmd:         runCmd,
		runners:        runners,
		path:           path,
		file:           file,
		upEnv:          upEnv,
	}
}

func (p *Context) Clone(format string, a ...interface{}) *Context {
	return &Context{
		runnerGroupMap: p.runnerGroupMap,
		runnerMap:      p.runnerMap,
		config:         p.config,
		parent:         p.parent,
		rawCmd:         p.rawCmd,
		runCmd:         p.runCmd,
		runners:        p.runners,
		path:           fmt.Sprintf(format, a...),
		file:           p.file,
		upEnv:          p.upEnv,
	}
}

func (p *Context) getRunnersNameByRunnerGroup(groupName string) []string {
	if sshGroup, ok := p.runnerGroupMap[groupName]; ok {
		return append([]string{}, sshGroup...)
	} else if p.parent != nil {
		return p.parent.getRunnersNameByRunnerGroup(groupName)
	} else {
		return []string{}
	}
}

func (p *Context) getRunners(runOn string) []Runner {
	ret := make([]Runner, 0)

	for _, groupName := range strings.Split(runOn, ",") {
		if groupName = strings.TrimSpace(groupName); groupName != "" {
			runnersName := p.getRunnersNameByRunnerGroup(groupName)

			if len(runnersName) == 0 {
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

func (p *Context) getRootEnv() Env {
	return Env{
		"KeyESC":   "\033",
		"KeyEnter": "\n",
	}.Merge(Env{
		"ConfigDir": filepath.Dir(p.file),
	})
}

func (p *Context) runJob() bool {
	// Get job
	job, ok := p.config[p.runCmd.Exec]
	if !ok {
		p.Clone("jobs.%s", p.runCmd.Exec).
			LogError("could not find job \"%s\"", p.runCmd.Exec)
		return false
	}

	// Make jobEnv
	rootEnv := p.getRootEnv()
	jobEnv := rootEnv.Merge(rootEnv.ParseEnv(job.Env)).Merge(p.runCmd.Env)

	// If the commands are run in sequence, run them one by one and return
	if !job.Async {
		for i := 0; i < len(job.Commands); i++ {
			ctx := p.Clone("%s.commands[%d]", p.runCmd.Exec, i).
				subContext(job.Commands[i], jobEnv)

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
	waitCH := make(chan bool, len(job.Commands))

	for i := 0; i < len(job.Commands); i++ {
		go func(idx int) {
			ctx := p.Clone("jobs.%s.commands[%d]", p.runCmd.Exec, idx).
				subContext(job.Commands[idx], jobEnv)
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

func (p *Context) AbsPath(path string) (string, bool) {
	dir := p.file
	if IsFile(p.file) {
		dir = filepath.Dir(p.file)
	}

	if filepath.IsAbs(path) {
		return path, true
	} else if ret, e := filepath.Abs(
		filepath.Join(dir, path),
	); e != nil {
		p.LogError(e.Error())
		return "", false
	} else {
		return ret, true
	}
}

func (p *Context) LoadConfig(absPath string, v interface{}) (string, bool) {
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

func (p *Context) ParseCommand() *Command {
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
