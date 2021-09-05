package context

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"time"
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
	ctx Context, e error, out *bytes.Buffer, err *bytes.Buffer,
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
	Run(ctx Context) bool
}
