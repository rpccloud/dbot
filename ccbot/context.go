package context

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"golang.org/x/term"
	"gopkg.in/yaml.v2"
)

type CmdContext struct {
	runnerGroupMap map[string][]string
	runnerMap      map[string]Runner

	parent   *CmdContext
	cmd      *Command
	runners  []Runner
	path     string
	config   string
	parseEnv Env
}

func (p *CmdContext) Clone(format string, a ...interface{}) *CmdContext {
	return &CmdContext{
		parent:   p.parent,
		cmd:      p.cmd,
		runners:  append([]Runner{}, p.runners...),
		path:     fmt.Sprintf(format, a...),
		config:   p.config,
		parseEnv: p.parseEnv.Merge(Env{}),
	}
}

func (p *CmdContext) getRunnersNameByRunnerGroup(groupName string) []string {
	if sshGroup, ok := p.runnerGroupMap[groupName]; ok {
		return append([]string{}, sshGroup...)
	} else if p.parent != nil {
		return p.parent.getRunnersNameByRunnerGroup(groupName)
	} else {
		return []string{}
	}
}

func (p *CmdContext) getRunners(runOn string) []Runner {
	ret := make([]Runner, 0)

	for _, groupName := range strings.Split(runOn, ",") {
		if groupName = strings.TrimSpace(groupName); groupName != "" {
			runnersName := p.getRunnersNameByRunnerGroup(groupName)

			if len(runnersName) == 0 {
				p.LogError("could not find SSHGroup \"%s\"", groupName)
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

func (p *CmdContext) newJobCommandContext(cmd *Command, parseEnv Env) *CmdContext {
	ret := &CmdContext{
		runnerGroupMap: make(map[string][]string),
		runnerMap:      p.runnerMap,
		parent:         p,
		cmd:            cmd,
		runners:        append([]Runner{}, p.runners...),
		path:           p.path,
		config:         p.config,
		parseEnv:       parseEnv,
	}

	// Parse Command
	parsedCmd := ret.ParseCommand()
	if parsedCmd == nil {
		return nil
	}

	// Redirect config
	if parsedCmd.Type == "job" && parsedCmd.Config != "" {
		config, ok := p.AbsPath(parsedCmd.Config)
		if !ok {
			return nil
		}
		ret.config = config
		ret.path = ""
	}

	// Redirect runners
	if parsedCmd.On != "" {
		runners := p.getRunners(parsedCmd.On)
		if runners == nil {
			return nil
		}
		ret.runners = runners
	}

	return ret
}

func (p *CmdContext) Run() bool {
	// Parse command
	cmd := p.ParseCommand()
	if cmd == nil {
		return false
	}

	if len(p.runners) == 0 {
		// Check
		p.Clone("kernel error: runners must be checked in previous call")
		return false
	} else if len(p.runners) == 1 {
		// If len(p.runners) == 1. Run it
		switch cmd.Type {
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

func (p *CmdContext) runJob() bool {
	// Parse command
	cmd := p.ParseCommand()
	if cmd == nil {
		return false
	}

	// Load config
	config := &Config{}
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
			ctx := p.Clone("jobs.%s.commands[%d]", cmd.Exec, i).
				newJobCommandContext(job.Commands[i], jobEnv)
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
			ctx := p.Clone("jobs.%s.commands[%d]", cmd.Exec, idx).
				newJobCommandContext(job.Commands[idx], jobEnv)
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

func (p *CmdContext) getRunnersName() string {
	nameArray := make([]string, 0)
	for _, runner := range p.runners {
		nameArray = append(nameArray, runner.Name())
	}

	if len(nameArray) == 0 {
		return ""
	}

	return strings.Join(nameArray, ",")
}

func (p *CmdContext) AbsPath(path string) (string, bool) {
	if absPath, e := filepath.Abs(path); e != nil {
		p.LogError(e.Error())
		return "", false
	} else if absPath == path {
		return path, true
	} else if ret, e := filepath.Abs(
		filepath.Join(filepath.Dir(p.config), path),
	); e != nil {
		p.LogError(e.Error())
		return "", false
	} else {
		return ret, true
	}
}

func (p *CmdContext) LoadConfig(v interface{}) bool {
	var fnUnmarshal (func(data []byte, v interface{}) error)

	// If config is a directory, we try to find default config file
	if IsDir(p.config) {
		yamlFile := filepath.Join(p.config, "main.yaml")
		ymlFile := filepath.Join(p.config, "main.yml")
		jsonFile := filepath.Join(p.config, "main.json")

		if IsFile(yamlFile) {
			p.config = yamlFile
		} else if IsFile(ymlFile) {
			p.config = ymlFile
		} else if IsFile(jsonFile) {
			p.config = jsonFile
		} else {
			p.LogError(
				"could not find main.yaml or main.yml or main.json "+
					"in directory \"%s\"\n",
				p.config,
			)
			return false
		}
	}

	// Check the file extension, and set corresponding unmarshal func
	ext := filepath.Ext(p.config)
	switch ext {
	case ".json":
		fnUnmarshal = json.Unmarshal
	case ".yml":
		fnUnmarshal = yaml.Unmarshal
	case ".yaml":
		fnUnmarshal = yaml.Unmarshal
	default:
		p.LogError("unsupported file extension \"%s\"", p.config)
		return false
	}

	// Read the config file, and unmarshal it to config structure
	if b, e := ioutil.ReadFile(p.config); e != nil {
		p.LogError(e.Error())
		return false
	} else if e := fnUnmarshal(b, v); e != nil {
		p.LogError(e.Error())
		return false
	} else {
		return true
	}
}

func (p *CmdContext) GetRootEnv() Env {
	return Env{
		"KeyESC":   "\033",
		"KeyEnter": "\n",
	}.Merge(Env{
		"ConfigDir": filepath.Dir(p.config),
	})
}

func (p *CmdContext) ParseCommand() *Command {
	if p.cmd == nil {
		p.LogError("kernel error: cmd is nil")
		return nil
	}

	cmdEnv := p.parseEnv.ParseEnv(p.cmd.Env)
	useEnv := p.parseEnv.Merge(cmdEnv)

	return &Command{
		Type:   p.parseEnv.ParseString(p.cmd.Type, "cmd", true),
		Exec:   useEnv.ParseString(p.cmd.Exec, "", false),
		On:     useEnv.ParseString(p.cmd.On, "", true),
		Inputs: useEnv.ParseStringArray(p.cmd.Inputs),
		Env:    cmdEnv,
		Config: useEnv.ParseString(p.cmd.Config, "", true),
	}
}

func (p *CmdContext) GetPath() string {
	return p.path
}

func (p *CmdContext) GetUserInput(desc string, mode string) (string, bool) {
	switch mode {
	case "password":
		p.LogInfo("")
		p.LogRawInfo(desc)
		b, e := term.ReadPassword(int(syscall.Stdin))
		if e != nil {
			p.LogRawError(e.Error() + "\n")
			return "", false
		}

		p.LogRawInfo("\n")
		return string(b), true
	case "text":
		p.LogInfo("")
		p.LogRawInfo(desc)
		ret := ""
		if _, e := fmt.Scanf("%s", &ret); e != nil {
			p.LogRawError(e.Error() + "\n")
			return "", false
		}
		return ret, true
	default:
		p.LogError("unsupported mode %s", mode)
		return "", false
	}
}

func (p *CmdContext) LogRawInfo(format string, a ...interface{}) {
	log(fmt.Sprintf(format, a...), color.FgBlue)
}

func (p *CmdContext) LogRawError(format string, a ...interface{}) {
	log(fmt.Sprintf(format, a...), color.FgRed)
}

func (p *CmdContext) LogInfo(format string, a ...interface{}) {
	p.Log(fmt.Sprintf(format, a...), "")
}

func (p *CmdContext) LogError(format string, a ...interface{}) {
	p.Log("", fmt.Sprintf(format, a...))
}

func (p *CmdContext) Log(outStr string, errStr string) {

	logItems := []interface{}{}

	logItems = append(logItems, p.getRunnersName())
	logItems = append(logItems, color.FgYellow)

	logItems = append(logItems, " > ")
	logItems = append(logItems, color.FgGreen)
	logItems = append(logItems, p.config)
	logItems = append(logItems, color.FgYellow)

	if p.path != "" {
		logItems = append(logItems, " > ")
		logItems = append(logItems, color.FgGreen)
		logItems = append(logItems, p.path)
		logItems = append(logItems, color.FgYellow)
	}

	logItems = append(logItems, "\n")
	logItems = append(logItems, color.FgGreen)

	if p.cmd != nil {
		parsedCmd := p.ParseCommand()
		if parsedCmd.Type == "cmd" && parsedCmd.Exec != "" {
			logItems = append(logItems, GetStandradOut(parsedCmd.Exec))
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
