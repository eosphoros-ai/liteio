package osutil

import (
	"k8s.io/klog/v2"
)

func CreateLoopDevice(exec ShellExec, devPath, filePath string) (err error) {
	var (
		out []byte
	)

	out, err = exec.ExecCmd("losetup", []string{
		devPath,
		filePath,
	})
	if err != nil {
		klog.Errorf("err %+v, output: %s %s", err, string(out))
		return
	}

	return
}
