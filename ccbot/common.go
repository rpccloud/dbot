package ccbot

import (
	"os"
	"strings"
	"sync"

	"github.com/fatih/color"
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
