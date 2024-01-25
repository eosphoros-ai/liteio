package kubeutil

import (
	"testing"

	// "lite.io/liteio/pkg/config"
	"lite.io/liteio/pkg/agent/config"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPatchNodeStatus(t *testing.T) {
	nodeName := "test-node"
	node := newNode()
	kubeClient := fake.NewSimpleClientset(node)
	opt := config.NodeInfoKeys{}
	config.SetNodeInfoDefaults(&opt)
	nig := NewKubeNodeInfoGetter(kubeClient)

	basicInfo := FromLabels(NodeInfoOption{}, node.Labels)
	t.Log(basicInfo.ToLabels())

	info, err := nig.GetByNodeID(nodeName, NodeInfoOption{})
	assert.NoError(t, err)
	t.Log(info)
	assert.Equal(t, "192.168.1.100", info.IP)
	assert.Equal(t, "test-hostname", info.Hostname)
	assert.Equal(t, nodeName, info.ID)

	node, err = nig.ReportLocalDiskResource(nodeName, 200*1024)
	assert.NoError(t, err)
	q := node.Status.Capacity[SdsLocalStorageResourceKey]
	t.Logf("%s", q.String())
	assert.Equal(t, "204800", q.String())

	node, err = nig.RemoveLocalDiskResource(nodeName)
	assert.NoError(t, err)
	q = node.Status.Capacity[SdsLocalStorageResourceKey]
	t.Log(q.String())
	assert.Equal(t, "0", q.String())
}

func newNode() *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				K8SLabelKeyHostname: "test-hostname",
			},
		},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{
					Type:    corev1.NodeInternalIP,
					Address: "192.168.1.100",
				},
			},
			Capacity: corev1.ResourceList{
				// SdsLocalStorageResourceKey: *resource.NewQuantity(1024, resource.BinarySI),
				corev1.ResourceCPU: *resource.NewQuantity(10, resource.DecimalSI),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:         *resource.NewQuantity(10, resource.DecimalSI),
				SdsLocalStorageResourceKey: *resource.NewQuantity(1024, resource.BinarySI),
			},
		},
	}
}
