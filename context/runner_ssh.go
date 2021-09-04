package context

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/ssh"
)

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

func (p *SSHRunner) Name() string {
	return fmt.Sprintf("%s@%s:%s", p.user, p.host, p.port)
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

	fnClient := func(c Context, cfg *ssh.ClientConfig, log bool) *ssh.Client {
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
