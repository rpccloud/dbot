package dbot

type Remote struct {
	Port uint16 `json:"port"`
	User string `json:"user"`
	Host string `json:"host"`
}

type Job struct {
	Concurrency  bool      `json:"concurrency"`
	Commands     []Command `json:"commands"`
	ErrorHandler []Command `json:"errorHandler"`
	Env          Env       `json:"env"`
}

type Command struct {
	Type   string   `json:"type"`
	Value  string   `json:"value"`
	RunAt  string   `json:"runAt"`
	Inputs []string `json:"inputs"`
	Env    Env      `json:"env"`
	Config string   `json:"config"`
}

type Config struct {
	Name    string            `json:"name"`
	Remotes map[string]Remote `json:"remotes"`
	Jobs    map[string]Job    `json:"jobs"`
	Env     map[string]string `json:"env"`
}
