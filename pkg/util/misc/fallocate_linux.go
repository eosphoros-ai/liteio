package misc

import (
	"fmt"
	"os"
	"syscall"
)

// Fallocate file
func Fallocate(file *os.File, offset, length int64) (err error) {
	err = syscall.Fallocate(int(file.Fd()), 0, offset, length)
	if err != nil {
		return
	}
	finfo, err := file.Stat()
	if err != nil {
		return
	}
	if finfo.Size() < (length + offset) {
		err = fmt.Errorf("fallocate file %s to size %d, want %d", file.Name(), finfo.Size(), length+offset)
		return
	}

	return
}
