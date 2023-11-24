//go:build !linux
// +build !linux

package mount

func SafeFormat(source, fsType string, mkfsAgs []string) (err error) {
	return
}
