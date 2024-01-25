package priority

import (
	"context"

	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"lite.io/liteio/pkg/controller/manager/state"
)

// PriorityByPositionAdivce is a PriorityFunc based on PositionAdvice
func PriorityByPositionAdivce(ctx context.Context, n *state.Node, vol *v1.AntstorVolume) int {
	var score int
	// vol.Position 根据给本地或者远程盘加分
	if vol.Spec.PositionAdvice == v1.PreferLocal {
		if n.Pool.Spec.NodeInfo.ID == vol.Spec.HostNode.ID {
			score += 20
		}
	}

	if vol.Spec.PositionAdvice == v1.PreferRemote {
		if n.Pool.Spec.NodeInfo.ID != vol.Spec.HostNode.ID {
			score += 20
		}
	}

	return score
}
