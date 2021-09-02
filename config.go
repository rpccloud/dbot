package dbot

type Import struct {
	Name   string
	Config string
}

type Input struct {
	Type string
	Desc string
}

type Remote struct {
	Port uint16
	User string
	Host string
}

type Task struct {
	Inputs  map[string]*Input
	Imports map[string]*Import
	Remotes map[string][]*Remote
	Env     Env
	Job     *Import

	groupMap map[string][]string
}

type MainConfig struct {
	Name  string
	Run   []string
	Tasks map[string]*Task
}

type Job struct {
	Concurrency bool
	Commands    []*Command
	Env         Env
}

type Command struct {
	Type   string
	Exec   string
	On     string
	Inputs []string
	Env    Env
	Config string
}

func (p *Command) Clone() *Command {
	inputs := []string(nil)

	for _, input := range p.Inputs {
		inputs = append(inputs, input)
	}

	return &Command{
		Type:   p.Type,
		Exec:   p.Exec,
		On:     p.On,
		Inputs: inputs,
		Env:    p.Env.merge(nil),
		Config: p.Config,
	}
}

type JobConfig struct {
	Name string
	Jobs map[string]*Job
}
