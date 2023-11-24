//go:build !linux && !darwin
// +build !linux,!darwin

package misc

import "os"

func Fallocate(file *os.File, offset int64, length int64) error {
	var buf [65536]byte

	file.Seek(offset, os.SEEK_SET)
	for length > 0 {
		now := int64(65536)
		if length < now {
			now = length
		}

		_, err := file.Write(buf[:now])
		if err != nil {
			return err
		}
		length -= now
	}

	return nil
}
