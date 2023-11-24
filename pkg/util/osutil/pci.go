package osutil

import (
	"errors"
	"fmt"
	"strings"
)

const (
	NVMeDriverName    = "nvme"
	VfioPCIDriverName = "vfio-pci"
)

type PCIUtilityIface interface {
	ListNVMeID() (ids []string, err error)
	UnbindNVMe(id, driver string) (err error)
	GetNVMeTypeID(id string) (tid string, err error)
	BindNVMeByType(tid, driver string) (err error)
	CheckNVMeExistence(id, driver string) (err error)
}

type PCIUtil struct {
	exec ShellExec
}

func NewPCIUtil(exec ShellExec) *PCIUtil {
	return &PCIUtil{
		exec: exec,
	}
}

func (pu *PCIUtil) ListNVMeID() (ids []string, err error) {
	shell := `set -o pipefail; lspci -vv | grep 'Non-Volatile memory controller' | awk '{print "0000:"$1}'`

	out, err := pu.exec.ExecCmd("bash", []string{"-c", shell})
	if err != nil {
		err = errors.New(err.Error() + string(out))
		return
	}

	lines := strings.Split(string(out), "\n")
	for _, item := range lines {
		item = strings.TrimSpace(item)
		if item != "" {
			ids = append(ids, item)
		}
	}

	return
}

func (pu *PCIUtil) UnbindNVMe(id, driver string) (err error) {
	shell := fmt.Sprintf(`echo "%s" > /sys/bus/pci/drivers/%s/unbind`, id, driver)

	out, err := pu.exec.ExecCmd("sh", []string{"-c", shell})
	if err != nil {
		err = errors.New(err.Error() + string(out))
		return
	}
	return
}

func (pu *PCIUtil) GetNVMeTypeID(id string) (tid string, err error) {
	/*
		lspci -n -s "$NVMeID" output maybe:
		11:00.0 0108: 144d:a822 or 11:00.0 0108: 144d:a822 (rev 01)
		awk '{print $NF}' is not compatible, use '{print $3}' instead
	*/
	shell := fmt.Sprintf(`lspci -n -s "%s" | awk '{print $3}'`, id)

	out, err := pu.exec.ExecCmd("sh", []string{"-c", shell})
	if err != nil {
		err = errors.New(err.Error() + string(out))
		return
	}

	tid = strings.TrimSpace(string(out))
	return
}

func (pu *PCIUtil) BindNVMeByType(tid, driver string) (err error) {
	tid = strings.Replace(tid, ":", " ", 1)

	shell := fmt.Sprintf(`echo "%s" > /sys/bus/pci/drivers/%s/new_id`, tid, driver)
	out, err := pu.exec.ExecCmd("sh", []string{"-c", shell})
	if err != nil {
		err = errors.New(err.Error() + string(out))
		return
	}

	return
}

// CheckNVMeExistence return nil error if nvme exists in driver
func (pu *PCIUtil) CheckNVMeExistence(id, driver string) (err error) {
	out, err := pu.exec.ExecCmd("ls", []string{fmt.Sprintf("/sys/bus/pci/drivers/%s/%s", driver, id)})
	if err != nil {
		err = errors.New(err.Error() + string(out))
		return
	}

	return
}
