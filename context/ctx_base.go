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
	LogInfo(format string, a ...interface{})
	LogError(format string, a ...interface{})
	Clone(pathFormat string, a ...interface{}) Context
	GetRootEnv() Env
}

type BaseContext struct {
	parent Context
	target string
	path   string
	config string
	exec   string
	env    Env
}

func (p *BaseContext) copy(pathFormat string, a ...interface{}) *BaseContext {
	return &BaseContext{
		parent: p.parent,
		target: p.target,
		path:   fmt.Sprintf(pathFormat, a...),
		config: p.config,
		exec:   p.exec,
		env:    p.env.Merge(Env{}),
	}
}

func (p *BaseContext) GetParent() Context {
	return p.parent
}

func (p *BaseContext) GetRootContext() *RootContext {
	v := p.parent

	for v != nil && v.GetParent() != nil {
		v = v.GetParent()
	}

	return v.(*RootContext)
}

func (p *BaseContext) AbsPath(path string) (string, bool) {
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

func (p *BaseContext) LoadConfig(v interface{}) bool {
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
				"could not found main.yaml or main.yml or main.json "+
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

func (p *BaseContext) GetRootEnv() Env {
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

func (p *BaseContext) GetPath() string {
	return p.path
}

func (p *BaseContext) GetUserInput(desc string, mode string) (string, bool) {
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

func (p *BaseContext) LogRawInfo(format string, a ...interface{}) {
	log(fmt.Sprintf(format, a...), color.FgBlue)
}

func (p *BaseContext) LogRawError(format string, a ...interface{}) {
	log(fmt.Sprintf(format, a...), color.FgRed)
}

func (p *BaseContext) LogInfo(format string, a ...interface{}) {
	p.Log(fmt.Sprintf(format, a...), "")
}

func (p *BaseContext) LogError(format string, a ...interface{}) {
	p.Log("", fmt.Sprintf(format, a...))
}

func (p *BaseContext) Log(outStr string, errStr string) {
	logItems := []interface{}{}

	logItems = append(logItems, p.target)
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

	if p.exec != "" {
		logItems = append(logItems, GetStandradOut(p.exec))
		logItems = append(logItems, color.FgBlue)
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
