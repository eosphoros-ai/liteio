package osutil

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCmd(t *testing.T) {
	_ = os.Remove("/tmp/echo.sh")
	f, err := os.Create("/tmp/echo.sh")
	assert.NoError(t, err)
	f.WriteString(`#!/bin/bash
echo "out"
>&2 echo "error"`)
	f.Chmod(os.ModePerm)
	err = f.Close()
	assert.NoError(t, err)

	exec := NewCommandExec()
	out, stderr, err := exec.ExecCmdWithError("/tmp/echo.sh", nil)
	t.Log(string(out))
	t.Log(string(stderr))
	t.Log(err)

	var targetErr = fmt.Errorf("TargetError")
	newErr := fmt.Errorf("%w, new error", targetErr)
	assert.True(t, errors.Is(newErr, targetErr))
}
