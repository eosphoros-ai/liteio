package engine

import (
	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
)

type VolumeServiceIface interface {
	CreateVolume(req CreateVolumeRequest) (resp CreateVolumeResponse, err error)
	DeleteVolume(volName string) (err error)
	GetVolume(volName string) (vol VolumeInfo, err error)
	CreateSnapshot(req CreateSnapshotRequest) (err error)
	RestoreSnapshot(snapshotName string) (err error)
	ExpandVolume(req ExpandVolumeRequest) (err error)
}

type PoolingInfoIface interface {
	PoolInfo(poolName string) (info StaticInfo, err error)
	TotalAndFreeSize() (total uint64, free uint64, err error)
}

type PoolEngineIface interface {
	PoolingInfoIface
	VolumeServiceIface
}

type VolumeInfo struct {
	Type     v1.VolumeType
	LvmLV    *v1.KernelLVol
	SpdkLvol *SpdkLvolBdev
}

type SpdkLvolBdev struct {
	Lvol     v1.SpdkLvol
	SizeByte uint64
}

type StaticInfo struct {
	LVM *v1.KernelLVM
	LVS *v1.SpdkLVStore
}

type CreateVolumeRequest struct {
	// for LVM and SpdkLVS
	VolName  string
	SizeByte uint64
	// FsType to mkfs. Optional for LVM
	FsType string
	// LvLayout of lv to create. Optional for LVM
	LvLayout v1.LVLayout
}

type CreateVolumeResponse struct {
	// for SpdkLVS
	UUID string
	// for LVM
	DevPath string
}

type CreateSnapshotRequest struct {
	SnapshotName string
	OriginName   string
	SizeByte     uint64
}

type ExpandVolumeRequest struct {
	VolName    string
	TargetSize uint64
	OriginSize uint64
}
