package pool

import (
	"lite.io/liteio/pkg/agent/pool/engine"
	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"lite.io/liteio/pkg/spdk"
)

/*
StoragePoolServiceIface
*/
type StoragePoolServiceIface interface {
	// PoolInfoGetterIface provides information of StoragePool
	PoolInfoGetterIface

	// PoolEngine returns a PoolEngine
	PoolEngine() engine.PoolEngineIface

	// SpdkService is for utilizing sdpk service
	SpdkService() spdk.SpdkServiceIface

	SpdkWatcher() *SpdkWatcher
	//
	Access() AccessIface
}

type PoolModeDetectorIface interface {
	DetectMode() (mode v1.PoolMode, nodeInfo v1.NodeInfo, err error)
}

type PoolInfoGetterIface interface {
	Mode() (mode v1.PoolMode)
	GetStoragePool() *v1.StoragePool
}

/*
type PoolStatusGetterIface interface {
	GetTotalAndFreeSize() (total uint64, free uint64, err error)
}

type VolumeServiceIface interface {
	PoolStatusGetterIface
	CreateVolume(req CreateVolumeRequest) (resp CreateVolumeResponse, err error)
	DeleteVolume(volName string) (err error)
	CreateSnapshot(req CreateSnapshotRequest) (err error)
	RestoreSnapshot(snapshotName string) (err error)
	ExpandVolume(req ExpandVolumeRequest) (err error)
}

type CreateVolumeRequest struct {
	// for LVM and SpdkLVS
	VolName  string
	SizeByte uint64
	// optional; for LVM
	FsType string
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

*/
