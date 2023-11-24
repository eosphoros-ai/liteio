//go:build linux
// +build linux

package mount

import (
	"fmt"
	"os/exec"

	"k8s.io/klog/v2"
)

const (
	FsTypeXfs  = "xfs"
	FsTypeExt4 = "ext4"
)

func SafeFormat(source, fsType string, mkfsAgs []string) (err error) {
	var mounter = NewSafeMounter()
	existingFormat, err := mounter.GetDiskFormat(source)
	if err != nil {
		return fmt.Errorf("format error: %s, %s, %+v", source, fsType, err)
	}

	if existingFormat == "" {
		// Disk is unformatted so format it.
		args := []string{source}
		if fsType == "ext4" || fsType == "ext3" {
			args = []string{
				"-F",  // Force flag
				"-m0", // Zero blocks reserved for super-user
			}
		} else if fsType == "xfs" {
			args = []string{
				"-f", // force flag
				// no reflink uses much less space for xfs metadata
				"-m", "reflink=0",
			}
		}
		if len(mkfsAgs) > 0 {
			args = append(args, mkfsAgs...)
		}
		args = append(args, source)

		klog.Infof("Disk %q appears to be unformatted, attempting to format as type: %q with args: %v", source, fsType, args)
		output, err := exec.Command("mkfs."+fsType, args...).CombinedOutput()
		if err != nil {
			detailedErr := fmt.Sprintf("format of disk %q failed: type:(%q) args:(%+v) errcode:(%v) output:(%v) ", source, fsType, args, err, string(output))
			klog.Error(detailedErr)
			return err
		}
	} else {
		klog.Infof("disk %s already has format %s, request fstype %s", source, existingFormat, fsType)
	}

	return
}
