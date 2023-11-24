package osutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

type ShellExec interface {
	ExecCmd(cmd string, args []string) (stdout []byte, err error)
	ExecCmdWithError(cmd string, args []string) (stdout, stderr []byte, err error)
}

type osExec struct{}

func NewCommandExec() ShellExec {
	return &osExec{}
}

// ExecCmd run command and return stdout and error
func (o *osExec) ExecCmd(cmd string, args []string) (out []byte, err error) {
	var exitErr *exec.ExitError
	out, err = exec.Command(cmd, args...).Output()
	if err != nil {
		if errors.As(err, &exitErr) {
			err = fmt.Errorf("cmd:%s %v, ProcState=%s StdErr=%s, %w", cmd, args, exitErr.ProcessState.String(), string(exitErr.Stderr), err)
		} else {
			err = fmt.Errorf("cmd:%s %v, %w", cmd, args, err)
		}
		return
	}

	return
}

// ExecCmdWithError run command and return stdout, stderr and error. If exit code > 0, err is not nil.
func (o *osExec) ExecCmdWithError(cmd string, args []string) (stdout, stderr []byte, err error) {
	var cmdExec = exec.Command(cmd, args...)
	var errb bytes.Buffer
	cmdExec.Stderr = &errb
	stdout, err = cmdExec.Output()
	// get stderr
	stderr, _ = io.ReadAll(&errb)

	// if cmd exit code > 0, err is not nil
	if err != nil {
		err = fmt.Errorf("cmd:%s %v, out=%s err=%s, %w", cmd, args, string(stdout), string(stderr), err)
		return
	}

	return
}
