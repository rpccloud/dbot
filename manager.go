package dbot

// type Manager struct {
// 	runnerMap map[string]CommandRunner
// 	configMap map[string]*JobConfig
// }

// func NewManager() *Manager {
// 	ret := &Manager{
// 		runnerMap: make(map[string]CommandRunner),
// 		configMap: make(map[string]*JobConfig),
// 	}

// 	ret.runnerMap["local"] = &LocalRunner{}

// 	return ret
// }

// func (p *Manager) getConfig(configPath string) (*JobConfig, error) {
// 	if config, ok := p.configMap[configPath]; !ok {
// 		return nil, fmt.Errorf("could not find config \"%s\"", configPath)
// 	} else {
// 		return config, nil
// 	}
// }

// func (p *Manager) getJob(configPath string, jobName string) (*Job, error) {
// 	if config, e := p.getConfig(configPath); e != nil {
// 		return nil, e
// 	} else if job, ok := config.Jobs[jobName]; !ok {
// 		return nil, fmt.Errorf(
// 			"could not find job \"%s\"", p.getJobFullPath(configPath, jobName),
// 		)
// 	} else {
// 		return job, nil
// 	}
// }

// func (p *Manager) getJobFullPath(configPath string, jobName string) string {
// 	return configPath + " > " + jobName
// }

// func (p *Manager) Run(absEntryPath string) bool {
// 	ctx := NewContext(p, nil, &DbotRunner{}, absEntryPath, "", "", nil, nil)
// 	entryConfig := MainConfig{}

// 	if e := loadConfig(absEntryPath, &entryConfig); e != nil {
// 		ctx.LogError(e.Error())
// 		return false
// 	} else {
// 		// start prepare
// 		fnPrepare := func(taskName string) bool {
// 			task, ok := entryConfig.Tasks[taskName]
// 			if !ok {
// 				ctx.SetPathf("tasks.%s", taskName)
// 				ctx.LogError("task not found")
// 				return false
// 			}

// 			// prepare env
// 			env := Env{}.merge(task.Env)
// 			for key, it := range task.Inputs {
// 				ctx.SetPathf("tasks.%s.inputs.%s", taskName, key)
// 				ctx.LogInfo("")

// 				desc := it.Desc
// 				if desc == "" {
// 					desc = "input " + key + ": "
// 				}

// 				value, e := ctx.GetUserInput(desc, it.Type)
// 				if e != nil {
// 					ctx.LogRawError(e.Error() + "\n")
// 					return false
// 				}

// 				env[key] = value
// 			}
// 			task.Env = env

// 			task.groupMap = make(map[string][]string)

// 			fnPrepareRemoteList := func(
// 				ctx *Context,
// 				path string,
// 				key string,
// 				list []*Remote,
// 			) bool {
// 				group := make([]string, 0)
// 				for idx, it := range list {
// 					port := it.Port
// 					if port == 0 {
// 						port = 22
// 					}

// 					id := fmt.Sprintf("%s@%s:%d", it.User, it.Host, port)
// 					if _, ok := p.runnerMap[id]; !ok {

// 						ssh, e := NewSSHRunner(
// 							port,
// 							it.User,
// 							it.Host,
// 						)
// 						if e != nil {
// 							ctx.SetPathf(
// 								"%s%s[%d]",
// 								path, key, idx,
// 							)
// 							ctx.LogError(e.Error())
// 							return false
// 						}
// 						p.runnerMap[id] = ssh
// 					}
// 					group = append(group, id)
// 				}

// 				task.groupMap[key] = group
// 				return true
// 			}

// 			// prepare imports
// 			for key, it := range task.Imports {
// 				importConfig := make(map[string][]*Remote)

// 				if absPath, e := filepath.Abs(
// 					filepath.Join(path.Dir(absEntryPath), it.Config),
// 				); e != nil {
// 					ctx.SetPathf("tasks.%s.imports.%s.config", taskName, key)
// 					ctx.LogError(e.Error())
// 					return false
// 				} else if e := loadConfig(absPath, &importConfig); e != nil {
// 					ctx.SetPathf("tasks.%s.imports.%s.config", taskName, key)
// 					ctx.LogError(e.Error())
// 					return false
// 				} else if list, ok := importConfig[it.Name]; !ok {
// 					ctx.SetPathf("tasks.%s.imports.%s.name", taskName, key)
// 					ctx.LogErrorf(
// 						"could not find \"%s\" in \"%s\"",
// 						it.Name, absPath,
// 					)
// 					return false
// 				} else {
// 					subCtx := ctx.Clone().SetConfig(absPath)
// 					if !fnPrepareRemoteList(subCtx, "", key, list) {
// 						return false
// 					}
// 				}
// 			}

// 			// prepare remotes
// 			for key, list := range task.Remotes {
// 				path := fmt.Sprintf("asks.%s.remotes.", key)
// 				if !fnPrepareRemoteList(ctx, path, key, list) {
// 					return false
// 				}
// 			}

// 			fmt.Println(task.groupMap)
// 			return true
// 		}

// 		// prepare all run jobs
// 		for _, taskName := range entryConfig.Run {
// 			if !fnPrepare(taskName) {
// 				return false
// 			}
// 		}

// 		return true
// 	}
// }

// func (p *Manager) runCommand(jobCtx *Context, index int, cmd *Command) bool {
// 	ctx := jobCtx.Clone().SetEnv(jobCtx.env.merge(cmd.Env))

// 	cmdType := cmd.Type
// 	if cmdType == "" {
// 		cmdType = "cmd"
// 	}

// 	env := jobCtx.env.merge(cmd.Env)

// 	runners := []CommandRunner{}
// 	if cmd.On == "" {
// 		runners = append(runners, defaultRunner)
// 	} else {
// 		cmdOn := env.parseString(cmd.On)
// 		for _, rawName := range strings.Split(cmdOn, ",") {
// 			if runnerName := strings.TrimSpace(rawName); runnerName != "" {
// 				v, e := p.getRunner(jobConfig, runnerName)
// 				if e != nil {
// 					LogError(head, e.Error())
// 					return false
// 				}
// 				runners = append(runners, v)
// 			}
// 		}

// 		if len(runners) == 0 {
// 			LogError(head, fmt.Sprintf(
// 				"could not find runner \"%s\" in config file \"%s\"",
// 				cmdOn, jobConfig,
// 			))
// 			return false
// 		}
// 	}

// 	if cmdType == "job" {
// 		cmdConfig := jobConfig
// 		if cmd.Config != "" {
// 			var e error
// 			cmdConfig, e = GetAbsConfigPathFrom(jobConfig, cmd.Config)
// 			if e != nil {
// 				LogError(head, fmt.Sprintf(
// 					"\"%s\" is invalid in config file \"%s\" error: %s",
// 					cmd.Config,
// 					jobConfig,
// 					e.Error(),
// 				))
// 				return false
// 			}
// 		}

// 		for _, runner := range runners {
// 			if !p.runJob(
// 				cmdConfig,
// 				env.parseString(cmd.Exec),
// 				jobEnv.parseEnv(cmd.Env),
// 				runner,
// 			) {
// 				return false
// 			}
// 		}

// 		return true
// 	} else if cmdType == "cmd" {
// 		for _, runner := range runners {
// 			if !runner.RunCommand(
// 				jobName,
// 				env.parseString(cmd.Exec),
// 				env.parseStringArray(cmd.Inputs),
// 			) {
// 				return false
// 			}
// 		}

// 		return true
// 	} else if cmdType == "js" {
// 		for _, runner := range runners {
// 			if !p.runScript(
// 				jobConfig,
// 				jobName,
// 				env.parseString(cmd.Exec),
// 				env,
// 				runner,
// 			) {
// 				return false
// 			}
// 		}

// 		return true
// 	} else {
// 		LogError(head, fmt.Sprintf("unknown command type %s", cmdType))
// 		return false
// 	}
// }

// func (p *Manager) runScript(
// 	jobConfig string,
// 	jobName string,
// 	script string,
// 	env Env,
// 	defaultRunner CommandRunner,
// ) bool {
// 	head := defaultRunner.Name() + " > " + jobName + ": "
// 	stdout := &bytes.Buffer{}
// 	stderr := &bytes.Buffer{}

// 	vm := otto.New()
// 	_ = vm.Set("dbot", &DbotObject{
// 		stdout:        stdout,
// 		stderr:        stderr,
// 		mgr:           p,
// 		jobConfig:     jobConfig,
// 		jobName:       jobName,
// 		scriptEnv:     env,
// 		defaultRunner: defaultRunner,
// 	})
// 	_, e := vm.Run(script)
// 	errStr := getStandradOut(stderr.String())
// 	if e != nil {
// 		errStr += getStandradOut(e.Error())
// 	}

// 	LogScript(head, script, stdout.String(), errStr)

// 	return e == nil
// }

// func (p *Manager) runCommandAA(jobCtx *Context, job *Job, cmd *Command, index int) bool {
// 	tmpCtx := jobCtx.Clone().
// 		SetPathf("%s.commands[%d]", jobCtx.path, index).
// 		SetEnv(rootEnv.merge(Env{
// 			"ConfigDir": filepath.Dir(jobCtx.config),
// 		}).merge(job.Env).merge(jobCtx.env).merge(cmd.Env)).
// 		SetCommand(nil)

// 	var e error
// 	var runners []CommandRunner
// 	cmdOn := strings.TrimSpace(tmpCtx.env.parseString(cmd.On))
// 	if cmdOn == "" {
// 		runners = []CommandRunner{tmpCtx.runner}
// 	} else {
// 		runners, e = tmpCtx.GetRunners(cmdOn)
// 		if e != nil {
// 			tmpCtx.LogError(e.Error())
// 			return false
// 		}
// 	}

// 	cmdType := cmd.Type
// 	if cmdType == "" {
// 		cmdType = "cmd"
// 	}

// 	return false
// 	// switch cmdType {
// 	// case "job":
// 	// 	cmdConfig := tmpCtx.config
// 	// 	if cmd.Config != "" {
// 	// 		cmdConfig = strings.TrimSpace(tmpCtx.ParseValue(cmd.Config))
// 	// 		absPath, e := filepath.Abs(
// 	// 			filepath.Join(path.Dir(absEntryPath), cmdConfig),
// 	// 		)
// 	// 		if ; e != nil {
// 	// 			tmpCtx.LogError(e.Error())
// 	// 			return false
// 	// 		}

// 	// 		cmdConfig, e = (jobConfig, cmd.Config)
// 	// 		if e != nil {
// 	// 			LogError(head, fmt.Sprintf(
// 	// 				"\"%s\" is invalid in config file \"%s\" error: %s",
// 	// 				cmd.Config,
// 	// 				jobConfig,
// 	// 				e.Error(),
// 	// 			))
// 	// 			return false
// 	// 		}
// 	// 	}

// 	// 	for _, runner := range runners {
// 	// 		if !p.runJob(
// 	// 			cmdConfig,
// 	// 			env.parseString(cmd.Exec),
// 	// 			jobEnv.parseEnv(cmd.Env),
// 	// 			runner,
// 	// 		) {
// 	// 			return false
// 	// 		}
// 	// 	}

// 	// 	return true
// 	// }
// 	// if cmdType == "job" {
// 	// 	cmdConfig := jobConfig
// 	// 	if cmd.Config != "" {
// 	// 		var e error
// 	// 		cmdConfig, e = GetAbsConfigPathFrom(jobConfig, cmd.Config)
// 	// 		if e != nil {
// 	// 			LogError(head, fmt.Sprintf(
// 	// 				"\"%s\" is invalid in config file \"%s\" error: %s",
// 	// 				cmd.Config,
// 	// 				jobConfig,
// 	// 				e.Error(),
// 	// 			))
// 	// 			return false
// 	// 		}
// 	// 	}

// 	// 	for _, runner := range runners {
// 	// 		if !p.runJob(
// 	// 			cmdConfig,
// 	// 			env.parseString(cmd.Exec),
// 	// 			jobEnv.parseEnv(cmd.Env),
// 	// 			runner,
// 	// 		) {
// 	// 			return false
// 	// 		}
// 	// 	}

// 	// 	return true
// 	// } else if cmdType == "cmd" {
// 	// 	for _, runner := range runners {
// 	// 		if !runner.RunCommand(
// 	// 			jobName,
// 	// 			env.parseString(cmd.Exec),
// 	// 			env.parseStringArray(cmd.Inputs),
// 	// 		) {
// 	// 			return false
// 	// 		}
// 	// 	}

// 	// 	return true
// 	// } else if cmdType == "js" {
// 	// 	for _, runner := range runners {
// 	// 		if !p.runScript(
// 	// 			jobConfig,
// 	// 			jobName,
// 	// 			env.parseString(cmd.Exec),
// 	// 			env,
// 	// 			runner,
// 	// 		) {
// 	// 			return false
// 	// 		}
// 	// 	}

// 	// 	return true
// 	// } else {
// 	// 	LogError(head, fmt.Sprintf("unknown command type %s", cmdType))
// 	// 	return false
// 	// }

// }

// func (p *Manager) runJob(ctx *Context) (ret bool) {
// 	jobName := ctx.cmd.Exec
// 	ctx.LogInfof("Job \"%s > %s\" Start", ctx.config, jobName)
// 	defer func() {
// 		if ret {
// 			ctx.LogInfof("Job \"%s > %s\" successful", ctx.config, jobName)
// 		} else {
// 			ctx.LogErrorf("Job \"%s > %s\" failed", ctx.config, jobName)
// 		}
// 	}()

// 	job, e := p.getJob(ctx.config, jobName)
// 	if e != nil {
// 		ctx.LogError(e.Error())
// 		return false
// 	}

// 	// update env
// 	ctx.SetEnv(
// 		rootEnv.merge(Env{
// 			"ConfigDir": filepath.Dir(ctx.config),
// 		}).merge(job.Env).merge(ctx.env),
// 	)

// 	concurrency := job.Concurrency
// 	commands := job.Commands

// 	if !concurrency {
// 		for i := 0; i < len(commands); i++ {
// 			command := commands[i]
// 			if !p.runCommand(ctx, command) {
// 				return false
// 			}
// 		}

// 		return true
// 	} else {
// 		retCH := make(chan bool, len(commands))

// 		for i := 0; i < len(commands); i++ {
// 			go func(command Command) {
// 				retCH <- p.runCommand(
// 					ctx.config,
// 					ctx.exec,
// 					env,
// 					command,
// 					ctx.runner,
// 				)
// 			}(commands[i])
// 		}

// 		ret := true
// 		for i := 0; i < len(commands); i++ {
// 			if !<-retCH {
// 				ret = false
// 			}
// 		}

// 		return ret
// 	}
// }
