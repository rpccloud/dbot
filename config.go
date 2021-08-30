package dbot

type Remote struct {
	Port uint16
	User string
	Host string
}

type Job struct {
	Concurrency bool
	Commands    []Command
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

type Config struct {
	Name    string
	Remotes map[string]Remote
	Jobs    map[string]Job
	Env     Env
}
