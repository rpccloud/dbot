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
}

type Command struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	RunAt string `json:"runAt"`
	Input string `json:"input"`
}

type Config struct {
	Name    string            `json:"name"`
	Remotes map[string]Remote `json:"remotes"`
	Jobs    map[string]Job    `json:"jobs"`
}
