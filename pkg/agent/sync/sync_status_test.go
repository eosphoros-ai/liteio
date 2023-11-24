package sync

import (
	"reflect"
	"testing"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestStatusCompare(t *testing.T) {
	status := &v1.StoragePoolStatus{
		Conditions: []v1.PoolCondition{
			{
				Type:   v1.PoolConditionSpkdHealth,
				Status: v1.StatusOK,
			},
		},
		Capacity: make(corev1.ResourceList),
	}

	dupStatus := status.DeepCopy()

	t.Log(reflect.DeepEqual(status, dupStatus))
}
