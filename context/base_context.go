package context

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"syscall"

	"github.com/fatih/color"
	"golang.org/x/term"
	"gopkg.in/yaml.v2"
)

type Context interface {
	Run() bool
	GetPath() string
	GetParent() Context
	GetRootContext() *RootContext
	GetUserInput(desc string, mode string) (string, bool)
	LogInfof(format string, a ...interface{})
	LogErrorf(format string, a ...interface{})
	Clone(pathFormat string, a ...interface{}) Context
	RootEnv() Env
}

type BaseContext struct {
	parent Context
	target string
	path   string
	config string
	exec   string
	env    Env
}

func (p *BaseContext) GetParent() Context {
	return p.parent
}

func (p *BaseContext) GetRootContext() *RootContext {
	v := p.parent

	for v != nil && v.GetParent() != nil {
		v = v.GetParent()
	}

	if v != nil {
		if ret, ok := v.(*RootContext); ok {
			return ret
		}
	}

	return nil
}

func (p *BaseContext) copy(pathFormat string, a ...interface{}) *BaseContext {
	return &BaseContext{
		parent: p.parent,
		target: p.target,
		path:   fmt.Sprintf(pathFormat, a...),
		config: p.config,
		exec:   p.exec,
		env:    p.env.Merge(nil),
	}
}

func (p *BaseContext) AbsPath(path string) (string, bool) {
	if absPath, e := filepath.Abs(path); e != nil {
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

func (p *BaseContext) LoadConfig(config string, v interface{}) bool {
	var fnUnmarshal (func(data []byte, v interface{}) error)

	// Get abs path of config
	absPath, ok := p.AbsPath(config)
	if !ok {
		return false
	}

	// If config is a directory, we try to find default config file
	if IsDir(absPath) {
		yamlFile := filepath.Join(absPath, "main.yaml")
		ymlFile := filepath.Join(absPath, "main.yml")
		jsonFile := filepath.Join(absPath, "main.json")

		if IsFile(yamlFile) {
			absPath = yamlFile
		} else if IsFile(ymlFile) {
			absPath = ymlFile
		} else if IsFile(jsonFile) {
			absPath = jsonFile
		} else {
			p.LogErrorf(
				"could not found main.yaml or main.yml or main.json "+
					"in directory \"%s\"\n",
				absPath,
			)
			return false
		}
	}

	// Check the file extension, and set corresponding unmarshal func
	ext := filepath.Ext(absPath)
	switch ext {
	case ".json":
		fnUnmarshal = json.Unmarshal
	case ".yml":
		fnUnmarshal = yaml.Unmarshal
	case ".yaml":
		fnUnmarshal = yaml.Unmarshal
	default:
		p.LogErrorf("unsupported file extension \"%s\"", absPath)
		return false
	}

	// Read the config file, and unmarshal it to config structure
	if b, e := ioutil.ReadFile(absPath); e != nil {
		p.LogError(e.Error())
		return false
	} else if e := fnUnmarshal(b, v); e != nil {
		p.LogError(e.Error())
		return false
	} else {
		return true
	}
}

func (p *BaseContext) RootEnv() Env {
	return Env{
		"KeyESC":   "\033",
		"KeyEnter": "\n",
	}.Merge(Env{
		"ConfigDir": filepath.Dir(p.config),
	})
}

func (p *BaseContext) GetEnv() Env {
	return p.env.Merge(nil)
}

func (p *BaseContext) SetEnv(env Env) {
	p.env = env.Merge(nil)
}

func (p *BaseContext) GetPath() string {
	return p.path
}

func (p *BaseContext) GetUserInput(desc string, mode string) (string, bool) {
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

func (p *BaseContext) LogRawInfo(v string) {
	log(v, color.FgBlue)
}

func (p *BaseContext) LogRawError(v string) {
	log(v, color.FgRed)
}

func (p *BaseContext) LogInfo(v string) {
	p.Log(v, "")
}

func (p *BaseContext) LogInfof(format string, a ...interface{}) {
	p.Log(fmt.Sprintf(format, a...), "")
}

func (p *BaseContext) LogError(v string) {
	p.Log("", v)
}

func (p *BaseContext) LogErrorf(format string, a ...interface{}) {
	p.Log("", fmt.Sprintf(format, a...))
}

func (p *BaseContext) Log(outStr string, errStr string) {
	logItems := []interface{}{}

	logItems = append(logItems, p.target)
	logItems = append(logItems, color.FgYellow)

	logItems = append(logItems, " > ")
	logItems = append(logItems, color.FgCyan)

	logItems = append(logItems, p.config)
	logItems = append(logItems, color.FgYellow)

	if p.path != "" {
		logItems = append(logItems, " > ")
		logItems = append(logItems, color.FgCyan)
		logItems = append(logItems, p.path)
		logItems = append(logItems, color.FgYellow)
	}

	logItems = append(logItems, "\n")
	logItems = append(logItems, color.FgGreen)

	if p.exec != "" {
		logItems = append(logItems, getStandradOut(p.exec))
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
