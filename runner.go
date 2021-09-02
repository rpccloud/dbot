package dbot

// type SSHAuth int

// const (
// 	SSHAuthIdle SSHAuth = iota
// 	SSHAuthFixedPassword
// 	SSHAuthInputPassword
// 	SSHAuthPublicKey
// 	SSHAuthPublicKeyRSA
// 	SSHAuthPublicKeyDSA
// 	SSHAuthPublicKeyECDSA
// 	SSHAuthPublicKeyED25519
// 	SSHAuthPublicKeyXMSS
// )

// type RunnerInput struct {
// 	delay  time.Duration
// 	reader io.Reader
// 	inputs []string
// 	stdin  io.Reader

// 	sync.Mutex
// }

// func NewRunnerInput(inputs []string, stdin io.Reader) *RunnerInput {
// 	return &RunnerInput{
// 		delay:  time.Second,
// 		reader: nil,
// 		inputs: inputs,
// 		stdin:  stdin,
// 	}
// }

// func (p *RunnerInput) Read(b []byte) (n int, err error) {
// 	p.Lock()
// 	defer p.Unlock()

// 	time.Sleep(p.delay)

// 	for {
// 		if p.reader == nil {
// 			if len(p.inputs) > 0 {
// 				p.reader = strings.NewReader(p.inputs[0])
// 				p.inputs = p.inputs[1:]
// 				p.delay = 400 * time.Millisecond
// 			} else {
// 				p.reader = p.stdin
// 				p.stdin = nil
// 				p.delay = 0
// 			}
// 		}

// 		if p.reader == nil {
// 			return 0, io.EOF
// 		}

// 		if n, e := p.reader.Read(b); e != io.EOF {
// 			return n, e
// 		}

// 		p.reader = nil
// 	}
// }

// type CommandRunner interface {
// 	Name() string
// 	RunCommand(
// 		jobName string,
// 		command string,
// 		inputs []string,
// 	) bool
// }

// type DbotRunner struct {
// }

// func (p *DbotRunner) Name() string {
// 	return fmt.Sprintf("%s@dbot", os.Getenv("USER"))
// }

// func (p *DbotRunner) RunCommand(
// 	jobName string,
// 	command string,
// 	inputs []string,
// ) bool {
// 	return false
// }

// type LocalRunner struct {
// 	sync.Mutex
// }

// func (p *LocalRunner) Name() string {
// 	return fmt.Sprintf("%s@local", os.Getenv("USER"))
// }

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

// type SSHRunner struct {
// 	name     string
// 	port     uint16
// 	user     string
// 	host     string
// 	password string

// 	sync.Mutex
// }

// func NewSSHRunner(
// 	port uint16,
// 	user string,
// 	host string,
// ) (*SSHRunner, error) {
// 	ret := &SSHRunner{
// 		port:     port,
// 		user:     user,
// 		host:     host,
// 		password: "",
// 	}

// 	client, e := ret.getClient(SSHAuthIdle)
// 	if e != nil {
// 		return nil, e
// 	}
// 	_ = client.Close()

// 	if ret.password == "" {
// 		LogInput(fmt.Sprintf("Use PublicKey on %s@%s\n", ret.user, ret.host))
// 	}

// 	return ret, nil
// }

// func (p *SSHRunner) Name() string {
// 	return fmt.Sprintf("%s@%s", p.user, p.host)
// }

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

// func (p *SSHRunner) getClient(auth SSHAuth) (client *ssh.Client, ret error) {
// 	fnGetPublicKeyConfig := func(fileName string) (*ssh.ClientConfig, error) {
// 		keyPath := filepath.Join(os.Getenv("HOME"), ".ssh", fileName)
// 		if key, e := ioutil.ReadFile(keyPath); e != nil {
// 			return nil, fmt.Errorf("ssh: read private key: %s", e.Error())
// 		} else if signer, e := ssh.ParsePrivateKey(key); e != nil {
// 			return nil, fmt.Errorf("ssh: parse private key: %s", e.Error())
// 		} else {
// 			return &ssh.ClientConfig{
// 				User:            p.user,
// 				Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
// 				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
// 			}, nil
// 		}
// 	}

// 	fnGetPassworldConfig := func(password string) *ssh.ClientConfig {
// 		return &ssh.ClientConfig{
// 			User:            p.user,
// 			Auth:            []ssh.AuthMethod{ssh.Password(p.password)},
// 			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
// 		}
// 	}

// 	var clientCfg *ssh.ClientConfig

// 	switch auth {
// 	case SSHAuthIdle:
// 		if p.password != "" {
// 			return p.getClient(SSHAuthFixedPassword)
// 		}

// 		if client, ret = p.getClient(SSHAuthPublicKey); ret == nil {
// 			return
// 		}

// 		return p.getClient(SSHAuthInputPassword)
// 	case SSHAuthFixedPassword:
// 		clientCfg = fnGetPassworldConfig(p.password)
// 	case SSHAuthInputPassword:
// 		if p.password, ret = GetPasswordFromUser(fmt.Sprintf(
// 			"Password for ssh -p %d %s@%s: ",
// 			p.port, p.user, p.host),
// 		); ret != nil {
// 			return
// 		}
// 		clientCfg = fnGetPassworldConfig(p.password)
// 	case SSHAuthPublicKey:
// 		for _, keyAuth := range []SSHAuth{
// 			SSHAuthPublicKeyRSA, SSHAuthPublicKeyDSA, SSHAuthPublicKeyECDSA,
// 			SSHAuthPublicKeyED25519, SSHAuthPublicKeyXMSS,
// 		} {
// 			if client, ret = p.getClient(keyAuth); ret == nil {
// 				return
// 			}
// 		}

// 		return nil, fmt.Errorf("ssh: connect by publicKey failed")
// 	case SSHAuthPublicKeyRSA:
// 		if clientCfg, ret = fnGetPublicKeyConfig("id_rsa"); ret != nil {
// 			return
// 		}
// 	case SSHAuthPublicKeyDSA:
// 		if clientCfg, ret = fnGetPublicKeyConfig("id_dsa"); ret != nil {
// 			return
// 		}
// 	case SSHAuthPublicKeyECDSA:
// 		if clientCfg, ret = fnGetPublicKeyConfig("id_ecdsa"); ret != nil {
// 			return
// 		}
// 	case SSHAuthPublicKeyED25519:
// 		if clientCfg, ret = fnGetPublicKeyConfig("id_ed25519"); ret != nil {
// 			return
// 		}
// 	case SSHAuthPublicKeyXMSS:
// 		if clientCfg, ret = fnGetPublicKeyConfig("id_xmss"); ret != nil {
// 			return
// 		}
// 	}

// 	return ssh.Dial("tcp", fmt.Sprintf("%s:%d", p.host, p.port), clientCfg)
// }
