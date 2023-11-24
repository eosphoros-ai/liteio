package misc

import (
	"fmt"
	"io/ioutil"
	"os"
)

// FileExists reports whether the named file or directory exists.
func FileExists(name string) (bool, error) {
	var err error
	if _, err = os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false, err
		}
	}
	return true, err
}

// ReadFileContent reads all contents from the file
func ReadFileContent(file string) (content []byte, err error) {
	var f *os.File
	f, err = os.OpenFile(file, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return
	}
	content, err = ioutil.ReadAll(f)
	return
}

// FileModifyTimestamp get ModTime in unix timstamp of the file
func FileModifyTimestamp(filename string) (int64, error) {
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return 0, err
	}
	return fileInfo.ModTime().Unix(), err
}

// RemoveFile removes a file at path, ignore error if file does not exist
func RemoveFile(path string) (err error) {
	if len(path) == 0 {
		return
	}
	has, _ := FileExists(path)
	if has {
		return os.Remove(path)
	}
	return
}

func CreateFallocateFile(fpath string, sizeByte int64) (err error) {
	if has, errF := FileExists(fpath); errF == nil && has {
		return nil
	}

	var file *os.File
	file, err = os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		err = fmt.Errorf("open file failed: %w", err)
		return
	}
	defer file.Close()

	err = Fallocate(file, 0, sizeByte)
	return
}
