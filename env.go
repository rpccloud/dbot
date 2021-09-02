package dbot

import (
	"strings"
)

type Env map[string]string

func (p Env) ParseString(v string, defaultStr string, trimSpace bool) string {
	replaceArray := make([]string, 0)
	for key, value := range p {
		replaceArray = append(replaceArray, "${"+key+"}", value)
	}

	replacer := strings.NewReplacer(replaceArray...)
	ret := replacer.Replace(v)

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
