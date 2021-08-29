package dbot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

var rootEnv = Env{
	"KeyESC":   "\033",
	"KeyEnter": "\n",
}

var outFilter = []string{
	"\033",
}

var errFilter = []string{
	"Output is not to a terminal",
	"Input is not from a terminal",
}

type Manager struct {
	runnerMap map[string]CommandRunner
	configMap map[string]*Config
}

func NewManager() *Manager {
	ret := &Manager{
		runnerMap: make(map[string]CommandRunner),
		configMap: make(map[string]*Config),
	}

	ret.runnerMap["local"] = &LocalRunner{name: os.Getenv("USER") + "@local"}

	return ret
}

func (p *Manager) getConfig(configPath string) (*Config, error) {
	if absConfigPath, e := filepath.Abs(configPath); e != nil {
		return nil, e
	} else if config, ok := p.configMap[absConfigPath]; !ok {
		return nil, fmt.Errorf("could not find config \"%s\"", configPath)
	} else {
		return config, nil
	}
}

func (p *Manager) getJob(configPath string, jobName string) (*Job, error) {
	if config, e := p.getConfig(configPath); e != nil {
		return nil, e
	} else if job, ok := config.Jobs[jobName]; !ok {
		return nil, fmt.Errorf(
			"could not find job \"%s\"", p.getJobFullPath(configPath, jobName),
		)
	} else {
		return &job, nil
	}
}

func (p *Manager) getJobFullPath(configPath string, jobName string) string {
	if absConfigPath, e := filepath.Abs(configPath); e != nil {
		return ""
	} else {
		return absConfigPath + " => jobs => " + jobName
	}
}

func (p *Manager) getRunner(
	configPath string,
	runAt string,
) (CommandRunner, error) {
	if config, e := p.getConfig(configPath); e != nil {
		return nil, e
	} else if runAt == "local" {
		return p.runnerMap["local"], nil
	} else if remote, ok := config.Remotes[runAt]; !ok {
		return nil, fmt.Errorf(
			"could not find runner \"%s\" in config file \"%s\"",
			runAt, configPath,
		)
	} else {
		return p.runnerMap[fmt.Sprintf("%s@%s", remote.User, remote.Host)], nil
	}
}

func (p *Manager) prepareJob(
	jobName string,
	configPath string,
	parentDebug []string,
) error {
	currentDebug := append(parentDebug, p.getJobFullPath(configPath, jobName))

	// get config absolute path
	absConfigPath, e := filepath.Abs(configPath)
	if e != nil {
		return e
	}

	// load config
	config, ok := p.configMap[absConfigPath]
	if !ok {
		jsonConfig := Config{}
		configBytes, e := ioutil.ReadFile(absConfigPath)
		if e != nil {
			return e
		}

		ext := filepath.Ext(absConfigPath)
		if ext == ".json" {
			if e := json.Unmarshal(configBytes, &jsonConfig); e != nil {
				return e
			}
		} else if ext == ".yml" || ext == ".yaml" {
			if e := yaml.Unmarshal(configBytes, &jsonConfig); e != nil {
				return e
			}
		} else {
			return fmt.Errorf(
				"illegal config file extension \"%s\"",
				absConfigPath,
			)
		}

		config = &jsonConfig
		p.configMap[absConfigPath] = config
	}

	// prepare job
	if job, ok := config.Jobs[jobName]; ok {
		for _, cmd := range job.Commands {
			if cmd.On != "" && cmd.On != "local" {
				if remote, ok := config.Remotes[cmd.On]; ok {
					userHost := remote.User + "@" + remote.Host
					if _, ok := p.runnerMap[userHost]; !ok {
						ssh, e := NewSSHRunner(
							userHost,
							remote.Port,
							remote.User,
							remote.Host,
						)
						if e != nil {
							return e
						}
						p.runnerMap[userHost] = ssh
					}
				} else {
					return fmt.Errorf(
						"remote \"%s\" is not found in config \"%s\"",
						cmd.On,
						absConfigPath,
					)
				}
			}

			if cmd.Type == "job" {
				cmdConfig := cmd.Config
				if cmdConfig == "" {
					cmdConfig = configPath
				}
				if e := p.prepareJob(
					cmd.Exec, cmdConfig, currentDebug,
				); e != nil {
					return e
				}
			}
		}
	}

	return nil
}

func (p *Manager) Run(configPath string, jobName string) bool {
	if e := p.prepareJob(jobName, configPath, []string{}); e != nil {
		LogError(os.Getenv("User")+"@dbot => loading config", e)
		return false
	} else if runner, e := p.getRunner(configPath, "local"); e != nil {
		LogError(os.Getenv("User")+"@dbot => get local runner", e)
		return false
	} else {
		return p.runJob(configPath, jobName, Env{}, runner)
	}
}

func (p *Manager) runCommand(
	jobConfig string,
	jobName string,
	jobEnv Env,
	command Command,
	defaultRunner CommandRunner,
) bool {
	head := defaultRunner.Name() + " => " + jobName + ": "

	runner := defaultRunner
	if command.On != "" {
		v, e := p.getRunner(jobConfig, command.On)
		if e != nil {
			LogError(head, e)
			return false
		}
		runner = v
	}

	cmdType := command.Type
	if cmdType == "" {
		cmdType = "cmd"
	}

	env := jobEnv.merge(command.Env)

	if cmdType == "job" {
		cmdConfig := command.Config
		if cmdConfig == "" {
			cmdConfig = jobConfig
		}
		return p.runJob(
			cmdConfig,
			env.parseString(command.Exec),
			jobEnv.parseEnv(command.Env),
			runner,
		)
	} else if cmdType == "cmd" {
		return runner.RunCommand(
			jobName,
			env.parseString(command.Exec),
			env.parseStringArray(command.Inputs),
		)
	} else {
		LogError(head, fmt.Errorf("unknown command type %s", cmdType))
		return false
	}
}

func (p *Manager) runJob(
	jobConfig string,
	jobName string,
	runningEnv Env,
	defaultRunner CommandRunner,
) bool {
	head := defaultRunner.Name() + " => " + jobName + ": "
	LogNotice(head, "Start Job: "+jobConfig+" => "+jobName+"\n")
	defer LogNotice(head, "End Job: "+jobConfig+" => "+jobName+"\n")

	config, e := p.getConfig(jobConfig)
	if e != nil {
		LogError(head, e)
		return false
	}

	job, e := p.getJob(jobConfig, jobName)
	if e != nil {
		LogError(head, e)
		return false
	}

	concurrency := job.Concurrency
	commands := job.Commands

	env := rootEnv.merge(config.Env).merge(job.Env).merge(runningEnv)

	if !concurrency {
		for i := 0; i < len(commands); i++ {
			command := commands[i]
			if !p.runCommand(jobConfig, jobName, env, command, defaultRunner) {
				return false
			}
		}

		return true
	} else {
		retCH := make(chan bool, len(commands))

		for i := 0; i < len(commands); i++ {
			go func(command Command) {
				retCH <- p.runCommand(
					jobConfig,
					jobName,
					env,
					command,
					defaultRunner,
				)
			}(commands[i])
		}

		ret := true
		for i := 0; i < len(commands); i++ {
			if !<-retCH {
				ret = false
			}
		}

		return ret
	}
}
