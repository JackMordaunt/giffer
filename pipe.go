package giffer

import (
	"bytes"
	"io"
	"log"
	"os/exec"

	"github.com/pkg/errors"
)

// CmdPipe executes a stack of commands, piping in order.
type CmdPipe struct {
	In    io.Reader
	Out   io.Writer
	Stack []*exec.Cmd

	Debug bool
}

// Run the commands.
func (p CmdPipe) Run() (err error) {
	var errBuf bytes.Buffer
	defer func() {
		if p.Debug {
			log.Printf("%s", errBuf.String())
		}
	}()
	pipes := make([]*io.PipeWriter, len(p.Stack)-1)
	ii := 0
	for ; ii < len(p.Stack)-1; ii++ {
		if ii == 0 {
			p.Stack[ii].Stdin = p.In
		}
		stdin, stdout := io.Pipe()
		p.Stack[ii].Stdout = stdout
		p.Stack[ii].Stderr = &errBuf
		p.Stack[ii+1].Stdin = stdin
		pipes[ii] = stdout
	}
	p.Stack[ii].Stdout = p.Out
	p.Stack[ii].Stderr = &errBuf
	if err := call(p.Stack, pipes); err != nil {
		return errors.Wrap(err, string(errBuf.Bytes()))
	}
	return err
}

func call(stack []*exec.Cmd, pipes []*io.PipeWriter) (err error) {
	if stack[0].Process == nil {
		if err = stack[0].Start(); err != nil {
			return err
		}
	}
	if len(stack) > 1 {
		if err = stack[1].Start(); err != nil {
			return err
		}
		defer func() {
			if err == nil {
				pipes[0].Close()
				err = call(stack[1:], pipes[1:])
			}
		}()
	}
	return stack[0].Wait()
}
