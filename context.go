package dbot

// type Context struct {
// 	manager *Manager
// 	task    *Task
// 	runner  CommandRunner
// 	config  string
// 	jobName string
// 	path    string
// 	env     Env
// 	cmd     *Command
// }

// func NewContext(
// 	manager *Manager,
// 	task *Task,
// 	runner CommandRunner,
// 	config string,
// 	jobName string,
// 	path string,
// 	env Env,
// 	cmd *Command,
// ) *Context {
// 	return &Context{
// 		manager: manager,
// 		task:    task,
// 		runner:  runner,
// 		config:  config,
// 		jobName: jobName,
// 		path:    path,
// 		env:     env,
// 		cmd:     cmd,
// 	}
// }

// func (p *Context) ParseValue(v string) string {
// 	return p.env.parseString(v)
// }

// func (p *Context) SetConfig(config string) *Context {
// 	p.config = config
// 	return p
// }

// func (p *Context) SetEnv(env Env) *Context {
// 	p.env = env
// 	return p
// }

// func (p *Context) SetPath(path string) *Context {
// 	p.path = path
// 	return p
// }

// func (p *Context) SetPathf(format string, a ...interface{}) *Context {
// 	p.path = fmt.Sprintf(format, a...)
// 	return p
// }

// func (p *Context) SetRunner(runner CommandRunner) *Context {
// 	p.runner = runner
// 	return p
// }

// func (p *Context) SetCommand(cmd *Command) *Context {
// 	p.cmd = cmd
// 	return p
// }

// func (p *Context) SetJobName(jobName string) *Context {
// 	p.jobName = jobName
// 	return p
// }

// func (p *Context) Clone() *Context {
// 	return &Context{
// 		manager: p.manager,
// 		runner:  p.runner,
// 		task:    p.task,
// 		config:  p.config,
// 		jobName: p.jobName,
// 		path:    p.path,
// 		env:     p.env,
// 		cmd:     p.cmd,
// 	}
// }

// func (p *Context) GetRunners(on string) ([]CommandRunner, error) {
// 	ret := make([]CommandRunner, 0)

// 	for _, groupKey := range strings.Split(on, ",") {
// 		groupKey = strings.TrimSpace(groupKey)

// 		if groupKey == "local" {
// 			ret = append(ret, p.manager.runnerMap["local"])
// 		} else {
// 			groupValue, ok := p.task.groupMap[groupKey]
// 			if !ok {
// 				return ret, fmt.Errorf("could not find \"%s\"", groupKey)
// 			}

// 			for _, key := range groupValue {
// 				runner, ok := p.manager.runnerMap[key]
// 				if !ok {
// 					return ret, fmt.Errorf("could not find runner \"%s\"", key)
// 				}
// 				ret = append(ret, runner)
// 			}
// 		}
// 	}

// 	if len(ret) == 0 {
// 		return ret, fmt.Errorf("could not find any runners")
// 	}

// 	return ret, nil
// }

// func (p *Context) GetUserInput(desc string, mode string) (string, error) {
// 	var e error = nil
// 	var ret string = ""
// 	log(desc, color.FgMagenta)
// 	defer log("\n", color.FgGreen)
// 	switch mode {
// 	case "password":
// 		if b, err := term.ReadPassword(int(syscall.Stdin)); err != nil {
// 			e = err
// 		} else {
// 			ret = string(b)
// 		}
// 	case "text":
// 		if _, err := fmt.Scanf("%s", &ret); err != nil {
// 			e = err
// 		}
// 	default:
// 		e = fmt.Errorf("unsupported mode %s", mode)
// 	}

// 	return ret, e
// }

// func (p *Context) LogRawInfo(v string) {
// 	log(getStandradOut(v), color.FgBlue)
// }

// func (p *Context) LogRawError(v string) {
// 	log(getStandradOut(v), color.FgRed)
// }

// func (p *Context) LogInfo(v string) {
// 	p.Log(v, "")
// }

// func (p *Context) LogInfof(format string, a ...interface{}) {
// 	p.Log(fmt.Sprintf(format, a...), "")
// }

// func (p *Context) LogError(v string) {
// 	p.Log("", v)
// }

// func (p *Context) LogErrorf(format string, a ...interface{}) {
// 	p.Log("", fmt.Sprintf(format, a...))
// }

// func (p *Context) Log(outStr string, errStr string) {
// 	logItems := []interface{}{}

// 	logItems = append(logItems, p.runner.Name())
// 	logItems = append(logItems, color.FgYellow)

// 	if p.config != "" {
// 		logItems = append(logItems, " > ")
// 		logItems = append(logItems, color.FgCyan)
// 		logItems = append(logItems, p.config)
// 		logItems = append(logItems, color.FgYellow)
// 	}

// 	if p.path != "" {
// 		logItems = append(logItems, " > ")
// 		logItems = append(logItems, color.FgCyan)
// 		logItems = append(logItems, p.path)
// 		logItems = append(logItems, color.FgYellow)
// 	}

// 	logItems = append(logItems, "\n")
// 	logItems = append(logItems, color.FgGreen)

// 	if p.cmd != nil && p.cmd.Exec != "" {
// 		logItems = append(logItems, getStandradOut(p.cmd.Exec))
// 		logItems = append(logItems, color.FgBlue)
// 	}

// 	if outStr != "" {
// 		logItems = append(logItems, getStandradOut(outStr))
// 		logItems = append(logItems, color.FgGreen)
// 	}

// 	if errStr != "" {
// 		logItems = append(logItems, getStandradOut(errStr))
// 		logItems = append(logItems, color.FgRed)
// 	}

// 	log(logItems...)
// }
