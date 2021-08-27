package dbot

import (
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

type CommandRunner interface {
	RunCommand(
		jobName string,
		command string,
		inputs []string,
		logCH chan *logRecord,
	) error
}

type LocalRunner struct{}

func (p *LocalRunner) RunCommand(
	jobName string,
	command string,
	inputs []string,
	logCH chan *logRecord,
) (ret error) {
	var stdout io.ReadCloser
	var stderr io.ReadCloser

	cmdArray := strings.Fields(command)
	cmd := exec.Command(cmdArray[0], cmdArray[1:]...)

	if stdout, ret = cmd.StdoutPipe(); ret != nil {
		return
	} else if stderr, ret = cmd.StderrPipe(); ret != nil {
		_ = stdout.Close()
		return
	} else {
		retCH := make(chan error, 2)

		go func() {
			str, e := ReadStringFromIOReader(stdout)
			_ = stdout.Close()
			retCH <- e
			if str != "" {
				logCH <- newLogRecordInfo("local", jobName, "Out: "+str)
			}
		}()

		go func() {
			str, e := ReadStringFromIOReader(stderr)
			_ = stderr.Close()
			retCH <- e
			if str != "" {
				logCH <- newLogRecordError("local", jobName, "Error: "+str)
			}
		}()

		cmd.Stdin = NewRunnerInput(inputs, nil)
		ret = cmd.Run()

		// wait for all goroutines
		for i := 0; i < 2; i++ {
			if e := <-retCH; e != nil && ret == nil {
				ret = e
			}
		}

		return ret
	}
}

type SSHRunner struct {
	name     string
	port     uint16
	user     string
	host     string
	password string
}

func NewSSHRunner(
	name string, port uint16, user string, host string,
) (*SSHRunner, error) {
	if port == 0 {
		port = 22
	}

	if user == "" {
		user = os.Getenv("USER")
	}

	ret := &SSHRunner{
		name:     name,
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
		_, _ = authColor.Printf("Use PublicKey on %s@%s\n", ret.user, ret.host)
	}

	return ret, nil
}

func (p *SSHRunner) RunCommand(
	jobName string,
	command string,
	inputs []string,
	logCH chan *logRecord,
) (ret error) {
	var client *ssh.Client
	var session *ssh.Session

	defer func() {
		if session != nil {
			if e := session.Close(); e != nil && e != io.EOF && ret == nil {
				ret = e
			}
		}

		if client != nil {
			if e := client.Close(); e != nil && e != io.EOF && ret == nil {
				ret = e
			}
		}
	}()

	if client, ret = p.getClient(SSHAuthIdle); ret != nil {
		return
	} else if session, ret = client.NewSession(); ret != nil {
		return
	} else {
		var stdout io.Reader
		var stderr io.Reader

		if stdout, ret = session.StdoutPipe(); ret != nil {
			return
		} else if stderr, ret = session.StderrPipe(); ret != nil {
			return
		} else {
			retCH := make(chan error, 2)

			go func() {
				str, e := ReadStringFromIOReader(stdout)
				retCH <- e
				if str != "" {
					logCH <- newLogRecordInfo(p.name, jobName, "Out: "+str)
				}
			}()

			go func() {
				str, e := ReadStringFromIOReader(stderr)
				retCH <- e
				if str != "" {
					logCH <- newLogRecordError(p.name, jobName, "Error: "+str)
				}
			}()

			session.Stdin = NewRunnerInput(inputs, nil)
			ret = session.Run(command)
			// wait for all goroutines
			for i := 0; i < 2; i++ {
				if e := <-retCH; e != nil && ret == nil {
					ret = e
				}
			}
			return ret
		}
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
		if p.password, ret = GetPasswordFromUser(
			fmt.Sprintf("Password for %s@%s: ", p.user, p.host),
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

	return ssh.Dial("tcp", fmt.Sprintf("%s:%d", p.host, p.port), clientCfg)
}
