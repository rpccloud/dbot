package main

import (
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

type TerminalStdin struct {
	delay  time.Duration
	reader io.Reader
	inputs []string
	stdin  io.Reader

	sync.Mutex
}

func NewTerminalStdin(inputs []string, stdin io.Reader) *TerminalStdin {
	return &TerminalStdin{
		delay:  time.Second,
		reader: nil,
		inputs: inputs,
		stdin:  stdin,
	}
}

func (p *TerminalStdin) Read(b []byte) (n int, err error) {
	p.Lock()
	defer p.Unlock()

	time.Sleep(p.delay)

	for {
		if p.reader == nil {
			if len(p.inputs) > 0 {
				p.reader = strings.NewReader(p.inputs[0])
				p.inputs = p.inputs[1:]
				p.delay = 500 * time.Millisecond
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

func main() {
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("World2019")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", "192.168.1.81:22", config)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()

	inputs := []string{
		"vim test.sh\n",
		"i", "echo \"hello world\"\n",
		"\033", ":wq\n",
		"exit\n",
	}

	fileDescriptor := int(os.Stdin.Fd())
	if term.IsTerminal(fileDescriptor) {
		originalState, err := term.MakeRaw(fileDescriptor)
		if err != nil {
			log.Fatal(err)

		}
		defer func() {
			_ = term.Restore(fileDescriptor, originalState)
		}()

		err = session.RequestPty("xterm-256color", 0, 0, ssh.TerminalModes{
			ssh.ECHO:          1,     // enable echoing
			ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
			ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
		})

		if err != nil {
			log.Fatal(err)
		}
	}

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = NewTerminalStdin(inputs, os.Stdin)

	err = session.Shell()
	if err != nil {
		log.Fatal(err)
	}

	// You should now be connected via SSH with a fully-interactive terminal
	// This call blocks until the user exits the session (e.g. via CTRL + D)
	_ = session.Wait()
}
