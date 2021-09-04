package context

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sync"
)

type LocalRunner struct {
	sync.Mutex
}

func (p *LocalRunner) Name() string {
	return fmt.Sprintf("%s@local", os.Getenv("USER"))
}

func (p *LocalRunner) Run(ctx Context) bool {
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
