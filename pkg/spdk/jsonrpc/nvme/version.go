package nvme

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"k8s.io/klog/v2"
)

const (
	nvmeTcpModVersionPath = "/sys/module/nvme_tcp/version"
)

var (
	NvmeTcpVersion NvmfVersion
)

type NvmfVersion struct {
	Version string
	Commit  string
	Time    string
}

func LoadNVMeTCP() (err error) {
	lsModCmd := exec.Command("bash", "-c", "lsmod | grep nvme_tcp")
	out, errLsMod := lsModCmd.Output()
	if bytes.Contains(out, []byte("nvme_tcp")) {
		klog.Infof("lsmod returns %s, mod nvme_tcp is already loaded", string(out))
		NvmeTcpVersion = GetNvmeTcpModVersion()
		return
	}

	if errLsMod != nil {
		klog.Info("Running modprobe nvme-tcp to load kernel module")
		out, errModProbe := exec.Command("modprobe", "nvme-tcp").CombinedOutput()
		if errModProbe != nil {
			err = fmt.Errorf("modprobe nvme-tcp failed: err=%+v, out=%s", errModProbe, string(out))
			return
		}
		NvmeTcpVersion = GetNvmeTcpModVersion()
		return
	}

	err = fmt.Errorf("unknown error when loading nvme_tcp mod")

	return
}

/*
cat /sys/module/nvme_tcp/version
alinvme v0.0.8 a4e115c 2022-11-17 18:44:03
alinvme v0.0.6
*/
func GetNvmeTcpModVersion() (ver NvmfVersion) {
	var err error
	var bs []byte
	bs, err = os.ReadFile(nvmeTcpModVersionPath)
	if err != nil {
		klog.Errorf("nvme-tcp mod has no version file, %s", err)
		return
	}

	ver = parseNVMeTCPVersion(string(bytes.TrimSpace(bs)))

	return
}

func parseNVMeTCPVersion(raw string) (ver NvmfVersion) {
	parts := strings.SplitN(raw, " ", 4)
	if len(parts) >= 2 {
		ver.Version = parts[1]
	}
	if len(parts) >= 3 {
		ver.Commit = parts[2]
	}
	if len(parts) >= 4 {
		ver.Time = parts[3]
	}
	return
}
