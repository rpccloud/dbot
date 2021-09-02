package dbot

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/fatih/color"
	"golang.org/x/term"
)

var outFilter = []string{
	"\033",
}

var errFilter = []string{
	"Output is not to a terminal",
	"Input is not from a terminal",
}

var rootEnv = Env{
	"KeyESC":   "\033",
	"KeyEnter": "\n",
}

var gLogLock sync.Mutex

func log(a ...interface{}) {
	gLogLock.Lock()
	defer gLogLock.Unlock()

	for i := 0; i < len(a); i += 2 {
		if s := a[i].(string); s != "" {
			_, _ = color.New(a[i+1].(color.Attribute), color.Bold).Print(s)
		}

	}
}

type XContext struct {
	parent  *XContext
	task    *Task
	current string
	cmd     *Command
	runner  Runner
	env     Env
}

func NewRootContext(absConfig string) *XContext {
	return &XContext{
		parent:  nil,
		task:    nil,
		current: "init",
		cmd: &Command{
			Type:   "main",
			Exec:   "init",
			On:     "local",
			Inputs: nil,
			Env:    nil,
			Config: absConfig,
		},
		runner: &MainRunner{},
	}
}

func (p *XContext) CreateTaskContext(name string) *XContext {
	if p.cmd.Type != "main" {
		p.LogError(
			"kernel error: CreateTaskContext must be called on RootContext",
		)
		return nil
	}

	return &XContext{
		parent:  p,
		task:    nil,
		current: "task." + name,
		cmd: &Command{
			Type:   "task",
			Exec:   name,
			On:     "local",
			Inputs: nil,
			Env:    nil,
			Config: p.cmd.Config,
		},
		runner: p.runner,
	}
}

func (p *XContext) CreateImportContext(name string, config string) *XContext {
	if p.cmd.Type != "task" {
		p.LogError(
			"kernel error: CreateImportContext must be called on TaskContext",
		)
		return nil
	}

	return &XContext{
		parent:  p,
		task:    p.task,
		current: name,
		cmd: &Command{
			Type:   "import",
			Exec:   name,
			On:     "local",
			Inputs: nil,
			Env:    nil,
			Config: config,
		},
		runner: p.runner,
	}
}

func (p *XContext) Prepare() bool {
	return p.runner.Prepare(p)
}

func (p *XContext) Run() bool {
	return p.runner.Run(p)
}

func (p *XContext) Clone() *XContext {
	return &XContext{
		parent:  p.parent,
		task:    p.task,
		current: p.current,
		cmd:     p.cmd.Clone(),
		runner:  p.runner,
		env:     p.env.merge(nil),
	}
}

func (p *XContext) AbsFilePath(path string) (string, bool) {
	ret, e := filepath.Abs(filepath.Join(
		filepath.Dir(p.cmd.Config),
		path,
	))

	if e != nil {
		p.LogError(e.Error())
		return "", false
	}

	return ret, true
}

func (p *XContext) loadConfig(absPath string, v interface{}) bool {
	if fnUnmarshal := getUnmarshalFn(absPath); fnUnmarshal == nil {
		p.LogErrorf("unsupported file extension \"%s\"", absPath)
		return false
	} else if b, e := ioutil.ReadFile(absPath); e != nil {
		p.LogError(e.Error())
		return false
	} else if e := fnUnmarshal(b, v); e != nil {
		p.LogError(e.Error())
		return false
	} else {
		return true
	}
}

func (p *XContext) LoadMainConfig(absPath string) *MainConfig {
	config := MainConfig{}

	if !p.loadConfig(absPath, &config) {
		return nil
	}

	return &config
}

func (p *XContext) LoadJobConfig(absPath string) *JobConfig {
	config := JobConfig{}

	if !p.loadConfig(absPath, &config) {
		return nil
	}

	return &config
}

func (p *XContext) LoadRemoteConfig(absPath string) map[string][]*Remote {
	config := make(map[string][]*Remote)

	if !p.loadConfig(absPath, &config) {
		return nil
	}

	return config
}

func (p *XContext) RootEnv() Env {
	return rootEnv.merge(rootEnv.merge(Env{
		"ConfigDir": filepath.Dir(p.cmd.Config),
	}))
}

func (p *XContext) GetEnv() Env {
	return p.env
}

func (p *XContext) SetEnv(env Env) *XContext {
	p.env = env
	return p
}

func (p *XContext) GetCurrent() string {
	return p.current
}

func (p *XContext) SetCurrent(current string) *XContext {
	p.current = current
	return p
}

func (p *XContext) SetTask(task *Task) *XContext {
	p.task = task
	return p
}

func (p *XContext) SetCurrentf(format string, a ...interface{}) *XContext {
	return p.SetCurrent(fmt.Sprintf(format, a...))
}

func (p *XContext) GetUserInput(desc string, mode string) (string, bool) {
	var e error = nil
	var ret string = ""

	switch mode {
	case "password":
		p.LogRawInfo(desc)
		if b, err := term.ReadPassword(int(syscall.Stdin)); err != nil {
			e = err
		} else {
			ret = string(b)
		}
		p.LogRawInfo("\n")
	case "text":
		p.LogRawInfo(desc)
		if _, err := fmt.Scanf("%s", &ret); err != nil {
			e = err
		}
	default:
		e = fmt.Errorf("unsupported mode %s", mode)
	}

	if e != nil {
		p.LogRawError(e.Error() + "\n")
	}

	return ret, e == nil
}

func (p *XContext) LogRawInfo(v string) {
	log(v, color.FgBlue)
}

func (p *XContext) LogRawError(v string) {
	log(v, color.FgRed)
}

func (p *XContext) LogInfo(v string) {
	p.Log(v, "")
}

func (p *XContext) LogInfof(format string, a ...interface{}) {
	p.Log(fmt.Sprintf(format, a...), "")
}

func (p *XContext) LogError(v string) {
	p.Log("", v)
}

func (p *XContext) LogErrorf(format string, a ...interface{}) {
	p.Log("", fmt.Sprintf(format, a...))
}

func (p *XContext) Log(outStr string, errStr string) {
	logItems := []interface{}{}

	logItems = append(logItems, p.runner.Name())
	logItems = append(logItems, color.FgYellow)

	logItems = append(logItems, " > ")
	logItems = append(logItems, color.FgCyan)

	logItems = append(logItems, p.cmd.Config)
	logItems = append(logItems, color.FgYellow)

	if p.current != "" {
		logItems = append(logItems, " > ")
		logItems = append(logItems, color.FgCyan)
		logItems = append(logItems, p.current)
		logItems = append(logItems, color.FgYellow)
	}

	logItems = append(logItems, "\n")
	logItems = append(logItems, color.FgGreen)

	if p.cmd.Type == "cmd" && p.cmd.Exec != "" {
		logItems = append(logItems, getStandradOut(p.cmd.Exec))
		logItems = append(logItems, color.FgBlue)
	}

	if outStr != "" {
		logItems = append(logItems, getStandradOut(outStr))
		logItems = append(logItems, color.FgGreen)
	}

	if errStr != "" {
		logItems = append(logItems, getStandradOut(errStr))
		logItems = append(logItems, color.FgRed)
	}

	log(logItems...)
}
