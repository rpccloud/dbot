package ccbot

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/robertkrimen/otto"
)

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

type Env map[string]string

func (p Env) ParseString(v string, defaultStr string, trimSpace bool) string {
	replaceArray := make([]string, 0)
	for key, value := range p {
		replaceArray = append(replaceArray, "${"+key+"}", value)
	}

	ret := strings.NewReplacer(replaceArray...).Replace(v)

	if trimSpace {
		ret = strings.TrimSpace(ret)
	}

	if ret == "" {
		ret = defaultStr
	}

	return ret
}

func (p Env) ParseStringArray(arr []string) []string {
	ret := make([]string, len(arr))

	for i := 0; i < len(arr); i++ {
		ret[i] = p.ParseString(arr[i], "", false)
	}

	return ret
}

func (p Env) ParseEnv(env Env) Env {
	ret := make(Env)

	for key, value := range env {
		ret[key] = p.ParseString(value, "", false)
	}

	return ret
}

func (p Env) Merge(env Env) Env {
	ret := make(Env)

	for key, value := range p {
		ret[key] = value
	}

	for key, value := range env {
		ret[key] = value
	}

	return ret
}

type Import struct {
	Name string
	File string
	Env  Env
}

type Input struct {
	Type string
	Desc string
}

type Remote struct {
	Port string
	User string
	Host string
}

type Job struct {
	Async    bool
	Imports  map[string]*Import
	Remotes  map[string][]*Remote
	Inputs   map[string]*Input
	Env      Env
	Commands []*Command
}

type Command struct {
	Tag   string
	Exec  string
	On    string
	Stdin []string
	Env   Env
	Args  Env
	File  string
}

func GetStandradOut(s string) string {
	if s != "" && s[len(s)-1] != '\n' {
		return s + "\n"
	} else {
		return s
	}
}

func IsDir(path string) bool {
	f, e := os.Stat(path)
	return e == nil && f.Mode().IsDir()
}

func IsFile(path string) bool {
	f, e := os.Stat(path)
	return e == nil && f.Mode().IsRegular()
}

func SplitCommand(str string) []string {
	command := " " + str + " "
	ret := make([]string, 0)
	isSingleQuote := false
	isDoubleQuotes := false
	preChar := uint8(0)
	cmdStart := -1

	for i := 0; i < len(command); i++ {
		if isSingleQuote {
			if command[i] == 0x27 {
				isSingleQuote = false
			}
			preChar = command[i]
			continue
		}

		if isDoubleQuotes {
			if command[i] == 0x22 && preChar != 0x5C {
				isDoubleQuotes = false
			}
			preChar = command[i]
			continue
		}

		if command[i] == ' ' {
			if cmdStart >= 0 {
				ret = append(ret, command[cmdStart:i])
				cmdStart = -1
			}
			preChar = command[i]
			continue
		}

		if cmdStart < 0 {
			cmdStart = i
		}

		if command[i] == 0x27 {
			isSingleQuote = true
		}

		if command[i] == 0x22 {
			isDoubleQuotes = true
		}

		preChar = command[i]
	}

	return ret
}

func parseCommandFromObject(object *otto.Object) (*Command, error) {
	ret := &Command{}
	keys := object.Keys()
	for _, key := range keys {
		value, e := object.Get(key)
		if e != nil {
			return ret, fmt.Errorf(
				"get object.%s error: %s", key, e.Error(),
			)
		}

		switch key {
		case "tag":
			if !value.IsString() {
				return ret, fmt.Errorf("object.%s must be string", key)
			}
			ret.Tag = value.String()
		case "exec":
			if !value.IsString() {
				return ret, fmt.Errorf("object.%s must be string", key)
			}
			ret.Exec = value.String()
		case "on":
			if !value.IsString() {
				return ret, fmt.Errorf("object.%s must be string", key)
			}
			ret.On = value.String()
		case "env":
			env := Env{}
			if !value.IsObject() {
				return ret, fmt.Errorf("object.%s must be object", key)
			}
			for _, key := range value.Object().Keys() {
				if value.Object() == nil {
					return ret, fmt.Errorf("object.env.%s is nil", key)
				}
				item, e := value.Object().Get(key)
				if e != nil {
					return ret, fmt.Errorf(
						"object.env.%s error: %s", key, e.Error(),
					)
				}
				if !item.IsString() {
					return ret, fmt.Errorf("object.env.%s must be string", key)
				}
				env[key] = item.String()
			}
			ret.Env = env
		case "stdin":
			stdin := []string{}
			if !value.IsObject() {
				return ret, fmt.Errorf("object.%s must be object", key)
			}
			startIndex := int64(0)
			for _, key := range value.Object().Keys() {
				if strconv.FormatInt(startIndex, 10) != key {
					return ret, fmt.Errorf("object.stdin must be array")
				}

				if value.Object() == nil {
					return ret, fmt.Errorf("object.stdin[%s] is nil", key)
				}

				item, e := value.Object().Get(key)
				if e != nil {
					return ret, fmt.Errorf(
						"object.stdin[%s] error: %s", key, e.Error(),
					)
				}
				if !item.IsString() {
					return ret, fmt.Errorf(
						"object.stdin[%s] must be string", key,
					)
				}

				stdin = append(stdin, item.String())
				startIndex++
			}
			ret.Stdin = stdin
		case "file":
			if !value.IsString() {
				return ret, fmt.Errorf("object.%s must be string", key)
			}
			ret.File = value.String()
		default:
			return ret, fmt.Errorf("object.%s is not supported", key)
		}
	}

	return ret, nil
}

type DbotObject struct {
	stdout *bytes.Buffer
	stderr *bytes.Buffer
	ctx    *Context
}

func (p *DbotObject) Log(call otto.FunctionCall) otto.Value {
	for _, v := range call.ArgumentList {
		p.stdout.WriteString(v.String())
	}
	p.stdout.WriteString("\n")
	return otto.Value{}
}

func (p *DbotObject) Error(call otto.FunctionCall) otto.Value {
	for _, v := range call.ArgumentList {
		p.stderr.WriteString(v.String())
	}
	p.stderr.WriteString("\n")
	return otto.Value{}
}

func (p *DbotObject) Command(call otto.FunctionCall) otto.Value {
	usage := "dbot.Command({\n\texec: 'echo \"hello\"'\n})"

	if len(call.ArgumentList) != 1 {
		p.stderr.WriteString(fmt.Sprintf(
			"dbot.Command(object): arguments error\nUsage: %s\n",
			usage,
		))
		return otto.Value{}
	}

	arg := call.ArgumentList[0].Object()
	if arg == nil {
		p.stderr.WriteString(fmt.Sprintf(
			"dbot.Command(object): argument is nil\nUsage: %s\n",
			usage,
		))
		return otto.Value{}
	}

	cmd, e := parseCommandFromObject(arg)
	if e != nil {
		p.stderr.WriteString(fmt.Sprintf(
			"dbot.Command(object): %s\nUsage: %s\n",
			e.Error(),
			usage,
		))
		return otto.Value{}
	}

	subCtx := p.ctx.subContext(cmd)

	if subCtx != nil {
		subCtx.Run()
	}

	return otto.Value{}
}
