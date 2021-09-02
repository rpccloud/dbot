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
	return &Command{
		Type:   p.Type,
		Exec:   p.Exec,
		On:     p.On,
		Inputs: append([]string{}, p.Inputs...),
		Env:    p.Env.Merge(nil),
		Config: p.Config,
	}
}

type JobConfig struct {
	Name string
	Jobs map[string]*Job
}
