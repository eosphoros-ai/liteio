package pool

import (
	"lite.io/liteio/pkg/agent/config"
	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"lite.io/liteio/pkg/controller/kubeutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	NodeLabelPoolModeKey = "custom.k8s.alipay.com/nvmf-pool-mode"
)

type PoolDetector struct {
	nodeID   string
	kubeCli  kubernetes.Interface
	nodeKeys config.NodeInfoKeys
}

func NewPoolDetector(nodeID string, kubeCli kubernetes.Interface, nodeKeys config.NodeInfoKeys) *PoolDetector {
	return &PoolDetector{
		nodeID:   nodeID,
		kubeCli:  kubeCli,
		nodeKeys: nodeKeys,
	}
}

func (pd *PoolDetector) DetectMode() (mode v1.PoolMode, nodeInfo v1.NodeInfo, err error) {
	// default is LVM mode
	mode = v1.PoolModeKernelLVM

	nodeInfo, err = kubeutil.NewKubeNodeInfoGetter(pd.kubeCli).GetByNodeID(pd.nodeID, kubeutil.NodeInfoOption(pd.nodeKeys))
	if err != nil {
		klog.Error(err)
		return
	}

	if nodeInfo.Labels[NodeLabelPoolModeKey] == string(v1.PoolModeSpdkLVStore) {
		mode = v1.PoolModeSpdkLVStore
	}

	return
}
