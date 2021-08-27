package dbot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fatih/color"
)

type Manager struct {
	jobName    string
	configPath string
	logCH      chan *logRecord
	config     *Config
	sshMap     map[string]*SSHRunner

	sync.Mutex
}

func NewManager(configPath string, jobName string) *Manager {
	ret := &Manager{
		configPath: configPath,
		jobName:    jobName,
		config:     &Config{},
		logCH:      make(chan *logRecord, 65536),
	}

	go func() {
		headInfoColor := color.New(color.FgGreen, color.Bold)
		headErrorColor := color.New(color.FgRed, color.Bold)
		headJobColor := color.New(color.FgYellow, color.Bold)
		headCommandColor := color.New(color.FgGreen, color.Bold)
		bodyInfoColor := color.New(color.FgBlue, color.Bold)
		bodyErrorColor := color.New(color.FgRed, color.Bold)
		bodyJobColor := color.New(color.FgYellow, color.Bold)
		bodyCommandColor := color.New(color.FgGreen, color.Bold)

		for {
			if log, ok := <-ret.logCH; !ok {
				return
			} else {
				if log.body != "" {
					switch log.level {
					case recordLevelInfo:
						headInfoColor.Printf(
							"<%s:%s>: ", log.runAt, log.jobName,
						)
						bodyInfoColor.Print(log.body)
					case recordLevelError:
						headErrorColor.Printf(
							"<%s:%s>: ", log.runAt, log.jobName,
						)
						bodyErrorColor.Print(log.body)
					case recordLevelJob:
						headJobColor.Printf(
							"<%s:%s>: ", log.runAt, log.jobName,
						)
						bodyJobColor.Print(log.body)
					case recordLevelCommand:
						headCommandColor.Printf(
							"<%s:%s>: ", log.runAt, log.jobName,
						)
						bodyCommandColor.Print(log.body)
					}
				}
			}
		}
	}()

	return ret
}

func (p *Manager) Close() {
	p.Lock()
	defer p.Unlock()

	for len(p.logCH) != 0 {
		time.Sleep(100 * time.Millisecond)
	}

	close(p.logCH)
}

func (p *Manager) Run() {
	p.Lock()
	defer p.Unlock()

	// init the manager
	p.sshMap = make(map[string]*SSHRunner)

	fnReportError := func(e error) {
		p.logCH <- newLogRecordError("dbot", "core", e.Error())
	}

	if configBytes, e := ioutil.ReadFile(p.configPath); e != nil {
		fnReportError(e)
		return
	} else if e := json.Unmarshal(configBytes, p.config); e != nil {
		fnReportError(e)
		return
	} else {
		for name, remote := range p.config.Remotes {
			if name == "local" {
				fnReportError(
					fmt.Errorf("remote name \"local\" is not allowed"),
				)
				return
			}
			ssh, e := NewSSHRunner(name, remote.Port, remote.User, remote.Host)
			if e != nil {
				fnReportError(e)
				return
			}
			p.sshMap[name] = ssh
		}
	}

	if e := p.runJob("local", p.jobName, p.config.Env, false); e != nil {
		fnReportError(e)
		return
	}
}

func (p *Manager) getRunner(runAt string) (CommandRunner, bool) {
	if runAt == "local" {
		return &LocalRunner{}, true
	}

	if sshRunner, ok := p.sshMap[runAt]; ok {
		return sshRunner, true
	}

	return nil, false
}

func (p *Manager) runCommand(
	runAt string, jobName string, env map[string]string, command Command,
) (ret error) {
	runner, ok := p.getRunner(runAt)
	if !ok {
		return fmt.Errorf(
			"could not find remote \"%s\" in config file",
			runAt,
		)
	}

	cmdRunAt := command.RunAt
	if cmdRunAt == "" {
		cmdRunAt = runAt
	}

	cmdType := command.Type
	if cmdType == "" {
		cmdType = "command"
	}

	if cmdType == "job" {
		if ret = p.runJob(
			cmdRunAt, command.Value, env, false,
		); ret != nil {
			return
		}
	} else if cmdType == "command" {
		if command.Value != "" {
			// print the command
			if command.Input != "" {
				p.logCH <- newLogRecordCommand(
					runAt,
					jobName,
					"Command: "+command.Value+" Input: "+command.Input+"\n",
				)
			} else {
				p.logCH <- newLogRecordCommand(
					runAt,
					jobName,
					"Command: "+command.Value+"\n",
				)
			}

			if ret = runner.RunCommand(
				jobName,
				ReplaceStringByEnv(command.Value, env),
				ReplaceStringByEnv(command.Input, env),
				false,
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
	runAt string, jobName string, jobEnv map[string]string, isHandlerError bool,
) error {
	// report job message
	startMsg := "Job Start!\n"
	endMsg := "Job End!\n"

	if isHandlerError {
		startMsg = "ErrorHandler Start!\n"
		endMsg = "ErrorHandler End!\n"
	}

	p.logCH <- newLogRecordJob(runAt, jobName, startMsg)
	defer func() {
		p.logCH <- newLogRecordJob(runAt, jobName, endMsg)
	}()

	// get job parameters
	job, ok := p.config.Jobs[jobName]
	if !ok {
		return fmt.Errorf(
			"could not find job \"%s\" in config file",
			jobName,
		)
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
				runAt, jobName, MergeEnv(jobEnv, command.Env), command,
			); e != nil {
				if !isHandlerError && len(job.ErrorHandler) > 0 {
					_ = p.runJob(runAt, jobName, jobEnv, true)
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
					runAt, jobName, MergeEnv(jobEnv, command.Env), command,
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
				_ = p.runJob(runAt, jobName, jobEnv, true)
			}
			return <-errCH
		}
	}

	return nil
}
