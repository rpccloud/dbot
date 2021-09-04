package context

import "sync"

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
