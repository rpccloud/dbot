package context

import (
	"io"
	"strings"
	"sync"
	"time"
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
	Name() string
	Run(ctx Context) bool
}
