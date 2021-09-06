package dbot

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/robertkrimen/otto"
)

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
	seed   int
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
		subCtx.Clone("%s.exec-script.dbot.Command[%d]", p.ctx.path, p.seed).Run()
	}

	p.seed++

	return otto.Value{}
}
