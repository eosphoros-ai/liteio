//go:build !linux
// +build !linux

package mount

import "k8s.io/mount-utils"

func NewSafeMounter() *mount.SafeFormatAndMount {
	return nil
}
