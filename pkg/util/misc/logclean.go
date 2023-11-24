package misc

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

type TruncateFile struct {
	// FilePath is full path of file
	FilePath string
	// MaxBytesSize is the max size of the file
	MaxBytesSize int64
}

type LogCleanByModifyTime struct {
	// logs under Dir
	Dir string
	// Delete logs modified Duration age
	Ago time.Duration
	// white list
	WhiteRegex []string
	regex      []*regexp.Regexp
	// static file to truncate
	filesToTruncate []TruncateFile
}

func NewLogCleanByModifyTime(dir string, ago time.Duration, whiteRegex []string, truncates []TruncateFile) *LogCleanByModifyTime {
	ret := &LogCleanByModifyTime{
		Dir:        dir,
		Ago:        ago,
		WhiteRegex: whiteRegex,
		regex:      make([]*regexp.Regexp, 0, len(whiteRegex)),
		// static files to truncate
		filesToTruncate: truncates,
	}
	for _, item := range whiteRegex {
		ret.regex = append(ret.regex, regexp.MustCompile(item))
	}

	return ret
}

func (l *LogCleanByModifyTime) Clean() {
	finfo, err := ioutil.ReadDir(l.Dir)
	if err != nil {
		return
	}
	// sort file by modify time, oldest to latest
	sort.Slice(finfo, func(i, j int) bool {
		return finfo[i].ModTime().Before(finfo[j].ModTime())
	})

	for _, info := range finfo {
		fname := info.Name()
		if time.Now().Sub(info.ModTime()) > l.Ago && validateFileName(fname, l.regex) {
			filePath := filepath.Join(l.Dir, fname)
			err = os.Remove(filePath)
			if err != nil {
				log.Println(err)
			}
		}
	}

	// clean file by static filename
	for _, file := range l.filesToTruncate {
		err = truncateFileIfExists(file)
		if err != nil {
			log.Println(err)
		}
	}
}

func validateFileName(fname string, regex []*regexp.Regexp) (valid bool) {
	for _, reg := range regex {
		if reg.Match([]byte(fname)) {
			return true
		}
	}
	return false
}

func truncateFileIfExists(file TruncateFile) (err error) {
	finfo, err := os.Stat(file.FilePath)
	if os.IsNotExist(err) {
		return nil
	}
	if finfo.Size() > file.MaxBytesSize {
		err = os.Truncate(file.FilePath, 0)
	}
	return
}
