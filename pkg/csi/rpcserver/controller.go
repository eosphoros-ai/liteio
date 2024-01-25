package rpcserver

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"lite.io/liteio/pkg/agent/config"
	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"lite.io/liteio/pkg/controller/kubeutil"
	"lite.io/liteio/pkg/csi/client"
	"lite.io/liteio/pkg/csi/driver"
	"lite.io/liteio/pkg/util/misc"
)

var _ csi.ControllerServer = &ControllerServer{}

const (
	pvcNameKey      = "csi.storage.k8s.io/pvc/name"
	pvcNamespaceKey = "csi.storage.k8s.io/pvc/namespace"
	pvNameKey       = "csi.storage.k8s.io/pv/name"

	// keys in volume_context
	podNameKey      = "csi.storage.k8s.io/pod.name"
	podNamespaceKey = "csi.storage.k8s.io/pod.namespace"
	podUuidKey      = "csi.storage.k8s.io/pod.uid"

	pvcNameKeyForLabel      = "csi.storage.k8s.io/pvc-name"
	pvcNamespaceKeyForLabel = "csi.storage.k8s.io/pvc-namespace"

	dataHolderKey   = "antstor.csi.alipay.com/data-holder"
	selectedNodeKey = "volume.kubernetes.io/selected-node"

	// CSI CreateVolumeRequest Context key, MustLocal or MustRemote or PreferLocal or PreferRemote
	diskLocationKey = "positionAdvice"
	// CSI CreateVolumeRequest Context key, same as volumeTypeAnnoKey
	volumeTypeKey = "volumeType"
	// CSI CreateVolumeRequest Context key, specifing filesystem type, e.g. xfs or ext4
	fsTypeKey = "fsType"
	// CSI CreateVolumeRequest Context key, specifing mkfs arguments
	mkfsParamsKey = "obnvmf/mkfsParams"

	// CSI CreateVolumeRequest Context key for DataControl and VoluemGroup
	// value is Volume or VolumeGroup
	pvTypeKey             = "obnvmf/pv-type"
	raidLevelKey          = "datacontrol/raid-level"
	engineTypeKey         = "datacontrol/engine-type"
	volGroupSymmetryKey   = "volgroup/size-symmetry"
	volGroupMinSizeKey    = "volgroup/min-size"
	volGroupMaxSizeKey    = "volgroup/max-size"
	volGroupMaxVolumesKey = "volgroup/max-volumes"
	volGroupAllowEmptyKey = "volgroup/allow-empty-node"

	// Volume Annotation key, value is KernelLVM or SpdkLVS or Flexible
	volumeTypeAnnoKey = "obnvmf/volume-type"

	// Volume Annotation key, which specifies pod runtime class
	containerTypeKey = "obnvmf/pod-runtime-class"
	// value of containerTypeKey, which indicates the volume is used by rund
	containerTypeForKata = "rund"

	// Volume Annotation key
	spdkConnectModeKey = "obnvmf/spdk-conn-mode"
	// value of spdkConnectModeKey, which indicates that guest kernel directly connect spdk target
	spdkConnectModeGuestKernelDirect = "guest-direct"

	// Volume Annotation key
	volumeModeKey = "obnvmf/volume-mode"
	// value of volumeModeKey, volume is mount as Block or Filesystem
	volumeModeBlock      = "Block"
	volumeModeFilesystem = "Filesystem"

	// PVC Annotation key, same as position advice of the volume
	volumePositionAdviceKey = "custom.k8s.alipay.com/sds-position-advice"

	// Volume Annotation key, value is xfs or ext4
	// fsTypeLabelKey = "obnvmf/fs-type"

	volContextKeySkipUpdatePublishParam = "skip-save-context"
)

type ControllerServer struct {
	driver  *driver.CSIDriver
	cli     client.AntstorClientIface
	locks   *misc.ResourceLocks
	kubeCli kubernetes.Interface
	// detachLimiter common.RetryLimiter
}

// NewControllerServer
// Create controller server
func NewControllerServer(driver *driver.CSIDriver, cli client.AntstorClientIface, kubeCli kubernetes.Interface) *ControllerServer {
	return &ControllerServer{
		driver:  driver,
		cli:     cli,
		locks:   misc.NewResourceLocks(),
		kubeCli: kubeCli,
	}
}

// CreateVolume creates a Voluem. This operation MUST be idempotent
// This operation MAY create three types of volumes:
// 1. Empty volumes: CREATE_DELETE_VOLUME
// 2. Restore volume from snapshot: CREATE_DELETE_VOLUME and CREATE_DELETE_SNAPSHOT
// 3. Clone volume: CREATE_DELETE_VOLUME and CLONE_VOLUME
// csi.CreateVolumeRequest: Name 				+Required
//
//	CapabilityRange		+Required
func (cs *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	klog.Infof("CreateVolume Req=%s", req.String())
	if req.GetCapacityRange() == nil {
		return nil, status.Error(codes.InvalidArgument, "CreateVolumeRequest.CapacityRange is nil")
	}

	var (
		// get pvc ns/name from req.Parameters [pvcNamespaceKey] [pvcNameKey]; get pvc by name; get node-name from pvc annotation volume.kubernetes.io/selected-node
		// pvc 标签上必须包含 数据副本信息，例如 x 集群， y Zone; dataHolderKey=ob.cluster1.zonea
		pvcName, pvcNs string

		// pod's selected node from Annotations of PVC
		nodeName string
		// data holder from Annotations of PVC
		// dataHolder string
		fsType, volType string

		err error
		opt client.PVCreateOption
		// attributes for AntstroVolume
		volLabels      = make(map[string]string)
		volAnnotations = make(map[string]string)
	)

	pvcName = req.Parameters[pvcNameKey]
	pvcNs = req.Parameters[pvcNamespaceKey]
	fsType = req.Parameters[fsTypeKey]
	if fsType == "" {
		fsType = "xfs"
	}
	volType = req.Parameters[volumeTypeKey]
	if volType == "" {
		volType = string(v1.VolumeTypeFlexible)
	}
	opt.VolumeType = v1.VolumeType(volType)
	// NoPreference is empty string
	opt.PositionAdvice = req.Parameters[diskLocationKey]
	opt.Size = req.GetCapacityRange().RequiredBytes
	opt.PvName = req.Name
	opt.PvType = req.Parameters[pvTypeKey]

	volLabels[v1.FsTypeLabelKey] = fsType
	volLabels[v1.VolumePVNameLabelKey] = req.Name
	volLabels[pvcNameKeyForLabel] = pvcName
	volLabels[pvcNamespaceKeyForLabel] = pvcNs
	volAnnotations[v1.FsTypeLabelKey] = fsType

	opt.RaidLevel = req.Parameters[raidLevelKey]
	opt.EngineType = req.Parameters[engineTypeKey]
	opt.SizeSymmetry = req.Parameters[volGroupSymmetryKey]
	opt.MaxVolumeSize = req.Parameters[volGroupMaxSizeKey]
	opt.MinVolumeSize = req.Parameters[volGroupMinSizeKey]
	if val, has := req.Parameters[volGroupMaxVolumesKey]; has {
		opt.MaxVolumes, _ = strconv.Atoi(val)
	}
	// defualt is true
	opt.AllowEmptyNode = true
	if val, has := req.Parameters[volGroupAllowEmptyKey]; has {
		opt.AllowEmptyNode = val == "true"
	}

	// get volume content source info
	if req.VolumeContentSource.GetSnapshot() != nil {
		id := req.VolumeContentSource.GetSnapshot().SnapshotId
		snap, err := cs.cli.GetSnapshotByID(id)
		if err != nil {
			klog.Error(err)
			return nil, status.Error(codes.Internal, err.Error())
		}
		if snap.Status.Status != v1.SnapshotStatusReady {
			err = fmt.Errorf("snapshot has not been ready yet, status %s", snap.Status.Status)
			klog.Error(err)
			return nil, status.Error(codes.Internal, err.Error())
		}
		volLabels[v1.VolumeSourceSnapNameLabelKey] = snap.Name
		volLabels[v1.VolumeSourceSnapNamespaceLabelKey] = snap.Namespace
		volAnnotations[v1.PoolLabelSelectorKey] = fmt.Sprintf("%s=%s", v1.PoolLabelsNodeSnKey, snap.Spec.OriginVolTargetNodeID)
	}

	if pvcNs != "" && pvcName != "" {
		pvc, err := cs.kubeCli.CoreV1().PersistentVolumeClaims(pvcNs).Get(context.Background(), pvcName, metav1.GetOptions{})
		if err != nil {
			klog.Error(err)
			return nil, status.Error(codes.Internal, err.Error())
		}

		nodeName = pvc.Annotations[selectedNodeKey]
		// data-holder 数据拥有者， 用于辨别相同数据, 格式是 存储类型(应用名).存储区域路径(应用内定义的存储区域) ，例如ob是 ob.集群名.zone名,
		if dataHolder := pvc.Annotations[dataHolderKey]; dataHolder != "" {
			volLabels[v1.VolumeDataHolderKey] = dataHolder
		}
		// postion advice
		if val, has := pvc.Annotations[volumePositionAdviceKey]; has {
			opt.PositionAdvice = val
		}
		// snap size
		if snapSize, has := pvc.Annotations[v1.SnapshotReservedSpaceAnnotationKey]; has {
			volAnnotations[v1.SnapshotReservedSpaceAnnotationKey] = snapSize
		}
		// volume type
		if typ, has := pvc.Annotations[volumeTypeAnnoKey]; has {
			opt.VolumeType = v1.VolumeType(typ)
		}

		klog.Infof("PVC ResourceVersion %s, Annotations %+v", pvc.ResourceVersion, pvc.Annotations)

		// copy PVC Annotations whose key starting with "obnvmf/" to volume's annotations
		for key, val := range pvc.Annotations {
			if strings.HasPrefix(key, "obnvmf/") {
				volAnnotations[key] = val
			}
		}
	}

	// set HostNode info
	if nodeName != "" {
		// TODO: config
		cfg := config.NodeInfoKeys{}
		config.SetNodeInfoDefaults(&cfg)
		opt.HostNode, err = kubeutil.NewKubeNodeInfoGetter(cs.kubeCli).GetByNodeID(nodeName, kubeutil.NodeInfoOption(cfg))
		if err != nil {
			klog.Error(err)
			return nil, status.Error(codes.Internal, err.Error())
		}
		opt.HostNode.ID = nodeName
		if opt.HostNode.Labels != nil {
			opt.HostNode.Hostname = opt.HostNode.Labels[kubeutil.K8SLabelKeyHostname]
		}
	}

	// if volume is MustLocal, add a Topology to PV
	var topo []*csi.Topology
	if opt.PositionAdvice == string(v1.MustLocal) {
		topo = append(topo, &csi.Topology{
			Segments: map[string]string{
				kubeutil.K8SLabelKeyHostname: opt.HostNode.Hostname,
			},
		})
		klog.Infof("volume %s add topo %#v", opt.PvName, topo)
	}

	opt.Labels = volLabels
	opt.Annotations = volAnnotations

	volID, err := cs.cli.CreatePV(opt)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volID,
			CapacityBytes: opt.Size,
			VolumeContext: req.GetParameters(),
			ContentSource: req.GetVolumeContentSource(),
			// AccessibleTopology indicates where this PV is accessible.
			// if a Pod wants to use this PV, kube scheduler will respcet this Topology as node selector
			AccessibleTopology: topo,
		},
	}

	return resp, nil
}

// DeleteVolume deletes the volume. This operation MUST be idempotent
// volume id is REQUIRED in csi.DeleteVolumeRequest
func (cs *ControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	klog.Infof("DeleteVolume Req=%s", req.String())
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "DeleteVolumeRequest is invalid")
	}

	// DeleteVolume is idempotent in node-disk-controller RPC
	err := cs.cli.DeletePV(req.VolumeId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &csi.DeleteVolumeResponse{}, nil
}

func (cs *ControllerServer) ControllerGetVolume(context.Context, *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {

	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerPublishVolume attaches the volume to the node
// volume id 			+ Required
// node id				+ Required
// volume capability 	+ Required
// readonly			+ Required (This field is NOT provided when requesting in Kubernetes)
func (cs *ControllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	klog.Infof("ControllerPublishVolume Req=%s", req.String())

	if req.VolumeId == "" || req.NodeId == "" || req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolumeRequest is invalid")
	}

	// nvmf does not need AttachVolume

	return &csi.ControllerPublishVolumeResponse{}, nil
}

// ControllerUnpublishVolume detaches the volume from the node. It MUST be idempotent
// csi.ControllerUnpublishVolumeRequest:
// volume id	+Required
func (cs *ControllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	klog.Infof("ControllerUnpublishVolume req=%s", req.String())

	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerUnpublishVolumeRequest is invalid")
	}

	// nvmf does not need DetachVolume

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

// This operation MUST be idempotent
// volume id 			+ Required
// volume capability 	+ Required
func (cs *ControllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	klog.Infof("ValidateVolumeCapabilities req=%s", req.String())

	if req.VolumeId == "" || req.VolumeCapabilities == nil {
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilitiesRequest is invalid")
	}

	_, err := cs.cli.GetPvByID(req.VolumeId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	for _, item := range req.GetVolumeCapabilities() {
		klog.Infof("Volume capability %s", item.String())
	}

	resp := &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: req.GetVolumeCapabilities(),
		},
	}

	return resp, nil
}

// ControllerExpandVolume allows the CO to expand the size of a volume
// volume id is REQUIRED in csi.ControllerExpandVolumeRequest
// capacity range is REQUIRED in csi.ControllerExpandVolumeRequest
func (cs *ControllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	klog.Infof("ControllerExpandVolume req=%s", req.String())

	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerExpandVolume.VolumeId is empty")
	}

	capRange := req.GetCapacityRange()
	if capRange == nil {
		return nil, status.Error(codes.InvalidArgument, "ControllerExpandVolume.CapacityRange is nil")
	}

	pv, err := cs.cli.GetPvByID(req.VolumeId)
	if err != nil {
		var errCode codes.Code
		if errors.IsNotFound(err) {
			errCode = codes.NotFound
		} else {
			errCode = codes.Internal
		}
		klog.Error(err)
		return nil, status.Error(errCode, err.Error())
	}

	volSize := pv.GetSize()
	if capRange.RequiredBytes <= volSize {
		err = fmt.Errorf("target size is %d, current size is %d, only expasion is allowed", capRange.RequiredBytes, volSize)
		klog.Error(err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// check whether StoragePool has enough free space to expend the volume
	tgtNodeId := pv.GetTargetNodeId()
	sp, err := cs.cli.GetStoragePoolByName(pv.Namespace, tgtNodeId)
	if err != nil {
		klog.Error(err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	freeBytes := sp.GetVgFreeBytes()
	if freeBytes < capRange.RequiredBytes-volSize {
		err = fmt.Errorf("storagepool %s has no enough free space to expand the volume, free space %d, request size %d, original size %d", tgtNodeId, freeBytes, capRange.RequiredBytes, volSize)
		klog.Error(err)
		return nil, status.Error(codes.OutOfRange, err.Error())
	}
	klog.Infof("free space %d, request size %d, original size %d", freeBytes, capRange.RequiredBytes, volSize)

	err = cs.cli.ResizePV(req.VolumeId, int64(capRange.RequiredBytes))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         capRange.RequiredBytes,
		NodeExpansionRequired: true,
	}

	return resp, nil
}

func (cs *ControllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	klog.Infof("ListVolumes req=%s", req.String())
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// CreateSnapshot allows the CO to create a snapshot.
// This operation MUST be idempotent.
// 1. If snapshot successfully cut and ready to use, the plugin MUST reply 0 OK.
// 2. If an error occurs before a snapshot is cut, the plugin SHOULD reply a corresponding error code.
// 3. If snapshot successfully cut but still being precessed,
// the plugin SHOULD return 0 OK and ready_to_use SHOULD be set to false.
// Source volume id is REQUIRED
// Snapshot name is REQUIRED
func (cs *ControllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.
	CreateSnapshotResponse, error) {

	klog.Infof("CreateSnapshot Req=%s", req.String())

	// parameters validation
	if req.GetSourceVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "CreateSnapshotRequest.SourceVolumeId is nil")
	}
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "CreateSnapshotRequest.Name is nil")
	}

	// check volume status
	pv, err := cs.cli.GetPvByID(req.GetSourceVolumeId())
	if err != nil {
		klog.Error(err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if pv.Type != client.PvTypeVolume || pv.Volume == nil {
		return nil, status.Error(codes.InvalidArgument, "only support Volume snapshot")
	}
	vol := pv.Volume

	if vol.Status.Status != v1.VolumeStatusReady {
		err = fmt.Errorf("snapshot shoule be created after original volume is ready, volume status %s", vol.Status.Status)
		klog.Error(err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	// check snapshot existence
	snapshot, err := cs.cli.GetSnapshotByName(v1.DefaultNamespace, req.GetName())
	if err != nil && !errors.IsNotFound(err) {
		klog.Error(err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	// found snapshot
	if err == nil && snapshot != nil {
		resp := &csi.CreateSnapshotResponse{
			Snapshot: &csi.Snapshot{
				SizeBytes:      int64(vol.Spec.SizeByte),
				SnapshotId:     snapshot.Labels[v1.SnapUuidLabelKey],
				SourceVolumeId: vol.Spec.Uuid,
				CreationTime:   timestamppb.New(time.Now()),
			},
		}
		if snapshot.Status.Status == v1.SnapshotStatusReady {
			resp.Snapshot.ReadyToUse = true
		} else {
			resp.Snapshot.ReadyToUse = false
		}
		return resp, nil
	}

	// create AntstorSnapshot
	snapLabels := make(map[string]string)
	snapLabels[v1.OriginVolumeNameLabelKey] = vol.Name
	snapLabels[v1.OriginVolumeNamespaceLabelKey] = vol.Namespace
	snap := v1.AntstorSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   v1.DefaultNamespace,
			Name:        req.GetName(),
			Labels:      snapLabels,
			Annotations: make(map[string]string), // todo
		},
		Spec: v1.AntstorSnapshotSpec{
			// KernelLvol/SpdkLvol will be set in agent
			// OriginVolTargetNodeID will be set in node-disk-controller
			VolType:            vol.Spec.Type,
			Size:               int64(vol.Spec.SizeByte),
			OriginVolName:      vol.Name,
			OriginVolNamespace: vol.Namespace,
		},
		Status: v1.AntstorSnapshotStatus{
			Status: v1.SnapshotStatusCreating,
		},
	}
	snapID, err := cs.cli.CreateSnapshot(snap)
	if err != nil {
		klog.Error(err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	if snapID == "" {
		klog.Errorf("uuid of snapshot %s is empty", req.Name)
		return nil, status.Error(codes.Internal, err.Error())
	}
	resp := &csi.CreateSnapshotResponse{
		Snapshot: &csi.Snapshot{
			SizeBytes:      int64(vol.Spec.SizeByte),
			SnapshotId:     snapID,
			SourceVolumeId: vol.Spec.Uuid,
			CreationTime:   timestamppb.New(time.Now()),
			ReadyToUse:     false,
		},
	}

	return resp, nil
}

// DeleteSnapshot allows the CO to delete a snapshot.
// This operation MUST be idempotent.
// Snapshot id is REQUIRED
func (cs *ControllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse,
	error) {
	klog.Infof("DeleteSnapshot Req=%s", req.String())
	if req.SnapshotId == "" {
		return nil, status.Error(codes.InvalidArgument, "DeleteSnapshotRequest is invalid")
	}

	err := cs.cli.DeleteSnapshot(req.SnapshotId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &csi.DeleteSnapshotResponse{}, nil
}

func (cs *ControllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	klog.Infof("ListSnapshots Req=%s", req.String())
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) ControllerGetCapabilities(ctx context.Context,
	req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: cs.driver.GetControllerCapability(),
	}, nil
}
