package filter

import (
	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	"k8s.io/klog/v2"
)

const (
	ReasonLocalStorageTooLow = "LocalStorageTooLow"
)

// MinLocalStorageFilterFunc ensures local storage cannot be less than 20% of total storage
func MinLocalStorageFilterFunc(ctx *FilterContext, n *state.Node, vol *v1.AntstorVolume) bool {
	var err = ctx.Error
	var isLocalVol = vol.Spec.HostNode.ID == n.Pool.Spec.NodeInfo.ID
	var minLocalStoragePct = float64(ctx.Config.MinLocalStoragePct)

	if !isLocalVol {
		allocRemotes := n.GetAllocatedRemoteBytes()
		total := n.Pool.GetAvailableBytes()
		localPct := float64(total-int64(allocRemotes)-int64(vol.Spec.SizeByte)) / float64(total) * 100
		if localPct < minLocalStoragePct {
			klog.Infof("[SchedFail] vol=%s Pool %s local-storage pct too low (%f)", vol.Name, n.Pool.Name, localPct)
			err.AddReason(ReasonLocalStorageTooLow)
			return false
		}
	}

	return true
}
