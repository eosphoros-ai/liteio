package filter

import (
	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/config"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	"k8s.io/klog/v2"
)

func BasicFilterFunc(ctx *FilterContext, n *state.Node, vol *v1.AntstorVolume) bool {
	// consider Pool status
	if !n.Pool.IsSchedulable() {
		klog.Infof("[SchedFail] vol=%s Pool %s status is %s, or check Pool labels", vol.Name, n.Pool.Name, n.Pool.Status.Status)
		ctx.Error.AddReason(ReasonPoolUnschedulable)
		return false
	}

	var (
		cfg config.SchedulerConfig = ctx.Config
		err *MergedError           = ctx.Error
	)
	// check if err is nil

	// consider Pool FreeSpace
	var freeRes = n.GetFreeResourceNonLock()
	var freeDisk = freeRes[v1.ResourceDiskPoolByte]
	// comparing quantity. freeDisk cannot be convert to int64 by AsInt64()
	if freeDisk.CmpInt64(int64(vol.Spec.SizeByte)) < 0 {
		klog.Infof("[SchedFail] vol=%s Pool %s freeBytes is %s, has %d volumes on it. volSize=%d,", vol.Name, n.Pool.Name, freeDisk.String(), len(n.Volumes), vol.Spec.SizeByte)
		err.AddReason(ReasonPoolFreeSize)
		return false
	}

	// if Pool type is SPDK Lvol and spdk condition is bad, then reject the Volume
	var tgtConditionOk bool
	var spdkCond v1.ConditionStatus
	for _, item := range n.Pool.Status.Conditions {
		if item.Type == v1.PoolConditionSpkdHealth {
			tgtConditionOk = item.Status == v1.StatusOK
			spdkCond = item.Status
		}
	}
	if !tgtConditionOk && n.Pool.Spec.SpdkLVStore.Name != "" {
		klog.Infof("[SchedFail] vol=%s Pool(Spdk) %s, Spdk Condition(%s) is not OK", vol.Name, n.Pool.Name, string(spdkCond))
		err.AddReason(ReasonSpdkUnhealthy)
		return false
	}

	// consider exploding radius
	// limit remote disk, not local disk
	var isLocalVol = n.Pool.Spec.NodeInfo.ID == vol.Spec.HostNode.ID
	if !isLocalVol {
		var remoteVolCount = n.RemoteVolumesCount(cfg.RemoteIgnoreAnnoSelector)
		if remoteVolCount >= cfg.MaxRemoteVolumeCount {
			klog.Infof("[SchedFail] vol=%s Pool %s , Volumes count is %d, remote volume count is %d", vol.Name, n.Pool.Name, len(n.Volumes), remoteVolCount)
			err.AddReason(ReasonRemoteVolMaxCount)
			return false
		}

		if !tgtConditionOk {
			klog.Infof("[SchedFail] vol=%s Pool %s, Spdk Condition(%s) is not OK", vol.Name, n.Pool.Name, string(spdkCond))
			err.AddReason(ReasonSpdkUnhealthy)
			return false
		}
	}

	// consider Volume Position Preference
	if vol.Spec.PositionAdvice == v1.MustLocal {
		if n.Pool.Spec.NodeInfo.ID != vol.Spec.HostNode.ID {
			klog.Infof("[SchedFail] vol=%s posision advice %s, but Pool.NodeID %s, vol.HostNodeID %s",
				vol.Name, vol.Spec.PositionAdvice, n.Pool.Spec.NodeInfo.ID, vol.Spec.HostNode.ID)
			err.AddReason(ReasonPositionNotMatch)
			return false
		}
	}
	if vol.Spec.PositionAdvice == v1.MustRemote {
		if n.Pool.Spec.NodeInfo.ID == vol.Spec.HostNode.ID {
			klog.Infof("[SchedFail] vol=%s posision advice %s, but Pool.NodeID %s, vol.HostNodeID %s",
				vol.Name, vol.Spec.PositionAdvice, n.Pool.Spec.NodeInfo.ID, vol.Spec.HostNode.ID)
			err.AddReason(ReasonPositionNotMatch)
			return false
		}
	}

	// consider VolumeType
	switch vol.Spec.Type {
	case v1.VolumeTypeSpdkLVol:
		if n.Pool.Spec.SpdkLVStore.UUID == "" {
			klog.Infof("[SchedFail] vol=%s Pool %s, VolumeType not match", vol.Name, n.Pool.Name)
			err.AddReason(ReasonVolTypeNotMatch)
			return false
		}
	case v1.VolumeTypeKernelLVol:
		if n.Pool.Spec.KernelLVM.VgUUID == "" {
			klog.Infof("[SchedFail] vol=%s Pool %s, VolumeType not match", vol.Name, n.Pool.Name)
			err.AddReason(ReasonVolTypeNotMatch)
			return false
		}
	}

	return true
}
