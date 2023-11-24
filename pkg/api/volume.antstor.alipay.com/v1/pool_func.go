package v1

import (
	"math"
)

// GetVgTotalBytes get total space of VolumeGroup in byte, including reserved space
// VG总空间，包括保留LV的空间，单位字节
func (sp *StoragePool) GetVgTotalBytes() int64 {
	var bytes int64
	if bytes <= 0 && sp.Spec.KernelLVM.Bytes > 0 {
		bytes = int64(sp.Spec.KernelLVM.Bytes)
	}

	if bytes <= 0 && sp.Spec.SpdkLVStore.Bytes > 0 {
		bytes = int64(sp.Spec.SpdkLVStore.Bytes)
	}

	if bytes <= 0 {
		var ok bool
		storeQuan := sp.Status.Capacity[ResourceDiskPoolByte]
		bytes, ok = storeQuan.AsInt64()
		if !ok {
			bytes = int64(math.Round(storeQuan.AsApproximateFloat64()))
		}
	}

	return bytes
}

// GetVgFreeBytes get free space of VolumeGroup in byte. Reserved space is used, therefore it is excluded.
// VG剩余空间, 单位是字节, 不包含保留LV空间
func (sp *StoragePool) GetVgFreeBytes() int64 {
	var freeDisk = sp.Status.VGFreeSize
	var freeBytes, ok = freeDisk.AsInt64()
	if !ok {
		freeBytes = int64(math.Round(freeDisk.AsApproximateFloat64()))
	}
	return freeBytes
}

// GetAvailableBytes get total available space, excluding reserved space.
// AvailableSpace = Total - Reserved
// 可分配的总空间 = VG总空间 - 保留空间
func (sp *StoragePool) GetAvailableBytes() int64 {
	var size = sp.GetVgTotalBytes()
	// minus reserved lvol
	for _, item := range sp.Spec.KernelLVM.ReservedLVol {
		size -= int64(item.SizeByte)
	}
	return size
}

// GetLocalStorageBytes get the current watermark of local storage in bytes
// func (sp *StoragePool) GetLocalStorageBytes_0() int64 {
// 	if val, has := sp.Labels[PoolLocalStorageBytesKey]; has {
// 		var hintLocalTotal int
// 		var err error

// 		hintLocalTotal, err = strconv.Atoi(val)
// 		if err != nil {
// 			klog.Error(err)
// 			return 0
// 		}

// 		return int64(hintLocalTotal)
// 	}

// 	return 0
// }

func (sp *StoragePool) IsSchedulable() bool {
	val, has := sp.Labels[PoolSchedulingStatusLabelKey]
	labelLocked := has && val == string(PoolSchedulingStatusLocked)
	statusNotReady := sp.Status.Status != PoolStatusReady

	return !labelLocked && !statusNotReady
}

func (sp *StoragePool) Mode() (mode PoolMode) {
	if sp.Spec.KernelLVM.Name != "" {
		mode = PoolModeKernelLVM
	}
	if sp.Spec.SpdkLVStore.Name != "" {
		mode = PoolModeSpdkLVStore
	}
	return
}
