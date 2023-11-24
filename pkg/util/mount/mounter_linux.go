//go:build linux
// +build linux

package mount

import (
	"k8s.io/mount-utils"
	"k8s.io/utils/exec"
)

func NewSafeMounter() *mount.SafeFormatAndMount {
	realMounter := mount.New("")
	realExec := exec.New()
	return &mount.SafeFormatAndMount{
		Interface: realMounter,
		Exec:      realExec,
	}
}
