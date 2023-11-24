package osutil

import (
	"bytes"
	"errors"
	"fmt"
)

type KmodUtilityIface interface {
	HasKmod(name string) (err error)
	ProbeKmod(name string) (err error)
}

type KmodUtil struct {
	exec ShellExec
}

func NewKmodUtil(exec ShellExec) *KmodUtil {
	return &KmodUtil{
		exec: exec,
	}
}

func (ku *KmodUtil) HasKmod(name string) (err error) {
	shell := fmt.Sprintf(`lsmod | grep %s`, name)
	out, err := ku.exec.ExecCmd("sh", []string{"-c", shell})
	if !bytes.Contains(out, []byte(name)) {
		err = errors.New(err.Error() + string(out))
		return
	}

	return nil
}

func (ku *KmodUtil) ProbeKmod(name string) (err error) {
	shell := fmt.Sprintf(`modprobe %s`, name)
	out, err := ku.exec.ExecCmd("sh", []string{"-c", shell})
	if err != nil {
		err = errors.New(err.Error() + string(out))
		return
	}

	return nil
}
