package antplugin

import (
	"testing"

	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"lite.io/liteio/pkg/controller/manager/state"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNode(t *testing.T) {
	pool := v1.StoragePool{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: v1.DefaultNamespace,
			Name:      "node1",
			Labels:    map[string]string{},
		},
		Spec: v1.StoragePoolSpec{
			NodeInfo: v1.NodeInfo{
				ID: "node1",
			},
			KernelLVM: v1.KernelLVM{
				Bytes: 38654705664,
				ReservedLVol: []v1.KernelLVol{
					{
						Name:     "reserved-lv",
						SizeByte: 1024 * 1024 * 100, // 100MiB
					},
				},
			},
		},
		Status: v1.StoragePoolStatus{
			Capacity: corev1.ResourceList{
				v1.ResourceDiskPoolByte: resource.MustParse("36864Mi"),
			},
		},
	}
	node := state.NewNode(&pool)

	vol := v1.AntstorVolume{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: v1.DefaultNamespace,
			Name:      "vol1",
		},
		Spec: v1.AntstorVolumeSpec{
			Uuid:     "uuid-1",
			SizeByte: 10737418240 / 2, // 10737418240 is 10Gi
			// SizeByte: 0 * 1024 * 1024,
			HostNode: &v1.NodeInfo{
				ID: "node2",
			},
		},
	}

	node.AddVolume(&vol)

	freeList := node.GetFreeResourceNonLock()
	bytes, ok := freeList.Storage().AsInt64()
	t.Logf("free resource %s, int64 %d, bool %t", freeList.Storage().String(), bytes, ok)

	t.Logf("free resource %s", node.FreeResource.Storage().String())

	t.Log(node.RemoteVolumesCount(nil))

	t.Log("local storage capacity", CalculateLocalStorageCapacity(node))
	// localFree, err := GetFreeLocalBytes(node)
	// assert.NoError(t, err)
	// t.Log("local storge free space", localFree)

	// t.Log("vg space execpt reserved lvol", node.Pool.GetAvailableBytes())
	// remoteFree, err := GetFreeRemoteBytes(node)
	// assert.NoError(t, err)
	// t.Log("remote free space", remoteFree)
	// assert.Equal(t, int64(27812429824), remoteFree)

	// 10737418240 is 10Gi
	// 38654705664 - 1024 * 1024 * 100 - 10737418240 / 2 = 33181138944
	assert.Equal(t, uint64(33181138944), CalculateLocalStorageCapacity(node))
}
