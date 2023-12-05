package filter

import (
	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/scheduler/filter"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util"
	"k8s.io/klog/v2"
)

const (
	minLocalStoragePct float64 = 20
	//
	ReasonLocalStorageTooLow = "LocalStorageTooLow"
)

// MinLocalStorageFilterFunc ensures local storage cannot be less than 20% of total storage
func MinLocalStorageFilterFunc(ctx *filter.FilterContext, n *state.Node, vol *v1.AntstorVolume) bool {
	var err = ctx.Error
	var isLocalVol = vol.Spec.HostNode.ID == n.Pool.Spec.NodeInfo.ID

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

func GetAllocatableRemoveVolumeSize(node *state.Node, volSize int64) (result int64) {
	result = volSize
	if result == 0 {
		return
	}
	if node != nil {
		allocRemotes := node.GetAllocatedRemoteBytes()
		total := node.Pool.GetAvailableBytes()
		// maxResultSize := int64(float64(total)*(100-minLocalStoragePct)*100) - int64(allocRemotes)
		maxResultSize := total - int64(float64(total)*minLocalStoragePct/100) - int64(allocRemotes)
		// cannot allocate remote volume
		if maxResultSize < 0 {
			return 0
		}

		if int64(maxResultSize) < result {
			result = int64(maxResultSize)
		}
	}

	result = result / util.FourMiB * util.FourMiB
	return
}
