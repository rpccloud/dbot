package dbot

import (
	"fmt"
	"strings"
)

type EnvItem struct {
	Type  string
	Desc  string
	Value string
}
type Env map[string]EnvItem

func (p Env) parseString(str string) string {
	replaceArray := make([]string, 0)
	for key, it := range p {
		replaceArray = append(replaceArray, "${"+key+"}", it.Value)
	}

	replacer := strings.NewReplacer(replaceArray...)
	return replacer.Replace(str)
}

func (p Env) parseStringArray(arr []string) []string {
	ret := make([]string, len(arr))

	for i := 0; i < len(arr); i++ {
		ret[i] = p.parseString(arr[i])
	}

	return ret
}

func (p Env) parseEnv(env Env) Env {
	ret := make(Env)

	for key, it := range env {
		ret[key] = EnvItem{
			Type:  it.Type,
			Desc:  it.Desc,
			Value: p.parseString(it.Value),
		}
	}

	return ret
}

func (p Env) merge(env Env) Env {
	ret := make(Env)
	for key, value := range p {
		ret[key] = value
	}

	for key, it := range env {
		if it.Type != "password" {
			ret[key] = EnvItem{
				Type:  it.Type,
				Desc:  it.Desc,
				Value: p.parseString(it.Value),
			}
		} else {
			ret[key] = EnvItem{
				Type:  it.Type,
				Desc:  it.Desc,
				Value: it.Value,
			}
		}
	}

	return ret
}

func (p Env) initialize(head string, body string) (Env, error) {
	hasShowNotice := false
	ret := Env{}

	for key, it := range p {
		if it.Type == "password" {
			if !hasShowNotice {
				LogNotice(head, body)
				hasShowNotice = true
			}

			desc := it.Desc
			if desc == "" {
				desc = "password " + key + ": "
			}
			password, e := GetPasswordFromUser(desc)
			if e != nil {
				return Env{}, e
			}
			ret[key] = EnvItem{
				Type:  it.Type,
				Desc:  it.Desc,
				Value: password,
			}
		} else if it.Type == "input" {
			if !hasShowNotice {
				LogNotice(head, body)
				hasShowNotice = true
			}

			input := ""
			desc := it.Desc
			if desc == "" {
				desc = "input " + key + ": "
			}
			LogInput(desc)
			_, e := fmt.Scanf("%s", &input)
			if e != nil {
				return Env{}, e
			}
			ret[key] = EnvItem{
				Type:  it.Type,
				Desc:  it.Desc,
				Value: input,
			}
		} else {
			ret[key] = EnvItem{
				Type:  it.Type,
				Desc:  it.Desc,
				Value: it.Value,
			}
		}
	}

	return ret, nil
}
