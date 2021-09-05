package context

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

var outFilter = []string{
	"\033",
}

var errFilter = []string{
	"Output is not to a terminal",
	"Input is not from a terminal",
}

func filterString(str string, filter []string) bool {
	for _, v := range filter {
		if strings.Contains(str, v) {
			return true
		}
	}

	return false
}

func reportRunnerResult(
	ctx *CmdContext, e error, out *bytes.Buffer, err *bytes.Buffer,
) (canContinue bool) {
	outString := ""
	errString := ""
	if s := out.String(); !filterString(s, outFilter) {
		outString += s
	}

	if s := err.String(); !filterString(s, errFilter) {
		errString += s
	}

	ctx.Log(outString, errString)

	if e != nil {
		ctx.LogError(e.Error())
		return false
	}

	return true
}

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
	Run(ctx *CmdContext) bool
}

type LocalRunner struct {
	sync.Mutex
}

func (p *LocalRunner) Name() string {
	return fmt.Sprintf("%s@local", os.Getenv("USER"))
}

func (p *LocalRunner) Run(ctx *CmdContext) bool {
	p.Lock()
	defer p.Unlock()

	// Parse command
	cmd := ctx.ParseCommand()
	if cmd == nil {
		return false
	}

	// Split command and check
	cmdArray := SplitCommand(cmd.Exec)
	if len(cmdArray) == 1 {
		ctx.LogError("the command is empty")
		return false
	}

	// Make exec command
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	execCommand := exec.Command(cmdArray[0], cmdArray[1:]...)
	execCommand.Stdin = NewRunnerInput(cmd.Inputs, nil)
	execCommand.Stdout = stdout
	execCommand.Stderr = stderr

	return reportRunnerResult(ctx, execCommand.Run(), stdout, stderr)
}

type SSHRunner struct {
	port     string
	user     string
	host     string
	password string
	key      string

	sync.Mutex
}

func NewSSHRunner(
	ctx *CmdContext,
	port string,
	user string,
	host string,
) *SSHRunner {
	ret := &SSHRunner{
		port:     port,
		user:     user,
		host:     host,
		password: "",
	}

	// Check if ssh can connect
	client := ret.getClient(ctx)
	if client == nil {
		return nil
	}
	_ = client.Close()

	return ret
}

func (p *SSHRunner) Name() string {
	return fmt.Sprintf("%s@%s:%s", p.user, p.host, p.port)
}

func (p *SSHRunner) Run(ctx *CmdContext) bool {
	p.Lock()
	defer p.Unlock()

	// Parse command
	cmd := ctx.ParseCommand()
	if cmd == nil {
		return false
	}

	if client := p.getClient(ctx); client == nil {
		return false
	} else if session, e := client.NewSession(); e != nil {
		_ = client.Close()
		ctx.LogError(e.Error())
		return false
	} else {
		// Make exec command
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		session.Stdin = NewRunnerInput(cmd.Inputs, nil)
		session.Stdout = stdout
		session.Stderr = stderr

		return reportRunnerResult(ctx, session.Run(cmd.Exec), stdout, stderr)
	}
}

func (p *SSHRunner) getClient(ctx *CmdContext) *ssh.Client {
	fnGetPassworldConfig := func(password string) *ssh.ClientConfig {
		return &ssh.ClientConfig{
			User:            p.user,
			Auth:            []ssh.AuthMethod{ssh.Password(password)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	}

	fnParseKeyConfig := func(
		ctx *CmdContext,
		fileBytes []byte,
		log bool,
	) *ssh.ClientConfig {
		signer, e := ssh.ParsePrivateKey(fileBytes)

		if e != nil {
			if log {
				ctx.LogError(e.Error())
			}
			return nil
		}

		return &ssh.ClientConfig{
			User:            p.user,
			Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	}

	fnClient := func(c *CmdContext, cfg *ssh.ClientConfig, log bool) *ssh.Client {
		ret, e := ssh.Dial("tcp", fmt.Sprintf("%s:%s", p.host, p.port), cfg)
		if e != nil {
			if log {
				c.LogError(e.Error())
			}
			return nil
		}
		return ret
	}

	if p.key != "" {
		config := fnParseKeyConfig(ctx, []byte(p.key), true)
		return fnClient(ctx, config, true)
	} else if p.password != "" {
		config := fnGetPassworldConfig(p.password)
		return fnClient(ctx, config, true)
	} else {
		// Try to load from ssh key
		for _, loc := range []string{
			"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519", "id_xmss",
		} {
			keyPath := filepath.Join(os.Getenv("HOME"), ".ssh", loc)

			fileBytes, e := ioutil.ReadFile(keyPath)
			if e != nil {
				continue
			}

			config := fnParseKeyConfig(ctx, fileBytes, false)
			if ret := fnClient(ctx, config, false); ret != nil {
				return ret
			}
		}

		// No password is set and there is no valid ssh key.
		// So we need to enter the password.
		desc := fmt.Sprintf(
			"password for ssh -p %s %s@%s: ",
			p.port, p.user, p.host,
		)
		password, ok := ctx.GetUserInput(desc, "password")
		if !ok {
			return nil
		}
		config := fnGetPassworldConfig(password)
		if ret := fnClient(ctx, config, true); ret != nil {
			p.password = password
			return ret
		}

		return nil
	}
}
