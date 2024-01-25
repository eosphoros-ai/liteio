package client

import (
	"fmt"

	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"lite.io/liteio/pkg/util"
)

type Volume = v1.AntstorVolume
type StoragePool = v1.StoragePool
type Snapshot = v1.AntstorSnapshot

type PV struct {
	Namespace string
	Name      string
	UUID      string
	// Volume or DataContrl
	Type string
	// under object
	Volume     *v1.AntstorVolume
	DataContrl *v1.AntstorDataControl
}

type PVCreateOption struct {
	PvName      string
	Labels      map[string]string
	Annotations map[string]string
	HostNode    v1.NodeInfo
	Size        int64
	// volume engine: lvm or spdk
	VolumeType v1.VolumeType
	// volume posision
	PositionAdvice string

	PvType string
	// for data control
	RaidLevel  string
	EngineType string
	// for volume group
	MaxVolumes     int
	MaxVolumeSize  string
	MinVolumeSize  string
	SizeSymmetry   string
	AllowEmptyNode bool
}

type PvBaseIface interface {
	// GetPvByNameAndType(name, typ string) (pv PV, err error)
	GetPvByID(id string) (pv PV, err error)

	CreatePV(opt PVCreateOption) (id string, err error)

	DeletePV(id string) (err error)

	ResizePV(id string, size int64) (err error)
}

type PvAdvancedIface interface {
	// UpdatePvHostNode update the host node info of the PV
	// UpdatePvHostNode(volID, hostNodeID string) (err error)

	SetNodePublishParameters(req SetNodePublishParamRequest) (err error)
}

type PvIface interface {
	PvBaseIface
	PvAdvancedIface
}

type SnapshotIface interface {
	GetSnapshotByID(snapID string) (snapshot *Snapshot, err error)
	GetSnapshotByName(ns, name string) (snapshot *Snapshot, err error)
	CreateSnapshot(snap Snapshot) (snapID string, err error)
	DeleteSnapshot(snapID string) (err error)
}

type StoragePoolIface interface {
	GetStoragePoolByName(ns, name string) (sp *StoragePool, err error)
}

type AntstorClientIface interface {
	PvIface
	SnapshotIface
	StoragePoolIface
}

func (p *PV) GetSize() int64 {
	switch p.Type {
	case PvTypeVolume:
		return int64(p.Volume.Spec.SizeByte)
	case PvTypeVolumeGroup:
		return p.DataContrl.Spec.TotalSize
	}
	return 0
}

func (p *PV) GetTargetNodeId() string {
	switch p.Type {
	case PvTypeVolume:
		return p.Volume.Spec.TargetNodeId
	case PvTypeVolumeGroup:
		return p.DataContrl.Spec.TargetNodeId
	}
	return ""
}

func (p *PV) GetStatus() v1.VolumeStatus {
	switch p.Type {
	case PvTypeVolume:
		return p.Volume.Status.Status
	case PvTypeVolumeGroup:
		return p.DataContrl.Status.Status
	}
	return ""
}

func (p *PV) GetLabels() map[string]string {
	switch p.Type {
	case PvTypeVolume:
		return p.Volume.Labels
	case PvTypeVolumeGroup:
		return p.DataContrl.Labels
	}

	return nil
}

func (p *PV) GetAnnotations() map[string]string {
	switch p.Type {
	case PvTypeVolume:
		return p.Volume.Annotations
	case PvTypeVolumeGroup:
		return p.DataContrl.Annotations
	}

	return nil
}

func (p *PV) GetFsType() (fsType string) {
	var labels = p.GetLabels()

	if labels != nil {
		fsType = labels[v1.FsTypeLabelKey]
	}

	if fsType == "" {
		fsType = util.FileSystemXfs
	}

	return
}

func (p *PV) IsLocal() bool {
	switch p.Type {
	case PvTypeVolume:
		return p.Volume.Spec.HostNode.ID == p.Volume.Spec.TargetNodeId
	case PvTypeVolumeGroup:
		return p.DataContrl.Spec.EngineType == v1.PoolModeKernelLVM
	}
	return false
}

func (p *PV) IsLVM() bool {
	switch p.Type {
	case PvTypeVolume:
		return p.Volume.Spec.Type == v1.VolumeTypeKernelLVol
	case PvTypeVolumeGroup:
		return p.DataContrl.Spec.EngineType == v1.PoolModeKernelLVM
	}
	return false
}

func (p *PV) GetDevPath() string {
	switch p.Type {
	case PvTypeVolume:
		if p.Volume != nil && p.Volume.Spec.KernelLvol != nil {
			return p.Volume.Spec.KernelLvol.DevPath
		}
	case PvTypeVolumeGroup:
		if p.DataContrl != nil && p.DataContrl.Spec.LVM != nil {
			return fmt.Sprintf("/dev/%s/%s", p.DataContrl.Spec.LVM.VG, p.DataContrl.Spec.LVM.LVol)
		}
	}
	return ""
}

func (p *PV) GetSpdkTarget() *v1.SpdkTarget {
	switch p.Type {
	case PvTypeVolume:
		return p.Volume.Spec.SpdkTarget
	case PvTypeVolumeGroup:
		// TODO: check  nil
		return nil
	}
	return nil
}
