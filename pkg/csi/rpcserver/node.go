package rpcserver

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	"k8s.io/mount-utils"
	"k8s.io/utils/exec"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/csi/client"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/csi/driver"
	spdkclient "code.alipay.com/dbplatform/node-disk-controller/pkg/spdk/jsonrpc/client"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/spdk/jsonrpc/nvme"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/kata"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/misc"
	mkfs "code.alipay.com/dbplatform/node-disk-controller/pkg/util/mount"
)

const (
	nvmeClientFilePath = "/home/admin/nvmeof/bin/nvme"
)

type NodeServer struct {
	driver  *driver.CSIDriver
	mounter *mount.SafeFormatAndMount
	locks   *misc.ResourceLocks
	cli     client.AntstorClientIface
}

var _ csi.NodeServer = &NodeServer{}

// NewNodeServer creates a node server
func NewNodeServer(driver *driver.CSIDriver, mnt *mount.SafeFormatAndMount, cli client.AntstorClientIface) *NodeServer {
	return &NodeServer{
		driver:  driver,
		cli:     cli,
		mounter: mnt,
		locks:   misc.NewResourceLocks(),
	}
}

// NodeStageVolume partition and format the disk and mount the disk on a node global directory. It MUST be idempotent
// csi.NodeStageVolumeRequest: 	volume id			+ Required
//
//	stage target path	+ Required
//	volume capability	+ Required
func (ns *NodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	klog.Infof("NodeStageVolume req=%s", req.String())

	if req.VolumeId == "" || req.StagingTargetPath == "" || req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "NodeStageVolumeRequest is invalid")
	}

	var (
		targetPath = req.GetStagingTargetPath()
		// check if the request is Block mode
		isBlockMode = req.GetVolumeCapability().GetBlock() != nil
		err         error
		pv          client.PV
	)

	// 1. Create dir of target path. This is the global directory to mount the volume
	// check if target path exists
	if hasTgtFile, err := misc.FileExists(targetPath); !hasTgtFile {
		err = os.MkdirAll(targetPath, 0750)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	} else {
		klog.Errorf("TargetPath %s maybe exist, %t , %+v", targetPath, hasTgtFile, err)
	}

	// 2. Update host node info of the volume; 这个请求是从 pod 所在node 发出，所以可以确保 host node 是正确的。
	// 因为存在机头漂移的情况，所以 host node 需要更新。如果 缺少target服务，还需要在 controller 发起创建 SPDK target。
	// TODO: update PV HostNode info

	klog.Infof("Try to find volume status, id=%s", req.VolumeId)
	pv, err = ns.cli.GetPvByID(req.VolumeId)
	if err != nil {
		klog.Errorf("cannot find volume by id %s, %+v", req.VolumeId, err)
		err = fmt.Errorf("cannot find volume by id: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	if pv.GetStatus() != v1.VolumeStatusReady {
		klog.Infof("Volume status is not ready, wait for it. id=%s", req.VolumeId)
		return nil, status.Error(codes.Internal, fmt.Sprintf("volume %s status is %s", req.VolumeId, pv.GetStatus()))
	}

	klog.Infof("volume is ready, id=%s", req.VolumeId)

	// 判断是否 远程盘+ guest kernel 直连SPDK模式
	/*
		if !usingLocalDisk && vol.Labels[spdkConnectModeKey] == spdkConnectModeGuestKernelDirect {
			// must specify rund
			if vol.Labels[containerTypeKey] == containerTypeForKata {
				// write config.json to targetPath
				cfgFile := kata.GetConfigFilePath(targetPath)
				err = kata.WriteConfigFileForKataSpdkDirectConnect(cfgFile, fsType, vol.Spec.SpdkTarget)
				if err != nil {
					return nil, status.Error(codes.Internal, err.Error())
				}
			} else {
				return nil, status.Error(codes.Internal, "volume lacks label: obnvmf/pod-runtime-class=rund")
			}

			return &csi.NodeStageVolumeResponse{}, nil
		}
	*/

	var (
		// 判断是否是本地磁盘, targetNode 是否等于 hostNode
		isLocalDisk  = pv.IsLocal()
		isLVM        = pv.IsLVM()
		targetNodeId = pv.GetTargetNodeId()
		anno         = pv.GetAnnotations()
		labels       = pv.GetLabels()
		fsType       = pv.GetFsType()
		// local device path of the volume
		devicePath string
		mkfsAgs    []string
	)
	if mkfsParams := req.VolumeContext[mkfsParamsKey]; mkfsParams != "" {
		mkfsAgs = strings.Split(mkfsParams, " ")
	}
	klog.Infof("Volume=%s useLvmDevicePath=%t targetNode=%s mkfsAgs=%+v", req.VolumeId, isLVM, targetNodeId, mkfsAgs)

	// 判断是否是kata rund
	// kata 的 rawfile 方案，不能在宿主机上挂载 dm 到 targetPath
	if anno[containerTypeKey] == containerTypeForKata {
		// 判断是否 远程盘+ kata guest kernel 直连SPDK模式
		// 由于是远程盘，所以在创建LV时，就已经格式化了
		if !isLocalDisk && anno[spdkConnectModeKey] == spdkConnectModeGuestKernelDirect {
			// write config.json to targetPath
			cfgFile := kata.GetConfigFilePath(targetPath)
			err = kata.WriteConfigFileForKataSpdkDirectConnect(cfgFile, fsType, pv.GetSpdkTarget())
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			return &csi.NodeStageVolumeResponse{}, nil
		}

		// TODO: 这里是否允许远程盘?
		if isLocalDisk {
			devicePath = pv.GetDevPath()
		}

		var isBlockModeForRund = anno[volumeModeKey] == volumeModeBlock
		if !isBlockModeForRund {
			// 格式化 rawfile 块设备
			err = mkfs.SafeFormat(devicePath, fsType, mkfsAgs)
			if err != nil {
				klog.Error(err)
				return nil, status.Error(codes.Internal, fmt.Sprintf("NodeStage format local dm for kata error: %+v", err))
			}
		}

		// rawfile配置写入 $targetPath/config.json
		cfgFile := kata.GetConfigFilePath(targetPath)
		err = kata.WriteKataVolumeConfigFile(cfgFile, devicePath, fsType, isBlockModeForRund)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		return &csi.NodeStageVolumeResponse{}, nil
	}

	// for runc:
	// 1. For local volume, skip doing `nvme connect`, use DevPath for formating and mounting.
	// 2. For remote volume, do `nvme connect`, get the connected DevPath.
	// 3. if the volume is local and type is SpdkLVol, do the same as remote volume.
	devicePath = pv.GetDevPath()
	if !isLocalDisk || (!isLVM && pv.GetSpdkTarget() != nil) {
		devicePath, err = connectSpdkTarget(pv.GetSpdkTarget())
		if err != nil {
			klog.Error(err)
			return nil, status.Error(codes.Internal, "cannot connect target to provide devicePath")
		}
	}

	if devicePath == "" {
		klog.Errorf("Cannot find block device by SpdkTarget %#v", pv.GetSpdkTarget())
		return nil, status.Error(codes.Internal, "cannot find block device to format and mount")
	}

	// do partition, mount to targetPath
	// do mount
	klog.Infof("Mounting volume %s from dev %s to %s, isBlockMode=%t", req.VolumeId, devicePath, targetPath, isBlockMode)

	if !isBlockMode {
		// if volume already mounted
		notMnt, err := ns.mounter.IsLikelyNotMountPoint(targetPath)
		if err != nil {
			if os.IsNotExist(err) {
				if err = os.MkdirAll(targetPath, 0750); err != nil {
					return nil, status.Error(codes.Internal, err.Error())
				}
				notMnt = true
			} else {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
		// already mount
		if !notMnt {
			return &csi.NodeStageVolumeResponse{}, nil
		}

		// do format and mount
		var _, hasSnapName = labels[v1.VolumeSourceSnapNameLabelKey]
		var _, hasSnapNS = labels[v1.VolumeSourceSnapNamespaceLabelKey]
		var isClonedVol = hasSnapName && hasSnapNS
		var mountOpts = make([]string, 0, 1)
		// if the volume is cloned, use nouuid option
		// cloned volume has identical UUID with original volume. XFS needs UUID to be unique.
		// ref: https://access.redhat.com/solutions/5494781
		if isClonedVol {
			mountOpts = append(mountOpts, "nouuid")
		}

		// split mounter.FormatAndMount to 2 methods, coz FormatAndMount does not have arguments for mkfs
		if err = mkfs.SafeFormat(devicePath, fsType, mkfsAgs); err != nil {
			klog.Error(err)
			return nil, status.Error(codes.Internal, err.Error())
		}
		if err = ns.mounter.Mount(devicePath, targetPath, fsType, mountOpts); err != nil {
			klog.Error(err)
			return nil, status.Error(codes.Internal, err.Error())
		}

		// if err = ns.mounter.FormatAndMount(devicePath, targetPath, fsType, mountOpts); err != nil {
		// 	klog.Error(err)
		// 	return nil, status.Error(codes.Internal, err.Error())
		// }
	} else {
		klog.Infof("runc BlockMode: do not FormatAndMount %s to %s", devicePath, targetPath)
	}
	return &csi.NodeStageVolumeResponse{}, nil
}

// NodeUnstageVolume MUST be idempotent
// csi.NodeUnstageVolumeRequest:	volume id	+ Required
//
//	target path	+ Required
func (ns *NodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	klog.Infof("NodeUnstageVolume req=%s", req.String())

	if req.VolumeId == "" || req.StagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "NodeUnstageVolumeRequest is invalid")
	}

	var targetPath = req.GetStagingTargetPath()
	var volumeId = req.GetVolumeId()
	// get vol by id
	pv, err := ns.cli.GetPvByID(volumeId)
	if err != nil {
		if err == client.ErrorNotFoundResource {
			klog.Infof("cannot find volume by id %s, %+v; consider NodeUnstage successful", volumeId, err)
			return &csi.NodeUnstageVolumeResponse{}, nil
		}
		klog.Errorf("cannot find volume by id %s, %+v", volumeId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var anno = pv.GetAnnotations()

	// 1. Unmount
	// check targetPath is mounted
	// If the volume corresponding to the volume id is not staged to the staging target path, the plugin MUST reply 0 OK.

	// NOTICE: input/output error blocks umount and disconnect operations.
	// If IO error occurs, notMnt is true. But targetPath need to be umount.
	// If remote spdk target failed, reading targetPath returns IO error.
	var needUmount bool
	notMnt, err := ns.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		klog.Errorf("check mount point of targetPath %s, err %+v", targetPath, err)
		if os.IsNotExist(err) {
			needUmount = false
		} else {
			needUmount = true
		}
	}

	if needUmount || !notMnt {
		// remove config.json in targetPath
		if anno[containerTypeKey] == containerTypeForKata {
			err = misc.RemoveFile(kata.GetConfigFilePath(targetPath))
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
		// count mount point
		_, cnt, err := mount.GetDeviceNameFromMount(ns.mounter, targetPath)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		// do unmount
		err = ns.mounter.Unmount(targetPath)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		klog.Infof("Disk volume %s has been unmounted.", volumeId)
		cnt--
		klog.Infof("Disk volume mount count: %d", cnt)
		if cnt > 0 {
			klog.Errorf("Volume %s still mounted in instance %s", volumeId, ns.driver.GetInstanceId())
			return nil, status.Error(codes.Internal, "unmount failed")
		}
	}

	klog.Info("sleep 5s before disconnecting nvme")
	time.Sleep(5 * time.Second)

	// 2. Disconnect from target subsystem by NQN
	// prepare nvme client
	var isLVM = pv.IsLVM()
	if !isLVM {
		var nqn = pv.GetSpdkTarget().SubsysNQN
		klog.Infof("volume %s is disconnecting remote nvme %s", volumeId, nqn)
		nvmeCli := nvme.NewClientWithCmdPath(nvmeClientFilePath)
		// disconnect must be idempotent; 如果nqn不存在， exit-code 还是0
		out, err := nvmeCli.DisconnectTarget(nvme.DisconnectTargetRequest{
			NQN: nqn,
		})
		if err != nil {
			klog.Errorf("out %s err %+v", string(out), err)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &csi.NodeUnstageVolumeResponse{}, nil
}

// NodePublishVolume bind mount the global directory on a container directory. It MUST be idempotent
// If the volume corresponding to the volume id has already been published at the specified target path,
// and is compatible with the specified volume capability and readonly flag, the plugin MUST reply 0 OK.
// csi.NodePublishVolumeRequest:	volume id			+ Required
//
//	target path			+ Required
//	volume capability	+ Required
//	read only			+ Required (This field is NOT provided when requesting in Kubernetes)
func (ns *NodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	klog.Infof("NodePublishVolume req=%s", req.String())

	if req.VolumeId == "" || req.TargetPath == "" || req.StagingTargetPath == "" || req.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "NodePublishVolumeRequest is invalid")
	}

	targetPath := req.GetTargetPath()
	stagePath := req.GetStagingTargetPath()
	nodeID := ns.driver.GetInstanceId()
	isBlockMode := req.GetVolumeCapability().GetBlock() != nil

	// VolumeContext not have key volContextKeySkipUpdatePublishParam by default
	// skipUpdatePublishParam should be false by default
	var skipUpdatePublishParam bool
	if req.VolumeContext != nil {
		skipUpdatePublishParam = req.VolumeContext[volContextKeySkipUpdatePublishParam] == "true"
	}
	klog.Info(req.VolumeId, "skipUpdatePublishParam", skipUpdatePublishParam)
	if !skipUpdatePublishParam {
		// only record pod-related kv
		volCtx := make(map[string]string, 3)
		if val, has := req.VolumeContext[podNameKey]; has {
			volCtx[podNameKey] = val
		}
		if val, has := req.VolumeContext[podNamespaceKey]; has {
			volCtx[podNamespaceKey] = val
		}
		if val, has := req.VolumeContext[podUuidKey]; has {
			volCtx[podUuidKey] = val
		}

		if mgr, ok := ns.cli.(*client.KubeAPIClient); ok {
			param := client.SetNodePublishParamRequest{
				ID:                req.VolumeId,
				HostNodeID:        nodeID,
				StagingTargetPath: stagePath,
				TargetPath:        targetPath,
				CSIVolumeContext:  volCtx,
			}
			klog.Infof("Record NodePublishVolume Param: %#v", param)
			errRPC := mgr.SetNodePublishParameters(param)
			if errRPC != nil {
				klog.Error(errRPC)
			}
		}
	}

	pv, err := ns.cli.GetPvByID(req.VolumeId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	var (
		fsType    = pv.GetFsType()
		isKataPod = isForRund(pv.GetAnnotations())
		// isLocalDisk = pv.IsLocal()
		isLVM = pv.IsLVM()
	)

	options := []string{"bind"}
	if req.GetReadonly() {
		options = append(options, "ro")
	}

	if isBlockMode && isKataPod {
		return nil, status.Error(codes.InvalidArgument, "Kata rund cannot use PVC with volumeMode=Block, because rund uses rawfile protocol to pass device info")
	}

	// Filesystem Mode 或者 rund-pod, 因为 rund 已经在 NodeStage 把 rawfile 信息写入 $stagePath/config.json
	// 所以对于rund pod, 只需要把 stagePath bind 到 targetPath 即可
	if !isBlockMode {
		// ensure that target path exist
		_, err := os.Stat(targetPath)
		if os.IsNotExist(err) {
			if err = os.MkdirAll(targetPath, 0750); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		} else if err != nil {
			klog.Errorf("unecpected error when check stat of dir %s, err %+v", targetPath, err)
			return nil, status.Error(codes.Internal, err.Error())
		}

		mounter := mkfs.NewMounter()
		mounted, err := mounter.IsMounted(targetPath)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if !mounted {
			// do bind mount
			klog.Infof("Bind mount %s at %s, fsType %s, options %v ...", stagePath, targetPath, fsType, options)
			if err := ns.mounter.Mount(stagePath, targetPath, fsType, options); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		} else {
			klog.Infof("targetPath %s is already mounted", targetPath)
		}
	} else {
		// 获取 device path
		var devPath string = pv.GetDevPath()
		if !isLVM {
			devPath, err = getDevicePath(pv.GetSpdkTarget())
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}

		if devPath == "" {
			errStr := fmt.Sprintf("cannot find devPath of volume %s, id=%s", pv.Name, req.VolumeId)
			return nil, status.Error(codes.Internal, errStr)
		}
		// 如果是 bind mount 块设备, 只需要 fsType 传空，就不会增加 -t fsType 参数
		klog.Infof("mount block device %s to targetPath %s", devPath, targetPath)

		mounter := mkfs.NewMounter()
		mounted, err := mounter.IsMounted(targetPath)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if !mounted {
			if err := mounter.EnsureBlock(targetPath); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			options := []string{"bind"}
			if err := mounter.MountBlock(devPath, targetPath, options...); err != nil {
				return nil, err
			}
		} else {
			klog.Infof("targetPath %s is already mounted", targetPath)
		}

		// if err := ns.mounter.Mount(devPath, targetPath, "", options); err != nil {
		// 	return nil, status.Error(codes.Internal, err.Error())
		// }
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume MUST be idempotent
// csi.NodeUnpublishVolumeRequest:	volume id	+ Required
//
//	target path	+ Required
func (ns *NodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	klog.Infof("NodeUnpublishVolume req=%s", req.String())

	if req.VolumeId == "" || req.TargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "NodeUnpublishVolumeRequest is invalid")
	}

	var targetPath = req.GetTargetPath()

	err := mount.CleanupMountPoint(targetPath, ns.mounter.Interface, true)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Unmount target path %s error: %v", targetPath, err)
	}
	klog.Infof("Unbound mount volume succeed")

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *NodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: ns.driver.GetNodeCapability(),
	}, nil
}

func (ns *NodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	var nodeID = ns.driver.GetInstanceId()
	return &csi.NodeGetInfoResponse{
		NodeId:            nodeID,
		MaxVolumesPerNode: ns.driver.GetMaxVolumePerNode(),
		// report topology to CSINode
		AccessibleTopology: &csi.Topology{
			Segments: map[string]string{
				// TODO: make key obnvmf-host configiurable
				"custom.k8s.alipay.com/obnvmf-host": "true",
				// TODO: set node Label seperately
				// "custom.k8s.alipay.com/nvme-tcp-version": nvme.NvmeTcpVersion.Version,
			},
		},
	}, nil
}

// NodeExpandVolume will expand filesystem of volume.
// Input Parameters:
//
//	volume id: REQUIRED
//	volume path: REQUIRED
func (ns *NodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	klog.Infof("NodeExpandVolume req=%s", req.String())

	if req.VolumeId == "" || req.VolumePath == "" {
		return nil, status.Error(codes.InvalidArgument, "NodeExpandVolumeRequest is invalid")
	}

	var (
		pv           client.PV
		err          error
		volMountPath = req.GetVolumePath()
	)

	klog.Infof("check if volume(%s) is expanded", req.VolumeId)
	// get volume info by volumeID. ip, svcID, nqn, serialNum, modelNumber
	pv, err = ns.cli.GetPvByID(req.VolumeId)
	if err != nil {
		klog.Errorf("cannot find volume by id %s, %+v", req.VolumeId, err)
		err = fmt.Errorf("cannot find volume by id: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var labels = pv.GetLabels()

	if pv.GetStatus() != v1.VolumeStatusReady {
		klog.Infof("Volume status is not ready, wait for it. id=%s", req.VolumeId)
		return nil, status.Error(codes.Internal, fmt.Sprintf("volume %s status is %s", req.VolumeId, pv.GetStatus()))
	}

	if _, has := labels["obnvmf/expansion-original-size"]; has {
		msg := fmt.Sprintf("vol %s has label obnvmf/expansion-original-size, expansion is not finished", req.VolumeId)
		klog.Info(msg)
		return nil, status.Error(codes.Internal, msg)
	}

	var devicePath string = pv.GetDevPath()
	var isLVM = pv.IsLVM()
	if !isLVM {
		devicePath, err = getDevicePath(pv.GetSpdkTarget())
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	if devicePath == "" {
		errStr := fmt.Sprintf("cannot find devPath of volume %s, id=%s", pv.Name, req.VolumeId)
		return nil, status.Error(codes.Internal, errStr)
	}

	fsResizer := mount.NewResizeFs(exec.New())
	ok, err := fsResizer.Resize(devicePath, volMountPath)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !ok {
		return nil, status.Error(codes.Internal, "failed to expand volume filesystem")
	}
	klog.Infof("vol %s resized FS to %d bytes successfully", req.VolumeId, pv.GetSize())

	realSize, err := getBlockDeviceSize(devicePath)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.NodeExpandVolumeResponse{
		CapacityBytes: realSize,
	}, nil
}

// NodeGetVolumeStats
// Input Arguments:
//
//	volume id: REQUIRED
//	volume path: REQUIRED
func (ns *NodeServer) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	volID := req.GetVolumeId()
	path := req.GetVolumePath()

	if len(volID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume id is not provided")
	}
	if len(path) == 0 {
		return nil, status.Error(codes.InvalidArgument, "path is not provided")
	}

	notMnt, err := mount.IsNotMountPoint(ns.mounter, path)
	if err != nil {
		return nil, status.Error(codes.NotFound, "failed to determine if path is a mount point. "+err.Error())
	}
	if notMnt {
		return nil, status.Error(codes.NotFound, "path is not a mount path")
	}

	var sfs unix.Statfs_t
	if err := unix.Statfs(path, &sfs); err != nil {
		return nil, status.Errorf(codes.Internal, "statfs on %s failed: %v", path, err)
	}

	var usage []*csi.VolumeUsage
	usage = append(usage, &csi.VolumeUsage{
		Unit:      csi.VolumeUsage_BYTES,
		Total:     int64(sfs.Blocks) * int64(sfs.Bsize),
		Used:      int64(sfs.Blocks-sfs.Bfree) * int64(sfs.Bsize),
		Available: int64(sfs.Bavail) * int64(sfs.Bsize),
	})
	usage = append(usage, &csi.VolumeUsage{
		Unit:      csi.VolumeUsage_INODES,
		Total:     int64(sfs.Files),
		Used:      int64(sfs.Files - sfs.Ffree),
		Available: int64(sfs.Ffree),
	})
	klog.Infof("vol %s, path %s usage: bytes %d/%d left %d, inodes %d/%d", volID, path,
		usage[0].Used, usage[0].Total, usage[0].Available,
		usage[1].Used, usage[1].Total)

	// TODO: get volume by id; call metric.SetFilesystemMetrics() to set fs metrics by (nodeID, pvcName, pvcNS)

	return &csi.NodeGetVolumeStatsResponse{Usage: usage}, nil
}

func getDevicePath(tgt *v1.SpdkTarget) (devicePath string, err error) {
	// remote disk
	nvmeCli := nvme.NewClientWithCmdPath(nvmeClientFilePath)
	devices, err := nvmeCli.ListNvmeDisk()
	if err != nil {
		klog.Error(err)
		return "", err
	}
	// 根据 SerialNumber 找到 dev path; find by SerialNumber
	for _, item := range devices {
		if item.SerialNumber == tgt.SerialNum {
			devicePath = item.DevicePath
		}
	}

	return
}

func connectSpdkTarget(tgt *v1.SpdkTarget) (devicePath string, err error) {
	// remote disk
	nvmeCli := nvme.NewClientWithCmdPath(nvmeClientFilePath)
	// check if already connected
	// TODO: 如果有残留的 connected nvme disk, 会导致list失败，此时需要清理残留的 disk;
	// 如何确定残留的disk?
	// 每个 volID 只处理自己的vol? 一旦在 connect 之后，如果遇到 error, 退出前必须 disconnect
	devices, err := nvmeCli.ListNvmeDisk()
	if err != nil {
		klog.Error(err)
		// TODO: 如果运行到这里， 说明可能是自身的 vol, 或者其他 vol 的 tgt 端断连
		// 分为两种情况:
		// 1. vol connect 之后，stage异常退出了，并且没有执行 disconnect; 同时 host 和 tgt 断连，例如 subsystem被删除了 ;会再次进入stage，此时list会报错
		// 对于这种情况，vol stage流程未完成，会重复进入 stage流程，由于 list失败，本流程中不知道是哪个 vol 断连，
		// 2. host端已经完成了 connect format mount; tgt 端临时故障，此时 host 端 的kubelet 重启; 这时会重新进入 stage 流程
		// return nil, status.Error(codes.Internal, err.Error())
		return "", err
	}
	// 根据 SerialNumber 找到 dev path; find by SerialNumber
	for _, item := range devices {
		if item.SerialNumber == tgt.SerialNum {
			devicePath = item.DevicePath
		}
	}

	// if devicePath is not found, do connect
	if devicePath == "" {
		var transType string
		var opts = nvme.ConnectTargetOpts{
			ReconnectDelaySec: 2,
			CtrlLossTMO:       10,
		}

		switch tgt.TransType {
		case spdkclient.TransportTypeVFIOUSER:
			transType = "vfio-user"
			opts.HostTransAddr = tgt.AddrFam
		case spdkclient.TransportTypeTCP:
			transType = "tcp"
		}
		out, err := nvmeCli.ConnectTarget(transType, tgt.Address, tgt.SvcID, tgt.SubsysNQN, opts)
		if err != nil {
			klog.Errorf("ConnectTarget returns %s, err %+v", string(out), err)
			return "", err
		}
		// TODO: defer (if need disconnect { do disconnect })
		// connect is async operation, so wait for some time before listing devices
		time.Sleep(8 * time.Second)
		devices, err := nvmeCli.ListNvmeDisk()
		if err != nil {
			klog.Error(err)
			return "", err
		}
		// find by SerialNumber
		for _, item := range devices {
			if item.SerialNumber == tgt.SerialNum {
				devicePath = item.DevicePath
			}
		}
	}

	return
}

func getBlockDeviceSize(devicePath string) (int64, error) {
	output, err := exec.New().Command("blockdev", "--getsize64", devicePath).CombinedOutput()
	if err != nil {
		return -1, fmt.Errorf("error when getting size of block volume at path %s: output: %s, err: %v", devicePath, string(output), err)
	}
	strOut := strings.TrimSpace(string(output))
	gotSizeBytes, err := strconv.ParseInt(strOut, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("failed to parse size %s into int a size", strOut)
	}
	return gotSizeBytes, nil
}

func isForRund(volAnnotations map[string]string) bool {
	if volAnnotations == nil {
		return false
	}

	if val, ok := volAnnotations[containerTypeKey]; ok {
		return val == containerTypeForKata
	}

	return false
}
