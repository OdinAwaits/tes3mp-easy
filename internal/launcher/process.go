package launcher

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// CommandExecutor starts a real process. Env entries are appended to the
// current environment, so they override inherited values on Unix and Windows.
func CommandExecutor(ctx context.Context, spec Spec) (Process, error) {
	if strings.TrimSpace(spec.Name) == "" {
		return nil, errors.New("launcher: executable is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	command := exec.CommandContext(ctx, spec.Name, spec.Args...)
	command.Dir = spec.Dir
	command.Env = append(os.Environ(), spec.Env...)
	if err := command.Start(); err != nil {
		return nil, err
	}
	return &commandProcess{command: command}, nil
}

type commandProcess struct {
	command *exec.Cmd
}

func (p *commandProcess) Wait() error {
	if p == nil || p.command == nil {
		return errors.New("launcher: process is not running")
	}
	return p.command.Wait()
}
