package scheduler

import (
	"testing"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/config"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSched(t *testing.T) {
	memState := state.NewState()
	sched := NewScheduler(
		config.Config{
			Scheduler: config.SchedulerConfig{
				Filters:    []string{"Basic", "Affinity"},
				Priorities: []string{"LeastResource", "PositionAdvice"},
			},
		})

	var nodeID = "node-1"
	pool := &v1.StoragePool{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeID,
			Labels: map[string]string{
				"logic-pool": "pool-1",
				"zone":       "zone-a",
			},
		},
		Spec: v1.StoragePoolSpec{
			KernelLVM: v1.KernelLVM{
				Bytes: 1024 * 10,
			},
			NodeInfo: v1.NodeInfo{
				ID: nodeID,
				Labels: map[string]string{
					"node-logic-pool": "node-pool-1",
					"node-zone":       "node-zone-a",
				},
			},
		},
		Status: v1.StoragePoolStatus{
			Status: v1.PoolStatusReady,
		},
	}

	memState.SetStoragePool(pool)

	vol := &v1.AntstorVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vol1",
			Labels: map[string]string{
				v1.VolumeDataHolderKey: "ob.clusterName.zoneName_01",
			},
		},
		Spec: v1.AntstorVolumeSpec{
			Uuid:           "vol1-uuid-xxx",
			PositionAdvice: v1.NoPreference,
			SizeByte:       1024,
			HostNode: &v1.NodeInfo{
				ID: "node-1",
			},
		},
	}

	targetNode, err := sched.ScheduleVolume(memState.GetAllNodes(), vol)
	assert.NoError(t, err)
	t.Logf("%+v", targetNode)
	// bind volume
	err = memState.BindAntstorVolume(targetNode.ID, vol)
	assert.NoError(t, err)

	// test node affinity, scheduling success
	vol2 := vol.DeepCopy()
	vol2.Name = "vol2"
	vol2.Spec.Uuid = "vol2-uuid"
	vol2.Annotations = map[string]string{
		v1.NodeLabelSelectorKey: "node-logic-pool=node-pool-1",
	}
	vol2.Spec.NodeAffinity = &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{
							Key:      "node-logic-pool",
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"node-pool-1"},
						},
					},
				},
			},
		},
	}
	vol2.Spec.PoolAffinity = &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{
							Key:      "logic-pool",
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"pool-1"},
						},
					},
				},
			},
		},
	}
	targetNode, err = sched.ScheduleVolume(memState.GetAllNodes(), vol2)
	assert.NoError(t, err)
	t.Logf("%+v, %+v", targetNode, err)

	// test node affinity fail
	vol2.Spec.NodeAffinity = &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{
							Key:      "node-logic-pool",
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"node-pool-xxxx"},
						},
					},
				},
			},
		},
	}
	targetNode, err = sched.ScheduleVolume(memState.GetAllNodes(), vol2)
	assert.Error(t, err)
	t.Logf("%+v, %+v", targetNode, err)

	// test node affinity fail
	vol2.Spec.NodeAffinity = nil
	vol2.Spec.PoolAffinity = &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{
							Key:      "logic-pool",
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"pool-xxxx"},
						},
					},
				},
			},
		},
	}
	targetNode, err = sched.ScheduleVolume(memState.GetAllNodes(), vol2)
	assert.Error(t, err)
	t.Logf("%+v, %+v", targetNode, err)

	// test node affinity fail
	vol2.Spec.NodeAffinity = nil
	vol2.Annotations = map[string]string{
		v1.PoolLabelSelectorKey: "logic-pool=pool-xxx",
	}
	targetNode, err = sched.ScheduleVolume(memState.GetAllNodes(), vol2)
	assert.Error(t, err)
	t.Logf("%+v, %+v", targetNode, err)

	// set nil Labels
	pool, err = memState.GetStoragePoolByNodeID(nodeID)
	assert.NoError(t, err)

	pool.Labels = nil
	pool.Spec.NodeInfo.Labels = nil
	targetNode, err = sched.ScheduleVolume(memState.GetAllNodes(), vol2)
	assert.Error(t, err)
	t.Logf("%+v, %+v", targetNode, err)

	vol2.Spec.NodeAffinity = nil
	vol2.Spec.PoolAffinity = nil
	delete(vol2.Annotations, v1.PoolLabelSelectorKey)
	delete(vol2.Annotations, v1.NodeLabelSelectorKey)
	targetNode, err = sched.ScheduleVolume(memState.GetAllNodes(), vol2)
	assert.NoError(t, err)
	t.Logf("%+v, %+v", targetNode, err)
}

func TestSchedVolGroup(t *testing.T) {
	var tenGiB uint64 = 10 << 30
	memState := state.NewState()
	sched := NewScheduler(
		config.Config{
			Scheduler: config.SchedulerConfig{
				MaxRemoteVolumeCount:     3,
				RemoteIgnoreAnnoSelector: nil,
				Filters:                  []string{"Basic", "Affinity"},
				Priorities:               []string{"LeastResource", "PositionAdvice"},
			},
		})

	memState.SetStoragePool(newStoragePool("node-1", tenGiB))
	memState.SetStoragePool(newStoragePool("node-2", tenGiB))
	memState.SetStoragePool(newStoragePool("node-3", tenGiB))
	memState.SetStoragePool(newStoragePool("node-4", tenGiB))

	memState.BindAntstorVolume("node-2", newVolume("vol-2", tenGiB/5))
	memState.BindAntstorVolume("node-3", newVolume("vol-1", tenGiB/10))

	volGroup := &v1.AntstorVolumeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "obnvmf",
			Name:      "volgroup-1",
		},
		Spec: v1.AntstorVolumeGroupSpec{
			TotalSize: 2 * int64(tenGiB),
			Uuid:      "uuid-vg-1",
			Stragety: v1.VolumeGroupStrategy{
				AllowEmptyNode: true,
			},
			DesiredVolumeSpec: v1.DesiredVolumeSpec{
				CountRange: v1.IntRange{
					Min: 1,
					Max: 5,
				},
				SizeRange: v1.QuantityRange{
					Min: resource.MustParse("2Gi"),
					Max: resource.MustParse("10Gi"),
				},
				SizeSymmetry: v1.Asymmetric,
			},
			Volumes: []v1.VolumeMeta{
				{
					VolId: v1.EntityIdentity{
						Namespace: "obnvmf",
						Name:      "volgroup-1-00-fixed",
						UUID:      "uuid-00-fixed",
					},
					Size:           4294967296, // 4Gi
					TargetNodeName: "node-1",
				},
				{
					VolId: v1.EntityIdentity{
						Namespace: "obnvmf",
						Name:      "volgroup-1-01-fixed",
						UUID:      "uuid-01-fixed",
					},
				},
			},
		},
	}

	err := sched.ScheduleVolumeGroup(memState.GetAllNodes(), volGroup)
	assert.NoError(t, err)

	t.Log(volGroup.Spec.Volumes)

}

func newStoragePool(nodeID string, size uint64) (pool *v1.StoragePool) {
	pool = &v1.StoragePool{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeID,
			Labels: map[string]string{
				"logic-pool": "pool-1",
				"zone":       "zone-a",
			},
		},
		Spec: v1.StoragePoolSpec{
			KernelLVM: v1.KernelLVM{
				Bytes: size,
			},
			NodeInfo: v1.NodeInfo{
				ID: nodeID,
				Labels: map[string]string{
					"node-logic-pool": "node-pool-1",
					"node-zone":       "node-zone-a",
				},
			},
		},
		Status: v1.StoragePoolStatus{
			Status: v1.PoolStatusReady,
			Conditions: []v1.PoolCondition{
				{
					Type:   v1.PoolConditionSpkdHealth,
					Status: v1.StatusOK,
				},
			},
		},
	}

	return
}

func newVolume(name string, size uint64) *v1.AntstorVolume {
	vol := &v1.AntstorVolume{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "obnvmf",
			Name:      name,
			Labels: map[string]string{
				v1.VolumeDataHolderKey: "ob.clusterName.zoneName_01",
			},
		},
		Spec: v1.AntstorVolumeSpec{
			Uuid:           "uuid-" + name,
			PositionAdvice: v1.NoPreference,
			SizeByte:       size,
			HostNode: &v1.NodeInfo{
				ID: "node-1",
			},
		},
	}
	return vol
}
