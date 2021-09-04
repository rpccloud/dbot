package dbot

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSHAuth int

const (
	SSHAuthIdle SSHAuth = iota
	SSHAuthFixedPassword
	SSHAuthInputPassword
	SSHAuthPublicKey
	SSHAuthPublicKeyRSA
	SSHAuthPublicKeyDSA
	SSHAuthPublicKeyECDSA
	SSHAuthPublicKeyED25519
	SSHAuthPublicKeyXMSS
)

type RunnerInput struct {
	delay  time.Duration
	reader io.Reader
	inputs []string
	stdin  io.Reader

	sync.Mutex
}

func NewRunnerInput(inputs []string, stdin io.Reader) *RunnerInput {
	return &RunnerInput{
		delay:  time.Second,
		reader: nil,
		inputs: inputs,
		stdin:  stdin,
	}
}

func (p *RunnerInput) Read(b []byte) (n int, err error) {
	p.Lock()
	defer p.Unlock()

	time.Sleep(p.delay)

	for {
		if p.reader == nil {
			if len(p.inputs) > 0 {
				p.reader = strings.NewReader(p.inputs[0])
				p.inputs = p.inputs[1:]
				p.delay = 400 * time.Millisecond
			} else {
				p.reader = p.stdin
				p.stdin = nil
				p.delay = 0
			}
		}

		if p.reader == nil {
			return 0, io.EOF
		}

		if n, e := p.reader.Read(b); e != io.EOF {
			return n, e
		}

		p.reader = nil
	}
}

type Runner interface {
	Name() string
	Run(ctx *XContext) bool
	Prepare(ctx *XContext) bool
}

type MainRunner struct {
	taskContexts []*XContext
}

func (p *MainRunner) Name() string {
	return fmt.Sprintf("%s@local", os.Getenv("USER"))
}

func (p *MainRunner) Run(ctx *XContext) bool {
	if len(p.taskContexts) <= 0 {
		ctx.LogError("there are no tasks to run")
		return false
	}

	for _, ctx := range p.taskContexts {
		if !ctx.Run() {
			return false
		}
	}

	return true
}

func (p *MainRunner) Prepare(ctx *XContext) bool {
	config := MainConfig{}

	if ok := ctx.loadConfig(ctx.cmd.Config, &config); !ok {
		return false
	}

	for _, taskName := range config.Main {
		taskCtx := ctx.CreateTaskContext(taskName, nil)

		if taskCtx == nil {
			return false
		}

		if !taskCtx.Prepare() {
			return false
		}

		p.taskContexts = append(p.taskContexts, taskCtx)
	}

	return true
}

type TaskRunner struct {
	// jobName   string
	// jobConfig string
}

func (p *TaskRunner) Name() string {
	return fmt.Sprintf("%s@local", os.Getenv("USER"))
}

func (p *TaskRunner) prepareRemoteList(
	ctx *XContext,
	key string,
	list []*Remote,
) bool {
	group := make([]string, 0)

	for idx, it := range list {
		host := ctx.GetContextEnv().ParseString(it.Host, "", true)
		user := ctx.GetContextEnv().ParseString(it.User, "", true)
		port := ctx.GetContextEnv().ParseString(it.Port, "22", true)

		id := fmt.Sprintf("%s@%s:%s", user, host, port)
		if gXManager.GetRunner(id) == nil {
			ssh, e := NewSSHRunner(
				port,
				user,
				host,
			)
			if e != nil {
				ctx.Clone().
					SetCurrentf("%s[%d]", ctx.current, idx).
					LogError(e.Error())
				return false
			}
			gXManager.SetRunner(id, ssh)
		}
		group = append(group, id)
	}

	ctx.task.groupMap[key] = group
	return true
}

func (p *TaskRunner) Prepare(ctx *XContext) bool {
	config := MainConfig{}

	if ok := ctx.loadConfig(ctx.cmd.Config, &config); !ok {
		return false
	}

	taskName := ctx.cmd.Exec
	task, ok := config.Tasks[taskName]
	if !ok {
		ctx.LogErrorf("task \"%s\" not found", taskName)
		return false
	}

	ctx.SetTask(task)
	// prepare env
	commandEnv := ctx.RootEnv().ParseEnv(task.Env)
	contextEnv := ctx.RootEnv().Merge(commandEnv)
	for key, it := range task.Inputs {
		ctx.Clone().
			SetCurrentf("%s.inputs.%s", ctx.current, key).
			LogInfo("")
		itDesc := contextEnv.ParseString(it.Desc, "input "+key+": ", false)
		itType := contextEnv.ParseString(it.Type, "text", true)
		value, ok := ctx.GetUserInput(itDesc, itType)
		if !ok {
			return false
		}
		commandEnv[key] = contextEnv.ParseString(value, "", false)
	}
	contextEnv = ctx.RootEnv().Merge(commandEnv)
	ctx.SetContextEnv(contextEnv)
	ctx.SetCommandEnv(commandEnv)

	// prepare imports
	task.groupMap = make(map[string][]string)
	for key, it := range task.Imports {
		importName := contextEnv.ParseString(it.Name, "", true)
		improtConfigPath, ok := ctx.Clone().
			SetCurrentf("%s.imports.%s.config", ctx.current, key).
			AbsPath(contextEnv.ParseString(it.Config, "", true))
		if !ok {
			return false
		}
		importConfig := ctx.Clone().
			SetCurrentf("%s.imports.%s.config", ctx.current, key).
			LoadRemoteConfig(improtConfigPath)
		if importConfig == nil {
			return false
		}

		list, ok := importConfig[importName]
		if !ok {
			ctx.Clone().
				SetCurrentf("%s.imports.%s.name", ctx.current, key).
				LogErrorf(
					"could not find \"%s\" in \"%s\"",
					it.Name, improtConfigPath,
				)
			return false
		}
		if !p.prepareRemoteList(
			ctx.CreateImportContext(
				importName,
				improtConfigPath,
				contextEnv.ParseEnv(it.Env),
			),
			key,
			list,
		) {
			return false
		}
	}

	// prepare remotes
	for key, list := range task.Remotes {
		if !p.prepareRemoteList(
			ctx.Clone().SetCurrentf(fmt.Sprintf("tasks.%s.remotes", key)),
			key,
			list,
		) {
			return false
		}
	}

	return true
}

func (p *TaskRunner) Run(ctx *XContext) bool {
	// configPath, ok := ctx.Clone().
	// 	SetCurrentf("%s.config", ctx.current).
	// 	AbsPath(ctx.GetContextEnv().ParseString(ctx.task.Config, "", true))

	// if !ok {
	// 	return false
	// }

	// run := ctx.GetContextEnv().ParseString(ctx.task.Run, "", true)

	// ctx.CreateLoa

	return false
}

type LocalRunner struct {
	sync.Mutex
}

func (p *LocalRunner) Name() string {
	return fmt.Sprintf("%s@local", os.Getenv("USER"))
}

func (p *LocalRunner) Prepare(ctx *XContext) bool {
	return true
}

func (p *LocalRunner) Run(ctx *XContext) bool {
	return false
}

func (p *LocalRunner) RunCommand(
	jobName string,
	command string,
	inputs []string,
) bool {
	p.Lock()
	defer p.Unlock()

	head := p.Name() + " > " + jobName + ": "
	cmdArray := ParseCommand(command)

	var cmd *exec.Cmd
	if len(cmdArray) == 1 {
		cmd = exec.Command(cmdArray[0])
	} else if len(cmdArray) > 1 {
		cmd = exec.Command(cmdArray[0], cmdArray[1:]...)
	} else {
		LogCommand(head, command, "", "the command is empty")
		return false
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdin = NewRunnerInput(inputs, nil)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	e := cmd.Run()

	outString := ""
	errString := ""
	if s := getStandradOut(stdout.String()); !FilterString(s, outFilter) {
		outString += s
	}

	if s := getStandradOut(stderr.String()); !FilterString(s, errFilter) {
		errString += s
	}

	if e != nil {
		errString += getStandradOut(e.Error())
	}

	LogCommand(head, command, outString, errString)
	return e == nil
}

type SSHRunner struct {
	port     string
	user     string
	host     string
	password string

	sync.Mutex
}

func NewSSHRunner(
	port string,
	user string,
	host string,
) (*SSHRunner, error) {
	ret := &SSHRunner{
		port:     port,
		user:     user,
		host:     host,
		password: "",
	}

	client, e := ret.getClient(SSHAuthIdle)
	if e != nil {
		return nil, e
	}
	_ = client.Close()

	if ret.password == "" {
		LogInput(fmt.Sprintf("Use PublicKey on %s@%s\n", ret.user, ret.host))
	}

	return ret, nil
}

func (p *SSHRunner) Prepare(ctx *XContext) bool {
	return true
}

func (p *SSHRunner) Run(ctx *XContext) bool {
	return false
}

func (p *SSHRunner) Name() string {
	return fmt.Sprintf("%s@%s", p.user, p.host)
}

func (p *SSHRunner) RunCommand(
	jobName string,
	command string,
	inputs []string,
) bool {
	p.Lock()
	defer p.Unlock()

	head := p.Name() + " > " + jobName + ": "

	if client, e := p.getClient(SSHAuthIdle); e != nil {
		LogError(head, e.Error())
		return false
	} else if session, e := client.NewSession(); e != nil {
		_ = client.Close()
		LogError(head, e.Error())
		return false
	} else {
		defer func() {
			_ = session.Close()
			_ = client.Close()
		}()

		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		session.Stdin = NewRunnerInput(inputs, nil)
		session.Stdout = stdout
		session.Stderr = stderr
		e = session.Run(command)
		outString := ""
		errString := ""
		if s := getStandradOut(stdout.String()); !FilterString(s, outFilter) {
			outString += s
		}

		if s := getStandradOut(stderr.String()); !FilterString(s, errFilter) {
			errString += s
		}

		if e != nil {
			errString += getStandradOut(e.Error())
		}

		LogCommand(head, command, outString, errString)
		return e == nil
	}
}

func (p *SSHRunner) getClient(auth SSHAuth) (client *ssh.Client, ret error) {
	fnGetPublicKeyConfig := func(fileName string) (*ssh.ClientConfig, error) {
		keyPath := filepath.Join(os.Getenv("HOME"), ".ssh", fileName)
		if key, e := ioutil.ReadFile(keyPath); e != nil {
			return nil, fmt.Errorf("ssh: read private key: %s", e.Error())
		} else if signer, e := ssh.ParsePrivateKey(key); e != nil {
			return nil, fmt.Errorf("ssh: parse private key: %s", e.Error())
		} else {
			return &ssh.ClientConfig{
				User:            p.user,
				Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			}, nil
		}
	}

	fnGetPassworldConfig := func(password string) *ssh.ClientConfig {
		return &ssh.ClientConfig{
			User:            p.user,
			Auth:            []ssh.AuthMethod{ssh.Password(p.password)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	}

	var clientCfg *ssh.ClientConfig

	switch auth {
	case SSHAuthIdle:
		if p.password != "" {
			return p.getClient(SSHAuthFixedPassword)
		}

		if client, ret = p.getClient(SSHAuthPublicKey); ret == nil {
			return
		}

		return p.getClient(SSHAuthInputPassword)
	case SSHAuthFixedPassword:
		clientCfg = fnGetPassworldConfig(p.password)
	case SSHAuthInputPassword:
		if p.password, ret = GetPasswordFromUser(fmt.Sprintf(
			"Password for ssh -p %s %s@%s: ",
			p.port, p.user, p.host),
		); ret != nil {
			return
		}
		clientCfg = fnGetPassworldConfig(p.password)
	case SSHAuthPublicKey:
		for _, keyAuth := range []SSHAuth{
			SSHAuthPublicKeyRSA, SSHAuthPublicKeyDSA, SSHAuthPublicKeyECDSA,
			SSHAuthPublicKeyED25519, SSHAuthPublicKeyXMSS,
		} {
			if client, ret = p.getClient(keyAuth); ret == nil {
				return
			}
		}

		return nil, fmt.Errorf("ssh: connect by publicKey failed")
	case SSHAuthPublicKeyRSA:
		if clientCfg, ret = fnGetPublicKeyConfig("id_rsa"); ret != nil {
			return
		}
	case SSHAuthPublicKeyDSA:
		if clientCfg, ret = fnGetPublicKeyConfig("id_dsa"); ret != nil {
			return
		}
	case SSHAuthPublicKeyECDSA:
		if clientCfg, ret = fnGetPublicKeyConfig("id_ecdsa"); ret != nil {
			return
		}
	case SSHAuthPublicKeyED25519:
		if clientCfg, ret = fnGetPublicKeyConfig("id_ed25519"); ret != nil {
			return
		}
	case SSHAuthPublicKeyXMSS:
		if clientCfg, ret = fnGetPublicKeyConfig("id_xmss"); ret != nil {
			return
		}
	}

	return ssh.Dial("tcp", fmt.Sprintf("%s:%s", p.host, p.port), clientCfg)
}
