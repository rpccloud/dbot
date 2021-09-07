package dbot

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/robertkrimen/otto"
)

func parseValueToEnv(path string, value otto.Value) (Env, error) {
	ret := Env{}

	if !value.IsObject() {
		return ret, fmt.Errorf("%s must be object", path)
	}

	for _, key := range value.Object().Keys() {
		if value.Object() == nil {
			return ret, fmt.Errorf("%s.%s is nil", path, key)
		} else if item, e := value.Object().Get(key); e != nil {
			return ret, fmt.Errorf("%s.%s error: %s", path, key, e.Error())
		} else if !item.IsString() {
			return ret, fmt.Errorf("%s.%s must be string", path, key)
		} else {
			ret[key] = item.String()
		}
	}

	return ret, nil
}

func parseValueToStdin(path string, value otto.Value) ([]string, error) {
	ret := []string{}

	if !value.IsObject() {
		return ret, fmt.Errorf("%s must be object", path)
	}

	for _, key := range value.Object().Keys() {
		if strconv.FormatInt(int64(len(ret)), 10) != key {
			return ret, fmt.Errorf("%s must be array", path)
		} else if value.Object() == nil {
			return ret, fmt.Errorf("%s[%s] is nil", path, key)
		} else if item, e := value.Object().Get(key); e != nil {
			return ret, fmt.Errorf("%s[%s] error: %s", path, key, e.Error())
		} else if !item.IsString() {
			return ret, fmt.Errorf("%s[%s] must be string", path, key)
		} else {
			ret = append(ret, item.String())
		}
	}

	return ret, nil
}

func parseObjectToCommand(object *otto.Object) (*Command, error) {
	ret := &Command{}
	keys := object.Keys()
	for _, key := range keys {
		value, e := object.Get(key)
		if e != nil {
			return nil, fmt.Errorf("%s error: %s", key, e.Error())
		}

		switch key {
		case "tag":
			if !value.IsString() {
				return nil, fmt.Errorf("tag must be string")
			}
			ret.Tag = value.String()
		case "exec":
			if !value.IsString() {
				return nil, fmt.Errorf("exec must be string")
			}
			ret.Exec = value.String()
		case "on":
			if !value.IsString() {
				return nil, fmt.Errorf("on must be string")
			}
			ret.On = value.String()
		case "stdin":
			stdin, e := parseValueToStdin("stdin", value)
			if e != nil {
				return nil, e
			}
			ret.Stdin = stdin
		case "env":
			env, e := parseValueToEnv("env", value)
			if e != nil {
				return nil, e
			}
			ret.Env = env
		case "args":
			args, e := parseValueToEnv("args", value)
			if e != nil {
				return nil, e
			}
			ret.Args = args
		case "file":
			if !value.IsString() {
				return nil, fmt.Errorf("file must be string")
			}
			ret.File = value.String()
		default:
			return nil, fmt.Errorf("%s is not supported", key)
		}
	}

	return ret, nil
}

type DbotObject struct {
	vm     *otto.Otto
	stdout *bytes.Buffer
	stderr *bytes.Buffer
	ctx    *Context
	seed   int64
}

func (p *DbotObject) LogInfo(call otto.FunctionCall) otto.Value {
	for _, v := range call.ArgumentList {
		p.stdout.WriteString(v.String())
	}
	return otto.Value{}
}

func (p *DbotObject) LogError(call otto.FunctionCall) otto.Value {
	for _, v := range call.ArgumentList {
		p.stderr.WriteString(v.String())
	}
	return otto.Value{}
}

func (p *DbotObject) Command(call otto.FunctionCall) otto.Value {
	retFalse, _ := p.vm.ToValue(false)
	retTrue, _ := p.vm.ToValue(true)
	idx := p.seed
	p.seed++

	if len(call.ArgumentList) != 1 {
		_, _ = p.vm.Call("new Error", nil, "arguments length error")
		return retFalse
	}

	arg0 := call.ArgumentList[0].Object()
	if arg0 == nil {
		_, _ = p.vm.Call("new Error", nil, "argument is nil")
		return retFalse
	}

	if cmd, e := parseObjectToCommand(arg0); e != nil {
		_, _ = p.vm.Call("new Error", nil, e.Error())
		return retFalse
	} else if ctx := p.ctx.subContext(cmd); ctx == nil {
		return retFalse
	} else if !ctx.Clone("%s.script.dbot.Command[%d]", p.ctx.path, idx).Run() {
		return retFalse
	} else {
		return retTrue
	}
}
