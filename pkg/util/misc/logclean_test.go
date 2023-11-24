package misc

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLogClean(t *testing.T) {
	dir := "/tmp/ob-logclean"
	os.RemoveAll(dir)

	err := os.Mkdir(dir, os.ModePerm)
	assert.NoError(t, err)

	filesToCreate := []string{
		"ocp-agent.ali-146033.jason.log.ERROR.20200526-114933.93798",
		"stdout.log",
	}
	for _, name := range filesToCreate {
		file := filepath.Join(dir, name)
		f, err := os.Create(file)
		assert.NoError(t, err)
		_, err = f.WriteString("test")
		assert.NoError(t, err)
		f.Close()
	}
	logFileName := filepath.Join(dir, "ocp-agent.ali-146033.jason.log.ERROR.20200526-114933.93798")
	stdFileName := filepath.Join(dir, "stdout.log")

	time.Sleep(time.Second)

	clean := NewLogCleanByModifyTime(dir, 10*time.Second, []string{"ocp-agent.+(ERROR|INFO|WARNING).+"}, []TruncateFile{{FilePath: stdFileName, MaxBytesSize: 1}})
	clean.Clean()

	exist, err := FileExists(logFileName)
	assert.True(t, exist)

	clean.Ago = time.Second
	clean.Clean()
	exist, err = FileExists(logFileName)
	assert.False(t, exist)

	finfo, err := os.Stat(stdFileName)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), finfo.Size())
}
