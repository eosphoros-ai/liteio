package hostnqn

import (
	"bytes"
	"os"

	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/misc"
	"k8s.io/klog/v2"
)

const (
	// filepath of hostnqn
	HostNQNFile = "/etc/nvme/hostnqn"
	// nqn prefix for antstor
	AntstorHostNQNPrefix = "nqn.2021-03.com.alipay.host:uuid:"
)

var (
	HostNQNValue = ""
)

func InitHostNQN(nodeId string) (err error) {
	var (
		hasFile bool
		fbytes  []byte
	)

	hasFile, err = misc.FileExists(HostNQNFile)
	if err != nil {
		klog.Error(err)
	}

	if hasFile {
		fbytes, err = os.ReadFile(HostNQNFile)
		if err != nil {
			return
		}
		HostNQNValue = string(bytes.TrimSpace(fbytes))
	}

	if len(HostNQNValue) == 0 {
		// generate hostnqn
		HostNQNValue = AntstorHostNQNPrefix + nodeId
		// create file if not exist
		err = os.WriteFile(HostNQNFile, []byte(HostNQNValue), 0644)
		if err != nil {
			return
		}
	}

	klog.Info("hostnqn is ", HostNQNValue)

	return
}
