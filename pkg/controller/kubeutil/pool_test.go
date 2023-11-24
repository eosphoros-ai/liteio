package kubeutil

import (
	"context"
	"encoding/json"
	"testing"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	fakev1 "code.alipay.com/dbplatform/node-disk-controller/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestFakeClient(t *testing.T) {
	var scheme = scheme.Scheme
	var err error
	utilruntime.Must(v1.AddToScheme(scheme))

	cli := fakev1.NewSimpleClientset(getStoragePool())
	spCli := cli.VolumeV1().StoragePools("obnvmf")
	_, err = spCli.Get(context.Background(), "testpool", metav1.GetOptions{})
	assert.NoError(t, err)

	fakeCli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(getStoragePool()).Build()
	var pool v1.StoragePool
	err = fakeCli.Get(context.Background(), client.ObjectKey{
		Namespace: "obnvmf",
		Name:      "testpool",
	}, &pool)
	assert.NoError(t, err)
}

func TestIsDifferent(t *testing.T) {
	a := v1.NodeInfo{
		Labels: map[string]string{
			"a": "b",
			"c": "d",
		},
	}
	b := v1.NodeInfo{
		Labels: map[string]string{
			"c": "d",
			"a": "a",
		},
	}
	assert.True(t, IsNodeInfoDifferent(a, b))

	b.Labels["a"] = "b"
	assert.False(t, IsNodeInfoDifferent(a, b))
}

func TestPatchPool(t *testing.T) {
	// test-control-plane
	var scheme = runtime.NewScheme()
	var pool v1.StoragePool
	v1.AddToScheme(scheme)
	kubeCli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(getStoragePool()).Build()
	poolUtil := NewStoragePoolUtil(kubeCli)

	err := kubeCli.Get(context.TODO(), client.ObjectKey{
		Namespace: "obnvmf",
		Name:      "testpool",
	}, &pool)
	assert.NoError(t, err)

	err = poolUtil.TriggerLabelEvent(&pool, "obnvmf/test-trigger", nil)
	assert.NoError(t, err)
}

func TestTwoWayMergePatch(t *testing.T) {
	oldPool := getStoragePool()

	newPool := oldPool.DeepCopy()
	newPool.Status.Capacity[v1.ResourceDiskPoolByte] = resource.MustParse("200Mi")
	newPool.Status.VGFreeSize = resource.MustParse("100Mi")
	newPool.Status.Conditions = []v1.PoolCondition{
		{
			Type:   v1.PoolConditionSpkdHealth,
			Status: v1.StatusError,
		},
	}
	newPool.Status.Status = ""

	old, _ := json.Marshal(oldPool)
	new, _ := json.Marshal(newPool)

	patch, err := strategicpatch.CreateTwoWayMergePatch(old, new, &v1.StoragePool{})
	assert.NoError(t, err)

	t.Log(string(patch))
}

func getStoragePool() *v1.StoragePool {
	oldPool := &v1.StoragePool{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "obnvmf",
			Name:      "testpool",
		},
		Status: v1.StoragePoolStatus{
			Status: v1.PoolStatusReady,
			Capacity: corev1.ResourceList{
				v1.ResourceDiskPoolByte: resource.MustParse("100Mi"),
			},
			Conditions: []v1.PoolCondition{
				{
					Type:   v1.PoolConditionSpkdHealth,
					Status: v1.StatusOK,
				},
				{
					Type:   v1.PoolConditionLvmHealth,
					Status: v1.StatusOK,
				},
			},
		},
	}
	return oldPool
}
