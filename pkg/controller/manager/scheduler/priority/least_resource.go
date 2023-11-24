package priority

import (
	"context"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	"k8s.io/klog/v2"
)

// PriorityByLeastResource is a PriorityFunc. Nodes with less free resource are more prefered.
func PriorityByLeastResource(ctx context.Context, n *state.Node, vol *v1.AntstorVolume) int {
	var score int
	// LeastResourcePoriotiy
	// the less free resource is remained, the larger the score is
	/*
		score = (allocated / total) * 100
	*/

	var total = float64(n.Pool.GetVgTotalBytes())
	var free = float64(n.Pool.GetVgFreeBytes())
	if total <= 0 {
		klog.Errorf("found StoragePool %s total space is invalid", n.Info.ID)
		return 0
	}

	score = int((total - free) / total * 100)

	if score < 0 {
		score = 0
	}

	return score
}
