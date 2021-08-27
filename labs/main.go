package main

import (
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

type TerminalStdin struct {
	reader io.Reader
	inputs []string
	stdin  io.Reader

	sync.Mutex
}

func NewTerminalStdin(inputs []string, stdin io.Reader) *TerminalStdin {
	return &TerminalStdin{
		reader: nil,
		inputs: inputs,
		stdin:  stdin,
	}
}

func (p *TerminalStdin) Read(b []byte) (n int, err error) {
	p.Lock()
	defer p.Unlock()

	time.Sleep(500 * time.Millisecond)

	for {
		if p.reader == nil {
			if len(p.inputs) > 0 {
				p.reader = strings.NewReader(p.inputs[0])
				p.inputs = p.inputs[1:]
			} else {
				p.reader = p.stdin
				p.stdin = nil
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

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	sessionStdin, _ := session.StdinPipe()

	inputs := []string{
		"apt-get update\n",
		"vim test.sh \n",
		"i", "echo \"hello world\"\n",
		"\033", ":wq\n",
		"exit\n",
	}

	go func() {
		time.Sleep(time.Second)
		for i := 0; i < len(inputs); i++ {
			time.Sleep(time.Second)
			_, _ = io.Copy(sessionStdin, strings.NewReader(inputs[i]))
		}
		_, _ = io.Copy(sessionStdin, os.Stdin)
	}()

	fileDescriptor := int(os.Stdin.Fd())

	if terminal.IsTerminal(fileDescriptor) {
		originalState, err := terminal.MakeRaw(fileDescriptor)
		if err != nil {
			log.Fatal(err)

		}
		defer terminal.Restore(fileDescriptor, originalState)

		termWidth, termHeight, err := terminal.GetSize(fileDescriptor)
		if err != nil {
			log.Fatal(err)
		}

		err = session.RequestPty("xterm-256color", termHeight, termWidth, ssh.TerminalModes{
			ssh.ECHO:          1,     // enable echoing
			ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
			ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
		})

		if err != nil {
			log.Fatal(err)
		}
	}

	err = session.Shell()
	if err != nil {
		log.Fatal(err)
	}

	// You should now be connected via SSH with a fully-interactive terminal
	// This call blocks until the user exits the session (e.g. via CTRL + D)
	session.Wait()
}
