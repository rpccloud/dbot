package context

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
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
	Run(ctx Context) bool
}

type LocalRunner struct {
	sync.Mutex
}

func (p *LocalRunner) Run(ctx Context) bool {
	return false
}

// func (p *LocalRunner) RunCommand(
// 	jobName string,
// 	command string,
// 	inputs []string,
// ) bool {
// 	p.Lock()
// 	defer p.Unlock()

// 	head := p.Name() + " > " + jobName + ": "
// 	cmdArray := ParseCommand(command)

// 	var cmd *exec.Cmd
// 	if len(cmdArray) == 1 {
// 		cmd = exec.Command(cmdArray[0])
// 	} else if len(cmdArray) > 1 {
// 		cmd = exec.Command(cmdArray[0], cmdArray[1:]...)
// 	} else {
// 		LogCommand(head, command, "", "the command is empty")
// 		return false
// 	}

// 	stdout := &bytes.Buffer{}
// 	stderr := &bytes.Buffer{}
// 	cmd.Stdin = NewRunnerInput(inputs, nil)
// 	cmd.Stdout = stdout
// 	cmd.Stderr = stderr
// 	e := cmd.Run()

// 	outString := ""
// 	errString := ""
// 	if s := getStandradOut(stdout.String()); !FilterString(s, outFilter) {
// 		outString += s
// 	}

// 	if s := getStandradOut(stderr.String()); !FilterString(s, errFilter) {
// 		errString += s
// 	}

// 	if e != nil {
// 		errString += getStandradOut(e.Error())
// 	}

// 	LogCommand(head, command, outString, errString)
// 	return e == nil
// }

type SSHRunner struct {
	port     string
	user     string
	host     string
	password string
	key      string

	sync.Mutex
}

func NewSSHRunner(
	ctx Context,
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

func (p *SSHRunner) Run(ctx Context) bool {
	return false
}

// func (p *SSHRunner) RunCommand(
// 	jobName string,
// 	command string,
// 	inputs []string,
// ) bool {
// 	p.Lock()
// 	defer p.Unlock()

// 	head := p.Name() + " > " + jobName + ": "

// 	if client, e := p.getClient(SSHAuthIdle); e != nil {
// 		LogError(head, e.Error())
// 		return false
// 	} else if session, e := client.NewSession(); e != nil {
// 		_ = client.Close()
// 		LogError(head, e.Error())
// 		return false
// 	} else {
// 		defer func() {
// 			_ = session.Close()
// 			_ = client.Close()
// 		}()

// 		stdout := &bytes.Buffer{}
// 		stderr := &bytes.Buffer{}
// 		session.Stdin = NewRunnerInput(inputs, nil)
// 		session.Stdout = stdout
// 		session.Stderr = stderr
// 		e = session.Run(command)
// 		outString := ""
// 		errString := ""
// 		if s := getStandradOut(stdout.String()); !FilterString(s, outFilter) {
// 			outString += s
// 		}

// 		if s := getStandradOut(stderr.String()); !FilterString(s, errFilter) {
// 			errString += s
// 		}

// 		if e != nil {
// 			errString += getStandradOut(e.Error())
// 		}

// 		LogCommand(head, command, outString, errString)
// 		return e == nil
// 	}
// }

func (p *SSHRunner) getClient(ctx Context) *ssh.Client {
	fnGetPassworldConfig := func(password string) *ssh.ClientConfig {
		return &ssh.ClientConfig{
			User:            p.user,
			Auth:            []ssh.AuthMethod{ssh.Password(password)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	}

	fnParseKeyConfig := func(
		ctx Context,
		fileBytes []byte,
		log bool,
	) *ssh.ClientConfig {
		signer, e := ssh.ParsePrivateKey(fileBytes)

		if e != nil {
			if log {
				ctx.LogErrorf(e.Error())
			}
			return nil
		}

		return &ssh.ClientConfig{
			User:            p.user,
			Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	}

	fnClient := func(c Context, cfg *ssh.ClientConfig, log bool) *ssh.Client {
		ret, e := ssh.Dial("tcp", fmt.Sprintf("%s:%s", p.host, p.port), cfg)
		if e != nil {
			if log {
				c.LogErrorf(e.Error())
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
