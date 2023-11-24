package sync

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"code.alipay.com/dbplatform/node-disk-controller/pkg/agent/metric"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/agent/pool"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/agent/pool/engine"
	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/generated/clientset/versioned"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/spdk"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/spdk/jsonrpc/client"
	spdkrpc "code.alipay.com/dbplatform/node-disk-controller/pkg/spdk/jsonrpc/client"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/misc"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// VolumeSyncer create queue and informer to sync volume on the node from APIServer
type VolumeSyncer struct {
	nodeID      string
	poolService pool.StoragePoolServiceIface
	// storeCli is used to read/write StoragePool, AntstorVolumes from APIServer
	storeCli versioned.Interface
	lister   metric.MetricTargetListerIface
}

func NewVolumeSyncer(storeCli versioned.Interface, poolSvc pool.StoragePoolServiceIface, lister metric.MetricTargetListerIface) *VolumeSyncer {
	return &VolumeSyncer{
		nodeID:      poolSvc.GetStoragePool().Name,
		poolService: poolSvc,
		storeCli:    storeCli,
		lister:      lister,
	}
}

// Start create queue and informer to sync volume on the node from APIServer
func (vs *VolumeSyncer) Start(ctx context.Context) (err error) {
	poolName := vs.poolService.GetStoragePool().GetName()
	volumeListWatcher := cache.NewFilteredListWatchFromClient(vs.storeCli.VolumeV1().RESTClient(), "antstorvolumes",
		v1.DefaultNamespace, func(options *metav1.ListOptions) {
			options.LabelSelector = fmt.Sprintf("%s=%s", v1.TargetNodeIdLabelKey, poolName)
		})

	volumeSyncLoop := NewSyncLoop("VolumeLoop", volumeListWatcher, &v1.AntstorVolume{}, func(name string) (err error) {
		return vs.syncOneVolumeByName(name)
	})

	volumeSyncLoop.RunLoop(ctx.Done())
	return
}

func (vs *VolumeSyncer) syncOneVolumeByName(nsName string) (err error) {
	ns, name, err := cache.SplitMetaNamespaceKey(nsName)
	if err != nil {
		return
	}
	cli := vs.storeCli.VolumeV1().AntstorVolumes(ns)
	volume, err := cli.Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		// volume is already deleted, ignore not-found error
		if errors.IsNotFound(err) {
			return nil
		}
		return
	}

	return vs.syncOneVolume(volume)
}

func (vs *VolumeSyncer) syncOneVolume(volume *v1.AntstorVolume) (err error) {
	if volume == nil {
		err = fmt.Errorf("volume is nil")
		return
	}

	var (
		needReturn bool
	)

	if !misc.InSliceString(v1.InStateFinalizer, volume.Finalizers) {
		klog.Infof("volume %s has no InStateFinalizer, wait for next turn", volume.Name)
		return
	}

	if volume.DeletionTimestamp != nil {
		klog.Infof("deleting resources of volume %s, RV %s", volume.Name, volume.ResourceVersion)
		return vs.handleDeletion(volume)
	}

	klog.Infof("syncing volume %s to create logic vol and spdk target", volume.Name)

	err = vs.expandVolume(volume)
	if err != nil {
		klog.Error(err)
		return
	}

	needReturn, err = vs.applyVolume(volume)
	if err != nil || needReturn {
		klog.Error(err)
		return
	}

	if volume.Status.Status == v1.VolumeStatusReady {
		klog.Infof("volume %s is ready, stop syncing", volume.Name)
		// add volume to volumeInfoLister
		vs.lister.AddObject(volume.DeepCopy())
		return
	}

	// set volume.Spec.Type, update and return;
	if volume.Spec.Type == v1.VolumeTypeFlexible {
		return vs.updateVolumeType(volume)
	}

	// create volume
	needReturn, err = vs.createVolume(volume)
	if err != nil || needReturn {
		return
	}

	// create openaccess
	needReturn, err = vs.createOpenAccess(volume)
	if err != nil || needReturn {
		return
	}

	// after creating lvol and subsystem, volume is supposed to be ready to use
	klog.Infof("volume %s is ready to use", volume.Name)
	volume.Status.Status = v1.VolumeStatusReady
	_, err = vs.storeCli.VolumeV1().AntstorVolumes(volume.Namespace).UpdateStatus(context.Background(), volume, metav1.UpdateOptions{})
	return
}

func (vs *VolumeSyncer) expandVolume(vol *v1.AntstorVolume) (err error) {
	var (
		volName       string
		originSizeStr string
		originalSize  int64
		targetSize    uint64
		inExpansion   bool
		aioVolume     *pool.AioVolume
	)

	if vol == nil {
		return
	}

	if vol.Labels != nil {
		if originSizeStr, inExpansion = vol.Labels[v1.ExpansionOriginalSize]; inExpansion {
			inExpansion = true
			originalSize, err = strconv.ParseInt(originSizeStr, 10, 64)
			if err != nil {
				klog.Error("invalid value of ExpansionOriginalSize")
				return
			}
		}
	}

	if !inExpansion {
		klog.Infof("volume %s is not in expansion", vol.Name)
		return
	}

	klog.Infof("try to expend volume %s", vol.Name)
	targetSize = vol.Spec.SizeByte

	switch vol.Spec.Type {
	case v1.VolumeTypeKernelLVol:
		volName = vol.Spec.KernelLvol.Name
		// if pool engine is LVM and volume is remote, then create aio_bdev and add it into SDPK subsystem
		if vol.Spec.TargetNodeId != vol.Spec.HostNode.ID {
			aioVolume = &pool.AioVolume{
				DevPath:  vol.Spec.KernelLvol.DevPath,
				BdevName: vol.Spec.SpdkTarget.BdevName,
			}
		}
	case v1.VolumeTypeSpdkLVol:
		volName = fmt.Sprintf("%s/%s", vol.Spec.SpdkLvol.LvsName, vol.Spec.SpdkLvol.Name)
	}
	err = vs.poolService.PoolEngine().ExpandVolume(engine.ExpandVolumeRequest{
		VolName:    volName,
		TargetSize: targetSize,
		OriginSize: uint64(originalSize),
	})
	if err != nil {
		return
	}

	if aioVolume != nil {
		err = vs.poolService.SpdkService().ResizeAioBdev(spdk.AioBdevResizeRequest{
			BdevName:   aioVolume.BdevName,
			TargetSize: targetSize,
		})
		if err != nil {
			klog.Error(err)
			return
		}
	}

	// removing label ExpansionOriginalSize means expansion is finished
	if inExpansion {
		delete(vol.Labels, v1.ExpansionOriginalSize)
		_, err = vs.storeCli.VolumeV1().AntstorVolumes(vol.Namespace).Update(context.Background(), vol, metav1.UpdateOptions{})
		if err != nil {
			return
		}
	}

	return
}

func (vs *VolumeSyncer) handleDeletion(volume *v1.AntstorVolume) (err error) {
	// TODO: reconsider deletion constraint
	// delete tgt
	if misc.InSliceString(v1.SpdkTargetFinalizer, volume.Finalizers) ||
		volume.Spec.SpdkTarget != nil {

		klog.Infof("removing SpdkTargetFinalizer from finalizer of volume %s", volume.Name)
		if volume.Spec.SpdkTarget == nil {
			klog.Error("volume has SpdkTargetFinalizer but volume spec has a nil SpdkTarget")
			err = fmt.Errorf("volume %s SpdkTarget is nil", volume.Name)
			return
		}

		err = vs.poolService.Access().RemoveAccces(pool.Access{
			AIO: &pool.AioVolume{
				BdevName: volume.Spec.SpdkTarget.BdevName,
			},
			OpenAccess: spdk.Target{
				TransAddr: volume.Spec.SpdkTarget.Address,
				TransType: volume.Spec.SpdkTarget.TransType,
				NQN:       volume.Spec.SpdkTarget.SubsysNQN,
			},
		})
		if err != nil {
			klog.Error(err)
			return
		}

		// remove v1.SpdkTargetFinalizer
		if misc.InSliceString(v1.SpdkTargetFinalizer, volume.Finalizers) {
			var newFinalizers = make([]string, 0, len(volume.Finalizers))
			for _, item := range volume.Finalizers {
				if item != v1.SpdkTargetFinalizer {
					newFinalizers = append(newFinalizers, item)
				}
			}
			volume.Finalizers = newFinalizers
			_, err = vs.storeCli.VolumeV1().AntstorVolumes(volume.Namespace).Update(context.Background(), volume, metav1.UpdateOptions{})
			return
		}
	}

	var hasLogicVolFinalizer = misc.InSliceString(v1.KernelLVolFinalizer, volume.Finalizers) ||
		misc.InSliceString(v1.SpdkLvolFinalizer, volume.Finalizers) ||
		misc.InSliceString(v1.LogicVolumeFinalizer, volume.Finalizers)

	_, hasSnapName := volume.Labels[v1.VolumeSourceSnapNameLabelKey]
	_, hasSnapNS := volume.Labels[v1.VolumeSourceSnapNamespaceLabelKey]
	var fromSnap = hasSnapNS && hasSnapName

	if hasLogicVolFinalizer {
		klog.Infof("removing logic volume finalizer from volume %s", volume.Name)

		if volume.Spec.Type == v1.VolumeTypeSpdkLVol {
			// if this volume is cloned from a snapshot, inflate the cloned volume first,
			// otherwise the snapshot will not be released (can only be removed forcely)
			if fromSnap {
				klog.Infof("inflate the cloned volume before deleting it")
				err = vs.poolService.SpdkService().InflateLvol(spdk.InflateLvolReq{
					LVStore:  vs.poolService.GetStoragePool().Spec.SpdkLVStore.Name,
					LvolName: volume.Name,
				})
				if err != nil {
					klog.Error(err)
					return
				}
			}

			// if the volume has a snapshot, inflate it before delete it
			var snapList *v1.AntstorSnapshotList
			snapList, err = vs.storeCli.VolumeV1().AntstorSnapshots(volume.Namespace).List(context.Background(), metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", v1.OriginVolumeNameLabelKey, volume.Name),
			})
			if err != nil {
				klog.Error(err)
				return
			}
			if len(snapList.Items) > 0 {
				klog.Infof("inflate the cloned volume before deleting it")
				err = vs.poolService.SpdkService().InflateLvol(spdk.InflateLvolReq{
					LVStore:  vs.poolService.GetStoragePool().Spec.SpdkLVStore.Name,
					LvolName: volume.Name,
				})
				if err != nil {
					klog.Error(err)
					return
				}
			}
		}

		// delete logic volume
		err = vs.poolService.PoolEngine().DeleteVolume(volume.Name)
		if err != nil {
			klog.Error(err)
			return
		}

		// remove v1.KernelLVolFinalizer
		var newFinalizers = make([]string, 0, len(volume.Finalizers))
		var toDelFinalizers = []string{v1.KernelLVolFinalizer, v1.SpdkLvolFinalizer, v1.LogicVolumeFinalizer}
		for _, item := range volume.Finalizers {
			if !misc.InSliceString(item, toDelFinalizers) {
				newFinalizers = append(newFinalizers, item)
			}
		}
		volume.Finalizers = newFinalizers
		_, err = vs.storeCli.VolumeV1().AntstorVolumes(volume.Namespace).Update(context.Background(), volume, metav1.UpdateOptions{})
		// delete volume in volumeInfoLister
		vs.lister.DeleteObject(volume.Name)
		return
	}

	klog.Infof("nothing to do with finalizers of volume %s", volume.Name)

	return
}

func (vs *VolumeSyncer) updateVolumeType(volume *v1.AntstorVolume) (err error) {
	switch vs.poolService.Mode() {
	case v1.PoolModeKernelLVM:
		volume.Spec.Type = v1.VolumeTypeKernelLVol
	case v1.PoolModeSpdkLVStore:
		volume.Spec.Type = v1.VolumeTypeSpdkLVol
	default:
		klog.Errorf("invalid pool mode %s", vs.poolService.Mode())
	}
	_, err = vs.storeCli.VolumeV1().AntstorVolumes(volume.Namespace).Update(context.Background(), volume, metav1.UpdateOptions{})
	if err != nil {
		klog.Error(err)
	}
	return
}

func (vs *VolumeSyncer) createVolume(volume *v1.AntstorVolume) (needReturn bool, err error) {
	var (
		hasSnapName, hasSnapNS bool
		fromSnap               bool
		snapName, snapNS       string
		// fsType for LVM
		fsType string
		// lv layout
		lvLayout v1.LVLayout
	)

	snapName, hasSnapName = volume.Labels[v1.VolumeSourceSnapNameLabelKey]
	snapNS, hasSnapNS = volume.Labels[v1.VolumeSourceSnapNamespaceLabelKey]
	fromSnap = hasSnapName && hasSnapNS
	if volume.Annotations[v1.SpdkConnectModeKey] == v1.SpdkConnectModeGuestKernelDirect && volume.Annotations[v1.FsTypeLabelKey] != "" {
		fsType = volume.Annotations[v1.FsTypeLabelKey]
	}
	if val := volume.Annotations[v1.LvLayoutAnnoKey]; val != "" {
		lvLayout = v1.LVLayout(val)
	}

	var hasLogicVolFinalizer = misc.InSliceString(v1.KernelLVolFinalizer, volume.Finalizers) ||
		misc.InSliceString(v1.SpdkLvolFinalizer, volume.Finalizers) ||
		misc.InSliceString(v1.LogicVolumeFinalizer, volume.Finalizers)

	if hasLogicVolFinalizer {
		klog.Infof("already created logic volume %s", volume.Name)
		return false, nil
	}

	var (
		req  engine.CreateVolumeRequest
		resp engine.CreateVolumeResponse
	)

	switch volume.Spec.Type {
	case v1.VolumeTypeKernelLVol:
		// Only Spdklvs volume can specify VolumeContentSource
		if fromSnap {
			// TODO: update volume status message
			err = fmt.Errorf("not support creating new lvm volume from a lvm snapshot yet")
			klog.Error(err)
			return
		}

		// validate lv layout

		// create new volume
		req = engine.CreateVolumeRequest{
			VolName:  volume.Name,
			SizeByte: volume.Spec.SizeByte,
			FsType:   fsType,
			LvLayout: lvLayout,
		}

		if volume.Spec.KernelLvol == nil {
			volume.Spec.KernelLvol = &v1.KernelLvol{}
		}
		volume.Spec.KernelLvol.Name = volume.Name

	case v1.VolumeTypeSpdkLVol:
		lvsName := vs.poolService.GetStoragePool().Spec.SpdkLVStore.Name
		if fromSnap {
			klog.Infof("cloning spdk lvol for vol %s", volume.Name)
			var snap *v1.AntstorSnapshot
			var uuid string
			snap, err = vs.storeCli.VolumeV1().AntstorSnapshots(snapNS).Get(context.Background(), snapName, metav1.GetOptions{})
			if err != nil {
				klog.Error(err)
				return
			}
			uuid, err = vs.poolService.SpdkService().CreateLvolClone(spdk.CreateLvolCloneReq{
				LVStore:   lvsName,
				SnapName:  snap.Spec.SpdkLvol.Name,
				CloneName: volume.Name,
			})
			if err != nil {
				klog.Error(err, uuid)
				return
			}
		} else {
			// create new volume
			req = engine.CreateVolumeRequest{
				VolName:  volume.Name,
				SizeByte: volume.Spec.SizeByte,
			}
		}

		if volume.Spec.SpdkLvol == nil {
			volume.Spec.SpdkLvol = &v1.SpdkLvol{}
		}
		volume.Spec.SpdkLvol.LvsName = lvsName
		volume.Spec.SpdkLvol.Name = volume.Name
		volume.Spec.SpdkLvol.Thin = false
	}

	// create new logic volume
	if req.SizeByte > 0 && req.VolName != "" {
		klog.Infof("creating logic volume for vol %s, req=%+v", volume.Name, req)
		resp, err = vs.poolService.PoolEngine().CreateVolume(req)
		if err != nil {
			klog.Error(err, resp.DevPath)
			return
		}
	}

	// set devPath for LVM volume
	if resp.DevPath != "" {
		volume.Spec.KernelLvol.DevPath = resp.DevPath
	}
	// add finalizer
	volume.Finalizers = append(volume.Finalizers, v1.LogicVolumeFinalizer)
	_, err = vs.storeCli.VolumeV1().AntstorVolumes(volume.Namespace).Update(context.Background(), volume, metav1.UpdateOptions{})
	return true, err
}

func (vs *VolumeSyncer) createOpenAccess(volume *v1.AntstorVolume) (needReturn bool, err error) {
	// for remote volume, create tgt subsystem
	var (
		// for local volume, create tgt subsystem for storagepool in SpdkLVStore Mode
		isLocal = volume.Spec.TargetNodeId == volume.Spec.HostNode.ID
		sp      = vs.poolService.GetStoragePool()
		nodeIP  = sp.Spec.NodeInfo.IP
	)

	// fetch StoragePool info
	if nodeIP == "" {
		err = fmt.Errorf("cannot get node ip (%+v) to create OpenAccess of volume", sp.Spec.NodeInfo)
		klog.Error(err)
		return
	}

	if misc.InSliceString(v1.SpdkTargetFinalizer, volume.Finalizers) {
		klog.Info("OpenAccess is already created, so skip creation. volume=", volume.Name)
		return false, nil
	}

	if volume.Spec.Uuid == "" {
		err = fmt.Errorf("invalid volume, no uuid")
		return
	}

	var aioVolume *pool.AioVolume
	var lvolVolume *pool.SpdkLVolume

	switch volume.Spec.Type {
	case v1.VolumeTypeKernelLVol:
		// for loacl lvm volume, do not create subsystem
		if isLocal {
			klog.Info("skip creating subsystem for local LVM volume")
			return false, nil
		}
		// validate
		if volume.Spec.KernelLvol.DevPath == "" {
			err = fmt.Errorf("invalid KernelLvol %+v to create OpenAccess", volume.Spec.KernelLvol)
			return
		}

		klog.Infof("creating spdk target for LVM vol %s", volume.Name)

		volume.Spec.SpdkTarget = &v1.SpdkTarget{
			SubsysNQN: GetNQNFromUUID(volume.Spec.Uuid),
			NSUUID:    volume.Spec.Uuid,
			BdevName:  GetBdevNameFromUUID(volume.Spec.Uuid),
			SerialNum: GetSNFromUUID(volume.Spec.Uuid),
			TransType: spdkrpc.TransportTypeTCP,
			Address:   nodeIP,
			AddrFam:   string(client.AddrFamilyIPv4),
			// NOTICE: SvcID is set after subsystem is created
		}
		aioVolume = &pool.AioVolume{
			DevPath:  volume.Spec.KernelLvol.DevPath,
			BdevName: volume.Spec.SpdkTarget.BdevName,
		}

	case v1.VolumeTypeSpdkLVol:
		// for spdk lvol, create subsystem for both local and remote volume
		klog.Infof("creating spdk target for SPDK lvol %s", volume.Name)

		// set SpdkTarget
		if volume.Spec.SpdkTarget == nil {
			volume.Spec.SpdkTarget = &v1.SpdkTarget{}
		}
		volume.Spec.SpdkTarget.BdevName = volume.Spec.SpdkLvol.FullName()
		volume.Spec.SpdkTarget.SerialNum = GetSNFromUUID(volume.Spec.Uuid)

		// TODO: validate the nvmf_tgt has VFIOUser capability
		var vfioUserCapable = false
		if isLocal && vfioUserCapable {
			volume.Spec.SpdkTarget.Address = GetSocketPathFromeUUID(volume.Spec.Uuid)
			volume.Spec.SpdkTarget.TransType = spdkrpc.TransportTypeVFIOUSER
			// in VFIOUSER mode, SvcID is set to the bdf of any NVMe lvs build with
			// this will be deprecated in the future
			// TODO: set a real bdf number
			volume.Spec.SpdkTarget.SvcID = "0000:6b:00.0"
			// use LOCAL_COPY for default, unless user specified in PVC annotation
			if mode, has := volume.Annotations[v1.VfiouserModeKey]; has {
				volume.Spec.SpdkTarget.AddrFam = mode
			} else {
				volume.Spec.SpdkTarget.AddrFam = string(client.AddrFamilyLocalCopy)
			}
		} else {
			// for remote volume, SvcID(port) is set after subsystem is created
			volume.Spec.SpdkTarget.Address = nodeIP
			volume.Spec.SpdkTarget.TransType = spdkrpc.TransportTypeTCP
			volume.Spec.SpdkTarget.AddrFam = string(client.AddrFamilyIPv4)
		}

		// for migration destination volume, NQN and NSUUID should be the same as resource target
		if volume.Spec.SpdkTarget.SubsysNQN == "" {
			volume.Spec.SpdkTarget.SubsysNQN = GetNQNFromUUID(volume.Spec.Uuid)
		}
		if volume.Spec.SpdkTarget.NSUUID == "" {
			volume.Spec.SpdkTarget.NSUUID = volume.Spec.Uuid
		}

		lvolVolume = &pool.SpdkLVolume{
			LvsName:  volume.Spec.SpdkLvol.LvsName,
			LvolName: volume.Spec.SpdkLvol.Name,
		}
	}

	var allowHosts []string
	var hostPool *v1.StoragePool
	var resp spdk.Target

	if volume.Spec.HostNode != nil && volume.Spec.HostNode.ID != "" {
		// get hostnqn from metadata
		hostPool, err = vs.storeCli.VolumeV1().StoragePools(v1.DefaultNamespace).Get(context.Background(), volume.Spec.HostNode.ID, metav1.GetOptions{})
		if err != nil {
			klog.Error(err)
			return
		}
		if hostNQN, has := hostPool.Annotations[v1.AnnotationHostNQN]; has {
			allowHosts = append(allowHosts, hostNQN)
		}
	}

	resp, err = vs.poolService.Access().ExposeAccess(pool.Access{
		AIO:  aioVolume,
		LVol: lvolVolume,
		OpenAccess: spdk.Target{
			NQN:          volume.Spec.SpdkTarget.SubsysNQN,
			SerialNumber: volume.Spec.SpdkTarget.SerialNum,
			NSUUID:       volume.Spec.SpdkTarget.NSUUID,
			TransAddr:    volume.Spec.SpdkTarget.Address,
			TransType:    volume.Spec.SpdkTarget.TransType,
			AddrFam:      volume.Spec.SpdkTarget.AddrFam,
			SvcID:        volume.Spec.SpdkTarget.SvcID,
		},
		AllowHostNQN: allowHosts,
	})

	if err != nil {
		klog.Error(err)
		return
	}
	klog.Info("exposed spdk access ", resp)
	// set SvcID by response
	volume.Spec.SpdkTarget.SvcID = resp.SvcID

	// create spdk tgt and add SpdkTargetFinalizer
	volume.Finalizers = append(volume.Finalizers, v1.SpdkTargetFinalizer)
	_, err = vs.storeCli.VolumeV1().AntstorVolumes(volume.Namespace).Update(context.Background(), volume, metav1.UpdateOptions{})

	return true, err
}

func GetSNFromUUID(uuid string) (sn string) {
	sn = strings.ReplaceAll(uuid, "-", "")
	if len(sn) > 20 {
		sn = sn[:20]
	}
	return sn
}

func GetBdevNameFromUUID(uuid string) (bdevName string) {
	return GetSNFromUUID(uuid)
}

func GetNQNFromUUID(uuid string) (nqn string) {
	return "nqn.2021-03.com.alipay.ob:uuid:" + uuid
}

func GetSocketPathFromeUUID(uuid string) (path string) {
	return "/usr/tmp/" + uuid
}

func (vs *VolumeSyncer) applyVolume(volume *v1.AntstorVolume) (needReturn bool, err error) {
	// apply allocated size of LV to Annotation
	if _, has := volume.Annotations[v1.AllocatedSizeAnnoKey]; !has {
		// get allocated size
		var (
			lvName   = volume.Name
			vol      engine.VolumeInfo
			sizeByte uint64
		)
		vol, err = vs.poolService.PoolEngine().GetVolume(lvName)
		if err != nil {
			return
		}

		switch vol.Type {
		case v1.VolumeTypeKernelLVol:
			if vol.LvmLV != nil {
				sizeByte = vol.LvmLV.SizeByte
			}
		case v1.VolumeTypeSpdkLVol:
			if vol.SpdkLvol != nil {
				sizeByte = vol.SpdkLvol.SizeByte
			}
		}

		// set annotation
		klog.Infof("volume %s, realSize=%d specSize=%d", volume.Name, sizeByte, volume.Spec.SizeByte)
		if sizeByte > 0 && sizeByte != volume.Spec.SizeByte {
			klog.Infof("set Annotation obnvmf/allocated-bytes=%d", sizeByte)
			if volume.Annotations == nil {
				volume.Annotations = make(map[string]string)
			}
			volume.Annotations[v1.AllocatedSizeAnnoKey] = strconv.Itoa(int(sizeByte))
			// update volume
			_, err = vs.storeCli.VolumeV1().AntstorVolumes(v1.DefaultNamespace).Update(context.Background(), volume, metav1.UpdateOptions{})
			if err != nil {
				return
			}
		}
	}

	// Apply allowHosts config to volume.
	// datacontrol and volgroup are created simultaneously. So when volume is ready,
	// hostNode info of the volume may be empty. A while later, datacontrol controller writes hostNode to
	// volgroup, and the info will be passed to the volumes.
	// So we must apply allowHosts to volume even if the status is Ready.
	klog.Infof("volume %s is ready, apply allowHosts to volume", volume.Name)
	if volume.Spec.SpdkTarget != nil {
		var allowHosts []string
		var hostPool *v1.StoragePool
		var tgt = spdk.Target{
			NQN:          volume.Spec.SpdkTarget.SubsysNQN,
			SerialNumber: volume.Spec.SpdkTarget.SerialNum,
			NSUUID:       volume.Spec.SpdkTarget.NSUUID,
			SvcID:        volume.Spec.SpdkTarget.SvcID,
			TransAddr:    volume.Spec.SpdkTarget.Address,
			TransType:    volume.Spec.SpdkTarget.TransType,
			AddrFam:      volume.Spec.SpdkTarget.AddrFam,
		}

		if volume.Spec.HostNode != nil && volume.Spec.HostNode.ID != "" {
			// get hostnqn from metadata
			hostPool, err = vs.storeCli.VolumeV1().StoragePools(v1.DefaultNamespace).Get(context.Background(), volume.Spec.HostNode.ID, metav1.GetOptions{})
			if err != nil {
				klog.Error(err)
				return
			}
			if hostNQN, has := hostPool.Annotations[v1.AnnotationHostNQN]; has {
				allowHosts = append(allowHosts, hostNQN)
			}
		}
		vs.poolService.Access().ExposeAccess(pool.Access{
			OpenAccess:   tgt,
			AllowHostNQN: allowHosts,
		})
	}

	return
}
