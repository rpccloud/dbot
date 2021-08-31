package dbot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

var rootEnv = Env{
	"KeyESC": EnvItem{
		Value: "\033",
	},
	"KeyEnter": EnvItem{
		Value: "\n",
	},
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
	if config, ok := p.configMap[configPath]; !ok {
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
	return configPath + " > " + jobName
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
	jobConfig string,
	parentDebug []string,
	runningEnv Env,
) error {
	currentDebug := append(parentDebug, p.getJobFullPath(jobConfig, jobName))

	// load config
	config, ok := p.configMap[jobConfig]
	if !ok {
		jsonConfig := Config{}
		configBytes, e := ioutil.ReadFile(jobConfig)
		if e != nil {
			return e
		}

		ext := filepath.Ext(jobConfig)
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
				jobConfig,
			)
		}

		config = &jsonConfig

		// init config env
		env, e := config.Env.initialize("init config env: ", jobConfig)
		if e != nil {
			return e
		}
		config.Env = env
		p.configMap[jobConfig] = config
	}

	// prepare job
	if job, ok := config.Jobs[jobName]; ok {
		for _, cmd := range job.Commands {
			jobEnv := rootEnv.merge(Env{
				"ConfigDir": EnvItem{
					Value: filepath.Dir(jobConfig),
				},
			}).merge(config.Env).merge(job.Env).merge(runningEnv)
			cmdOn := jobEnv.merge(cmd.Env).parseString(cmd.On)
			for _, rawName := range strings.Split(cmdOn, ",") {
				runnerName := strings.TrimSpace(rawName)
				if runnerName != "" && runnerName != "local" {
					if remote, ok := config.Remotes[runnerName]; ok {
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
							runnerName,
							jobConfig,
						)
					}
				}
			}

			if cmd.Type == "job" {
				cmdConfig := jobConfig
				if cmd.Config != "" {
					var e error
					cmdConfig, e = GetAbsConfigPathFrom(jobConfig, cmd.Config)
					if e != nil {
						return fmt.Errorf(
							"\"%s\" is invalid in config file \"%s\" error: %s",
							cmd.Config,
							jobConfig,
							e.Error(),
						)
					}
				}
				if e := p.prepareJob(
					cmd.Exec, cmdConfig, currentDebug, jobEnv.parseEnv(cmd.Env),
				); e != nil {
					return e
				}
			}

			// init command env
			env, e := cmd.Env.initialize(
				"init cmd env: ",
				jobConfig+" > "+jobName+" > "+cmd.Exec,
			)
			if e != nil {
				return e
			}
			cmd.Env = env
		}

		// init job env
		env, e := job.Env.initialize("init job env: ", jobConfig+" > "+jobName)
		if e != nil {
			return e
		}
		job.Env = env
	}

	return nil
}

func (p *Manager) Run(configPath string, jobName string) bool {
	if absConfigPath, e := filepath.Abs(configPath); e != nil {
		LogError(os.Getenv("User")+"@dbot > loading config: ", e.Error())
		return false
	} else if e := p.prepareJob(
		jobName, absConfigPath, []string{}, Env{},
	); e != nil {
		LogError(os.Getenv("User")+"@dbot > loading config: ", e.Error())
		return false
	} else if runner, e := p.getRunner(absConfigPath, "local"); e != nil {
		LogError(os.Getenv("User")+"@dbot > get local runner: ", e.Error())
		return false
	} else {
		return p.runJob(absConfigPath, jobName, Env{}, runner)
	}
}

func (p *Manager) runCommand(
	jobConfig string,
	jobName string,
	jobEnv Env,
	cmd Command,
	defaultRunner CommandRunner,
) bool {
	head := defaultRunner.Name() + " > " + jobName + ": "

	runners := []CommandRunner{}
	if cmd.On == "" {
		runners = append(runners, defaultRunner)
	} else {
		for _, rawName := range strings.Split(cmd.On, ",") {
			if runnerName := strings.TrimSpace(rawName); runnerName != "" {
				v, e := p.getRunner(jobConfig, runnerName)
				if e != nil {
					LogError(head, e.Error())
					return false
				}
				runners = append(runners, v)
			}
		}

		if len(runners) == 0 {
			LogError(head, fmt.Sprintf(
				"could not find runner \"%s\" in config file \"%s\"",
				cmd.On, jobConfig,
			))
			return false
		}
	}

	cmdType := cmd.Type
	if cmdType == "" {
		cmdType = "cmd"
	}

	env := jobEnv.merge(cmd.Env)

	if cmdType == "job" {
		cmdConfig := jobConfig
		if cmd.Config != "" {
			var e error
			cmdConfig, e = GetAbsConfigPathFrom(jobConfig, cmd.Config)
			if e != nil {
				LogError(head, fmt.Sprintf(
					"\"%s\" is invalid in config file \"%s\" error: %s",
					cmd.Config,
					jobConfig,
					e.Error(),
				))
				return false
			}
		}

		for _, runner := range runners {
			if !p.runJob(
				cmdConfig,
				env.parseString(cmd.Exec),
				jobEnv.parseEnv(cmd.Env),
				runner,
			) {
				return false
			}
		}

		return true
	} else if cmdType == "cmd" {
		for _, runner := range runners {
			if !runner.RunCommand(
				jobName,
				env.parseString(cmd.Exec),
				env.parseStringArray(cmd.Inputs),
			) {
				return false
			}
		}

		return true
	} else {
		LogError(head, fmt.Sprintf("unknown command type %s", cmdType))
		return false
	}
}

func (p *Manager) runJob(
	jobConfig string,
	jobName string,
	runningEnv Env,
	defaultRunner CommandRunner,
) (ret bool) {
	head := defaultRunner.Name() + " > " + jobName + ": "
	LogNotice(head, fmt.Sprintf("Job \"%s > %s\" Start", jobConfig, jobName))
	defer func() {
		if ret {
			LogNotice(
				head,
				fmt.Sprintf("Job \"%s > %s\" successful", jobConfig, jobName),
			)
		} else {
			LogError(
				head,
				fmt.Sprintf("Job \"%s > %s\" failed", jobConfig, jobName),
			)
		}
	}()

	config, e := p.getConfig(jobConfig)
	if e != nil {
		LogError(head, e.Error())
		return false
	}

	job, e := p.getJob(jobConfig, jobName)
	if e != nil {
		LogError(head, e.Error())
		return false
	}

	concurrency := job.Concurrency
	commands := job.Commands

	env := rootEnv.merge(Env{
		"ConfigDir": EnvItem{
			Value: filepath.Dir(jobConfig),
		},
	}).merge(config.Env).merge(job.Env).merge(runningEnv)

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
