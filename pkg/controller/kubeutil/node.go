package kubeutil

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"lite.io/liteio/pkg/agent/config"
	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
)

const (
	K8SLabelKeyHostname = "kubernetes.io/hostname"
	K8SLabelKeyArch     = "kubernetes.io/arch"

	SdsLocalStorageResourceKey = "sds/local-storage"
)

type NodeInfoGetterIface interface {
	GetByNodeID(nodeID string, opt NodeInfoOption) (info v1.NodeInfo, err error)
}

type NodeUpdaterIface interface {
	ReportLocalDiskResource(nodeName string, sizeByte uint64) (node *corev1.Node, err error)
	RemoveLocalDiskResource(nodeName string) (node *corev1.Node, err error)
}

type NodeInfoOption config.NodeInfoKeys

// get node from kube api
type kubeApiNodeInfo struct {
	// kube client
	kubeCli kubernetes.Interface
}

func NewKubeNodeInfoGetter(kubeCli kubernetes.Interface) (getter *kubeApiNodeInfo) {
	return &kubeApiNodeInfo{
		kubeCli: kubeCli,
	}
}

func (kn *kubeApiNodeInfo) GetByNodeID(nodeID string, opt NodeInfoOption) (info v1.NodeInfo, err error) {
	node, err := kn.kubeCli.CoreV1().Nodes().Get(context.Background(), nodeID, metav1.GetOptions{})
	if err != nil {
		return
	}

	basicInfo := FromLabels(opt, node.Labels)
	info.IP = basicInfo.IP
	info.Hostname = basicInfo.Hostname
	info.ID = nodeID

	info.Labels = node.Labels

	// fallback: if IP from labels is empty, use InternalIP of the node
	if info.IP == "" {
		for _, item := range node.Status.Addresses {
			if item.Type == corev1.NodeInternalIP {
				info.IP = item.Address
			}
		}
	}

	return
}

func (kn *kubeApiNodeInfo) ReportLocalDiskResource(nodeName string, sizeByte uint64) (node *corev1.Node, err error) {
	node, err = kn.kubeCli.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return
	}
	key := strings.ReplaceAll(SdsLocalStorageResourceKey, "/", "~1")
	patch := fmt.Sprintf(`[{"op": "add", "path": "/status/capacity/%s", "value": "%d"}]`, key, sizeByte)

	if _, has := node.Status.Capacity[SdsLocalStorageResourceKey]; has {
		patch = fmt.Sprintf(`[{"op": "replace", "path": "/status/capacity/%s", "value": "%d"}]`, key, sizeByte)
	}

	data := []byte(patch)
	node, err = kn.kubeCli.CoreV1().Nodes().Patch(context.Background(), nodeName, types.JSONPatchType, data, metav1.PatchOptions{}, "status")
	return
}

func (kn *kubeApiNodeInfo) RemoveLocalDiskResource(nodeName string) (node *corev1.Node, err error) {
	key := strings.ReplaceAll(SdsLocalStorageResourceKey, "/", "~1")
	jsonPatchStr := fmt.Sprintf(`[{"op": "remove", "path": "/status/capacity/%s"}, {"op": "remove", "path": "/status/allocatable/%s"}]`, key, key)
	data := []byte(jsonPatchStr)
	node, err = kn.kubeCli.CoreV1().Nodes().Patch(context.Background(), nodeName, types.JSONPatchType, data, metav1.PatchOptions{}, "status")
	return
}

type BasicNodeInfo struct {
	IP       string
	Hostname string
	Rack     string
	Room     string
	Arch     string
}

func FromLabels(opt NodeInfoOption, labels map[string]string) (b BasicNodeInfo) {
	if labels == nil {
		return
	}

	b = BasicNodeInfo{
		IP:       labels[opt.IPLabelKey],
		Hostname: labels[opt.HostnameLabelKey],
		Rack:     labels[opt.RackLabelKey],
		Room:     labels[opt.RoomLabelKey],
		Arch:     labels[K8SLabelKeyArch],
	}

	// fallback of Hostname
	if b.Hostname == "" {
		b.Hostname = labels[K8SLabelKeyHostname]
	}

	return b
}

func (b BasicNodeInfo) ToLabels() map[string]string {
	m := map[string]string{
		"ip":       b.IP,
		"hostname": b.Hostname,
		"rack":     b.Rack,
		"room":     b.Room,
		"arch":     b.Arch,
	}

	return m
}
