package dbot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fatih/color"
)

var rootEnv = map[string]string{
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
	logCH     chan *logRecord
	runnerMap map[string]CommandRunner
	configMap map[string]*Config
	closeCH   chan bool
	sync.Mutex
}

func NewManager() *Manager {
	ret := &Manager{
		logCH:     make(chan *logRecord, 65536),
		runnerMap: make(map[string]CommandRunner),
		configMap: make(map[string]*Config),
		closeCH:   make(chan bool, 1),
	}

	ret.runnerMap["local"] = &LocalRunner{name: os.Getenv("USER") + "@local"}

	go ret.printLog()

	return ret
}

func (p *Manager) Close() {
	p.Lock()
	defer p.Unlock()

	if p.closeCH != nil {
		for len(p.logCH) != 0 {
			time.Sleep(100 * time.Millisecond)
		}

		close(p.logCH)
		<-p.closeCH
		p.closeCH = nil
	}
}

func (p *Manager) printLog() {
	defer func() {
		p.closeCH <- true
	}()

	headInfoColor := color.New(color.FgGreen, color.Bold)
	headErrorColor := color.New(color.FgRed, color.Bold)
	headJobColor := color.New(color.FgYellow, color.Bold)
	headCommandColor := color.New(color.FgGreen, color.Bold)
	bodyInfoColor := color.New(color.FgBlue, color.Bold)
	bodyErrorColor := color.New(color.FgRed, color.Bold)
	bodyJobColor := color.New(color.FgYellow, color.Bold)
	bodyCommandColor := color.New(color.FgGreen, color.Bold)

	for {
		if log, ok := <-p.logCH; !ok {
			return
		} else {
			if log.body != "" {
				switch log.level {
				case recordLevelInfo:
					if !FilterString(log.body, outFilter) {
						headInfoColor.Printf(
							"%s => %s: ", log.runAt, log.jobName,
						)
						bodyInfoColor.Print(log.body)
					}
				case recordLevelError:
					if !FilterString(log.body, errFilter) {
						headErrorColor.Printf(
							"%s => %s: ", log.runAt, log.jobName,
						)
						bodyErrorColor.Print(log.body)
					}

				case recordLevelJob:
					headJobColor.Printf(
						"%s => %s: ", log.runAt, log.jobName,
					)
					bodyJobColor.Print(log.body)
				case recordLevelCommand:
					headCommandColor.Printf(
						"%s => %s: ", log.runAt, log.jobName,
					)
					bodyCommandColor.Print(log.body)
				}
			}
		}
	}
}

func (p *Manager) reportError(e error) {
	p.logCH <- newLogRecordError("dbot", "core", e.Error())
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
		if configBytes, e := ioutil.ReadFile(absConfigPath); e != nil {
			return e
		} else if e := json.Unmarshal(configBytes, &jsonConfig); e != nil {
			return e
		} else {
			config = &jsonConfig
			p.configMap[absConfigPath] = config
		}
	}

	// prepare job
	if job, ok := config.Jobs[jobName]; ok {
		commands := append(job.Commands, job.ErrorHandler...)

		for _, cmd := range commands {
			if cmd.RunAt != "" && cmd.RunAt != "local" {
				if remote, ok := config.Remotes[cmd.RunAt]; ok {
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
					return fmt.Errorf("remote \"%s\" is not found", cmd.RunAt)
				}
			}

			if cmd.Type == "job" {
				cmdConfig := cmd.Config
				if cmdConfig == "" {
					cmdConfig = configPath
				}
				if e := p.prepareJob(
					cmd.Value, cmdConfig, currentDebug,
				); e != nil {
					return e
				}
			}
		}
	}

	return nil
}

func (p *Manager) Run(configPath string, jobName string) bool {
	p.Lock()
	defer p.Unlock()

	if e := p.prepareJob(jobName, configPath, []string{}); e != nil {
		p.reportError(e)
		return false
	} else if config, e := p.getConfig(configPath); e != nil {
		p.reportError(e)
		return false
	} else if runner, e := p.getRunner(configPath, "local"); e != nil {
		p.reportError(e)
		return false
	} else if e := p.runJob(
		configPath,
		jobName,
		MergeEnv(rootEnv, config.Env),
		false,
		runner,
	); e != nil {
		p.reportError(e)
		return false
	} else {
		return true
	}
}

func (p *Manager) runCommand(
	jobConfig string,
	jobName string,
	env map[string]string,
	command Command,
	defaultRunner CommandRunner,
) (ret error) {
	runner := defaultRunner
	if command.RunAt != "" {
		if runner, ret = p.getRunner(jobConfig, command.RunAt); ret != nil {
			return
		}
	}

	cmdType := command.Type
	if cmdType == "" {
		cmdType = "cmd"
	}

	if cmdType == "job" {
		cmdConfig := command.Config
		if cmdConfig == "" {
			cmdConfig = jobConfig
		}
		if ret = p.runJob(
			cmdConfig, command.Value, env, false, runner,
		); ret != nil {
			return
		}
	} else if cmdType == "cmd" {
		if command.Value != "" {
			// print the command
			p.logCH <- newLogRecordCommand(
				runner.Name(),
				jobName,
				"Command: "+command.Value+"\n",
			)

			// parse inputs
			inputs := make([]string, len(command.Inputs))
			for i := 0; i < len(command.Inputs); i++ {
				inputs[i] = ReplaceStringByEnv(command.Inputs[i], env)
			}

			if ret = runner.RunCommand(
				jobName,
				ReplaceStringByEnv(command.Value, env),
				inputs,
				p.logCH,
			); ret != nil {
				return
			}
		}
	} else {
		ret = fmt.Errorf("unknown command type %s", cmdType)
	}

	return
}

func (p *Manager) runJob(
	jobConfig string,
	jobName string,
	jobEnv map[string]string,
	isHandlerError bool,
	defaultRunner CommandRunner,
) error {
	// report job message
	startMsg := "Job Start!\n"
	endMsg := "Job End!\n"

	if isHandlerError {
		startMsg = "ErrorHandler Start!\n"
		endMsg = "ErrorHandler End!\n"
	}

	p.logCH <- newLogRecordJob(defaultRunner.Name(), jobName, startMsg)
	defer func() {
		p.logCH <- newLogRecordJob(defaultRunner.Name(), jobName, endMsg)
	}()

	job, e := p.getJob(jobConfig, jobName)
	if e != nil {
		return e
	}
	concurrency := job.Concurrency
	commands := job.Commands
	if isHandlerError {
		commands = job.ErrorHandler
		concurrency = false
	}

	if !concurrency {
		for i := 0; i < len(commands); i++ {
			command := commands[i]
			if e := p.runCommand(
				jobConfig,
				jobName,
				MergeEnv(jobEnv, command.Env),
				command,
				defaultRunner,
			); e != nil {
				if !isHandlerError && len(job.ErrorHandler) > 0 {
					_ = p.runJob(
						jobConfig,
						jobName,
						jobEnv,
						true,
						defaultRunner,
					)
				}
				return e
			}
		}
	} else {
		evalCount := int64(len(commands))
		errCH := make(chan error, len(commands))

		for i := 0; i < len(commands); i++ {
			go func(command Command) {
				if e := p.runCommand(
					jobConfig,
					jobName,
					MergeEnv(jobEnv, command.Env),
					command,
					defaultRunner,
				); e != nil {
					errCH <- e
				}
				atomic.AddInt64(&evalCount, -1)
			}(commands[i])
		}

		for atomic.LoadInt64(&evalCount) > 0 {
			time.Sleep(100 * time.Millisecond)
		}

		if len(errCH) != 0 {
			if !isHandlerError && len(job.ErrorHandler) > 0 {
				_ = p.runJob(jobConfig, jobName, jobEnv, true, defaultRunner)
			}
			return <-errCH
		}
	}

	return nil
}
