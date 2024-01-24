package state

import (
	"testing"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
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
				Bytes: 38654705664, // 36864 MiB
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
	node := NewNode(&pool)

	vol := v1.AntstorVolume{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: v1.DefaultNamespace,
			Name:      "vol1",
			Labels: map[string]string{
				v1.VolumeContextKeyPvcNS:   "obnvmf",
				v1.VolumeContextKeyPvcName: "pvc-1",
			},
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
	node.Reserve(&reservation{
		id:             "obnvmf/pvc-1",
		namespacedName: "obnvmf/pvc-1",
		sizeByte:       10737418240 / 2,
	})

	freeList := node.GetFreeResourceNonLock()
	bytes, ok := freeList.Storage().AsInt64()
	t.Logf("free resource %s, int64 %d, bool %t, freeStorage=%s", freeList.Storage().String(), bytes, ok, node.FreeResource.Storage().String())

	assert.Equal(t, freeList.Storage().String(), node.FreeResource.Storage().String())

	t.Log(node.RemoteVolumesCount(nil))
	t.Log("vg space execpt reserved lvol", node.Pool.GetAvailableBytes())

	assert.Equal(t, 0, len(node.resvSet.Items()))

	node.Reserve(&reservation{
		id:             "obnvmf/pvc-2",
		namespacedName: "obnvmf/pvc-2",
		sizeByte:       10737418240 / 2,
	})
	freeList = node.GetFreeResourceNonLock()
	bytes, ok = freeList.Storage().AsInt64()
	t.Logf("free resource %s, int64 %d, bool %t, freeStorage=%s", freeList.Storage().String(), bytes, ok, node.FreeResource.Storage().String())

	volCopy := vol.DeepCopy()
	volCopy.Name = "vol-2"
	volCopy.Spec.Uuid = "uuid-2"
	volCopy.Labels = map[string]string{
		v1.VolumeContextKeyPvcNS:   "obnvmf",
		v1.VolumeContextKeyPvcName: "pvc-2",
	}
	node.AddVolume(volCopy)
	t.Logf("free resource %s, freeStorage=%s", freeList.Storage().String(), node.FreeResource.Storage().String())

	assert.Equal(t, 0, len(node.resvSet.Items()))
}

func TestCompareError(t *testing.T) {
	err := newNotFoundNodeError("test")
	assert.True(t, IsNotFoundNodeError(err))
}

func TestMinusResource(t *testing.T) {
	q := resource.NewQuantity(0, resource.BinarySI)
	assert.Zero(t, q.CmpInt64(0))

	q.Sub(resource.MustParse("1Mi"))
	assert.True(t, q.CmpInt64(0) == -1)

	t.Log(q.AsInt64())
}

func TestReservation(t *testing.T) {
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
				Bytes: 38654705664, // 36864 MiB
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
	node := NewNode(&pool)

	node.Reserve(NewReservation("resv-id", 1024*1024*100))

	t.Log(node.FreeResource.Storage().String())

}
