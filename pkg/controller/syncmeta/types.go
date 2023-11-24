package syncmeta

import (
	"encoding/json"
	"strings"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"k8s.io/klog/v2"
)

type ObjectMetaBrief struct {
	ID          int    `xorm:"id"`
	ClusterName string `xorm:"cluster_name"`
	Name        string `xorm:"name"`
	Status      string `xorm:"status"`
	CreatedAt   int    `xorm:"created_at"`
	UpdatedAt   int    `xorm:"updated_at"`
	DeletedAt   int    `xorm:"deleted_at"`
}

type StoragePoolBriefMapping = ObjectMetaBrief
type AntstorVolumeBriefMapping = ObjectMetaBrief

type StoragePoolMapping struct {
	// 主键
	ID           int    `xorm:"pk autoincr 'id'"`
	ClusterName  string `xorm:"cluster_name"`
	Name         string `xorm:"name"`
	VGType       string `xorm:"vg_type"`
	VGName       string `xorm:"vg_name"`
	ReservedVol  string `xorm:"reserved_vol"`
	TotalSize    uint64 `xorm:"total_size"`
	FreeSize     uint64 `xorm:"free_size"`
	NodeID       string `xorm:"node_id"`
	NodeIP       string `xorm:"node_ip"`
	NodeHostname string `xorm:"node_hostname"`
	Status       string `xorm:"status"`
	CreatedAt    int    `xorm:"created_at"`
	UpdatedAt    int    `xorm:"updated_at"`
	DeletedAt    int    `xorm:"deleted_at"`
}

type AntstorVolumeMapping struct {
	// 主键
	ID               int    `xorm:"pk autoincr 'id'"`
	ClusterName      string `xorm:"cluster_name"`
	Name             string `xorm:"name"`
	UUID             string `xorm:"uid"`
	Labels           string `xorm:"labels"`
	PvcNS            string `xorm:"pvc_ns"`
	PvcName          string `xorm:"pvc_name"`
	LvolDevPath      string `xorm:"lvol_dev_path"`
	LvolName         string `xorm:"lvol_name"`
	IsThin           int    `xorm:"is_thin"`
	PositionAdvice   string `xorm:"position_advice"`
	Size             uint64 `xorm:"size"`
	TargetNodeID     string `xorm:"t_node_id"`
	HostNodeID       string `xorm:"h_node_id"`
	HostNodeIP       string `xorm:"h_node_ip"`
	HostNodeHostname string `xorm:"h_node_hostname"`

	SpdkSubsysNQN string `xorm:"spdk_subsys_nqn"`
	SpdkSvcID     string `xorm:"spdk_svc_id"`
	SpdkSN        string `xorm:"spdk_sn"`
	SpdkTransType string `xorm:"spdk_trans_type"`
	SpdkBdevName  string `xorm:"spdk_bdev_name"`
	SpdkNsUUID    string `xorm:"spdk_ns_uuid"`
	SpdkAddress   string `xorm:"spdk_address"`

	CSIStagingPath string `xorm:"csi_staging_path"`
	CSIPublishPath string `xorm:"csi_publish_path"`

	PodNS   string `xorm:"pod_ns"`
	PodName string `xorm:"pod_name"`

	Status string `xorm:"status"`

	CreatedAt int `xorm:"created_at"`
	UpdatedAt int `xorm:"updated_at"`
	DeletedAt int `xorm:"deleted_at"`
}

type AntstorVolumeFullMapping struct {
	AntstorVolumeMapping    `xorm:"extends"`
	AntstorVolumeExtMapping `xorm:"extends"`
}

type AntstorVolumeExtMapping struct {
	// 主键
	ID        int    `xorm:"pk autoincr 'id'"`
	VolID     int    `xorm:"vol_id"`
	ObCluster string `xorm:"ob_cluster"`
	ObZone    string `xorm:"ob_zone"`
	CreatedAt int    `xorm:"created_at"`
	UpdatedAt int    `xorm:"updated_at"`
	DeletedAt int    `xorm:"deleted_at"`
}

type AntstorSnapshotMapping struct {
	// 主键
	ID                    int    `xorm:"pk autoincr 'id'"`
	Name                  string `xorm:"name"`
	ClusterName           string `xorm:"cluster_name"`
	OriginVolName         string `xorm:"origin_vol_name"`
	OriginVolNs           string `xorm:"origin_vol_ns"`
	OriginVolTargetNodeID string `xorm:"origin_vol_t_node_id"`
	VolType               string `xorm:"vol_type"`
	LvolDevPath           string `xorm:"lvol_dev_path"`
	LvolName              string `xorm:"lvol_name"`
	SpdkLvsName           string `xorm:"spdk_lvs_name"`
	SpdkSnapName          string `xorm:"spdk_snap_name"`
	Size                  int64  `xorm:"size"`
	Status                string `xorm:"status"`
	CreatedAt             int    `xorm:"created_at"`
	UpdatedAt             int    `xorm:"updated_at"`
	DeletedAt             int    `xorm:"deleted_at"`
}

func (spm *StoragePoolMapping) TableName() string {
	return "storage_pool"
}

func (spm *AntstorVolumeMapping) TableName() string {
	return "antstor_volume"
}

func (asm *AntstorSnapshotMapping) TableName() string {
	return "antstor_snapshot"
}

func (avem *AntstorVolumeExtMapping) TableName() string {
	return "antstor_volume_ext"
}

func ToStoragePoolMapping(clusterName string, sp *v1.StoragePool) (spm *StoragePoolMapping) {
	var freeVg int64
	var reservedVol []byte
	var vgType, vgName string
	freeVg = int64(sp.Status.VGFreeSize.AsApproximateFloat64())
	reservedVol, _ = json.Marshal(sp.Spec.KernelLVM.ReservedLVol)

	if sp.Spec.KernelLVM.Name != "" {
		vgType = string(v1.VolumeTypeKernelLVol)
		vgName = sp.Spec.KernelLVM.Name
	}

	if sp.Spec.SpdkLVStore.Name != "" {
		vgType = string(v1.VolumeTypeSpdkLVol)
		vgName = sp.Spec.SpdkLVStore.Name
	}

	spm = &StoragePoolMapping{
		ClusterName: clusterName,
		Name:        sp.Name,
		// TODO: could be VolumeTypeSpdkLVol
		VGType:      vgType,
		VGName:      vgName,
		ReservedVol: string(reservedVol),
		TotalSize:   uint64(sp.GetVgTotalBytes()),
		FreeSize:    uint64(freeVg),

		NodeID:       sp.Spec.NodeInfo.ID,
		NodeIP:       sp.Spec.NodeInfo.IP,
		NodeHostname: sp.Spec.NodeInfo.Hostname,
		Status:       string(sp.Status.Status),
	}
	return
}

func ToAntstorVolumeMapping(clusterName string, vol *v1.AntstorVolume) (avm *AntstorVolumeMapping) {
	var (
		devPath, lvolName string
		spdk              v1.SpdkTarget
		csi               v1.CSINodePubParams
		podName, podNS    string
		pvcName, pvcNS    string
		labelsJSON        string = "{}"
		hostNode          v1.NodeInfo
	)
	if vol.Spec.HostNode != nil {
		hostNode = *vol.Spec.HostNode
	}
	if vol.Spec.KernelLvol != nil {
		devPath = vol.Spec.KernelLvol.DevPath
		lvolName = vol.Spec.KernelLvol.Name
	}
	if vol.Spec.SpdkLvol != nil {
		lvolName = vol.Spec.SpdkLvol.LvsName
	}

	if vol.Spec.SpdkTarget != nil {
		spdk = *vol.Spec.SpdkTarget
	}
	if vol.Status.CSINodePubParams != nil {
		csi = *vol.Status.CSINodePubParams
		// copy map
		if csi.CSIVolumeContext == nil {
			csi.CSIVolumeContext = make(map[string]string)
		}
		for k, v := range vol.Status.CSINodePubParams.CSIVolumeContext {
			csi.CSIVolumeContext[k] = v
		}

		// read from map
		podName = vol.Status.CSINodePubParams.CSIVolumeContext[v1.VolumeContextKeyPodName]
		podNS = vol.Status.CSINodePubParams.CSIVolumeContext[v1.VolumeContextKeyPodNS]
	}

	if vol.Labels != nil {
		pvcName = vol.Labels[v1.VolumeContextKeyPvcName]
		pvcNS = vol.Labels[v1.VolumeContextKeyPvcNS]
		// TODO: need to prevent concurrent read
		bs, _ := json.Marshal(vol.Labels)
		labelsJSON = string(bs)
	}

	avm = &AntstorVolumeMapping{
		ClusterName: clusterName,
		Name:        vol.Name,
		UUID:        vol.Spec.Uuid,
		Labels:      labelsJSON,

		PvcNS:   pvcNS,
		PvcName: pvcName,

		LvolDevPath:      devPath,
		LvolName:         lvolName,
		IsThin:           0,
		PositionAdvice:   string(vol.Spec.PositionAdvice),
		Size:             vol.Spec.SizeByte,
		TargetNodeID:     vol.Spec.TargetNodeId,
		HostNodeID:       hostNode.ID,
		HostNodeHostname: hostNode.Hostname,
		HostNodeIP:       hostNode.IP,

		SpdkSubsysNQN: spdk.SubsysNQN,
		SpdkSvcID:     spdk.SvcID,
		SpdkSN:        spdk.SerialNum,
		SpdkTransType: spdk.TransType,
		SpdkBdevName:  spdk.BdevName,
		SpdkNsUUID:    spdk.NSUUID,
		SpdkAddress:   spdk.Address,

		CSIStagingPath: csi.StagingTargetPath,
		CSIPublishPath: csi.TargetPath,
		PodNS:          podNS,
		PodName:        podName,
		Status:         string(vol.Status.Status),
	}
	return
}

func ToAntstorVolumeExtMapping(vol *v1.AntstorVolume, volID int) (avem *AntstorVolumeExtMapping) {
	volLabels := vol.GetLabels()
	if len(volLabels) == 0 {
		return
	}
	// dataHolder format is like "ob.$cluster_name.$zone_name"
	dataHolder, has := volLabels["antstor.csi.alipay.com/data-holder"]
	if !has || len(dataHolder) == 0 {
		klog.Errorf("missing data-holder of vol %s", vol.Name)
		return
	}
	dataHolderArr := strings.SplitN(dataHolder, ".", 3)
	if len(dataHolderArr) < 3 {
		klog.Errorf("invalid data-holder %s, vol %s", dataHolder, vol.Name)
		return
	}
	avem = &AntstorVolumeExtMapping{
		VolID:     volID,
		ObCluster: dataHolderArr[1],
		ObZone:    dataHolderArr[2],
	}
	return
}

func ToAntstorSnapshotMapping(clusterName string, as *v1.AntstorSnapshot) (asm *AntstorSnapshotMapping) {
	asm = &AntstorSnapshotMapping{
		ClusterName:           clusterName,
		Name:                  as.Name,
		OriginVolName:         as.Spec.OriginVolName,
		OriginVolNs:           as.Spec.OriginVolNamespace,
		OriginVolTargetNodeID: as.Spec.OriginVolTargetNodeID,
		VolType:               string(as.Spec.VolType),
		LvolDevPath:           as.Spec.KernelLvol.DevPath,
		LvolName:              as.Spec.KernelLvol.Name,
		SpdkLvsName:           as.Spec.SpdkLvol.LvsName,
		SpdkSnapName:          as.Spec.SpdkLvol.Name,
		Size:                  as.Spec.Size,
		Status:                string(as.Status.Status),
	}
	return
}
