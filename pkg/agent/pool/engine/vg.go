package engine

import (
	"fmt"
	"strings"

	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"lite.io/liteio/pkg/util/lvm"
	"lite.io/liteio/pkg/util/misc"
	"lite.io/liteio/pkg/util/mount"
	"k8s.io/klog/v2"
)

const (
	// name prefix of reserved lvol
	reservedLvolPrefix = "reserved-"
)

var (
	ErrNotFoundVG = fmt.Errorf("NotFoundVG")
)

type LvmPoolEngine struct {
	VgName  string
	VgCache v1.KernelLVM
}

func NewLvmPoolEngine(vgName string) (pe *LvmPoolEngine) {
	pe = &LvmPoolEngine{
		VgName: vgName,
	}

	return
}

func (pe *LvmPoolEngine) PoolInfo(vgName string) (info StaticInfo, err error) {
	pe.VgCache, err = pe.initialize(vgName)
	if err != nil {
		return
	}
	info.LVM = &pe.VgCache

	return
}

func (pe *LvmPoolEngine) TotalAndFreeSize() (total, free uint64, err error) {
	var vgName = pe.VgName
	vgList, err := lvm.LvmUtil.ListVG()
	if err != nil {
		klog.Error(err)
		return
	}

	for _, item := range vgList {
		if item.Name == vgName {
			total = item.TotalByte
			free = item.FreeByte
			return
		}
	}

	return
}

func (pe *LvmPoolEngine) GetVolume(volName string) (vol VolumeInfo, err error) {
	var vgName = pe.VgName
	var volExists bool
	var target lvm.LV

	// look for LV in vg
	volExists, _, target, err = isVolumeExistent(vgName, volName)
	if err != nil {
		return
	}

	if volExists {
		vol = VolumeInfo{
			Type: v1.VolumeTypeKernelLVol,
			LvmLV: &v1.KernelLVol{
				Name:     volName,
				VGName:   vgName,
				DevPath:  target.DevPath,
				SizeByte: target.SizeByte,
				LvLayout: target.LvLayout,
			},
		}
	}

	return
}

func (pe *LvmPoolEngine) CreateVolume(req CreateVolumeRequest) (resp CreateVolumeResponse, err error) {
	klog.Info("creating lvm vol ", req)
	var vol v1.KernelLvol

	vol, err = pe.allocate(req.VolName, req.SizeByte, req.LvLayout)
	if err != nil {
		return
	}

	// TODO: Why backend formatting?
	if req.FsType != "" {
		err = mount.SafeFormat(vol.DevPath, req.FsType, nil)
		if err != nil {
			klog.Error(err)
			return
		}
	}

	resp.DevPath = vol.DevPath

	return
}

func (pe *LvmPoolEngine) DeleteVolume(volName string) (err error) {
	var vgName = pe.VgName
	klog.Infof("Removing LV %s", volName)

	// get vol
	var volExists bool
	volList, err := lvm.LvmUtil.ListLVInVG(vgName)
	if err != nil {
		return err
	}

	for _, item := range volList {
		if item.Name == volName {
			volExists = true
			break
		}
	}

	if volExists {
		err = lvm.LvmUtil.RemoveLV(vgName, volName)
		if err != nil {
			klog.Error(err)
			return
		}
	} else {
		klog.Infof("Vol %s not exists, consider removing successfully", volName)
	}

	return
}

func (pe *LvmPoolEngine) CreateSnapshot(req CreateSnapshotRequest) (err error) {
	klog.Info("creating snapshot of LVM vol", req)
	err = pe.createSnapshot(req.SnapshotName, req.OriginName, req.SizeByte)
	if err != nil {
		return
	}
	return
}

func (pe *LvmPoolEngine) RestoreSnapshot(snapshotName string) (err error) {
	klog.Info("restoring snapshot of LVM ", snapshotName)
	err = pe.mergeSnapshot(snapshotName)
	if err != nil {
		return
	}

	return
}

func (pe *LvmPoolEngine) ExpandVolume(req ExpandVolumeRequest) (err error) {
	klog.Info("expanding Logic Volume of LVM", req)
	err = lvm.LvmUtil.ExpandVolume(int64(req.TargetSize-req.OriginSize), fmt.Sprintf("%s/%s", pe.VgName, req.VolName))
	if err != nil {
		return
	}

	return
}

func (pe *LvmPoolEngine) allocate(name string, size uint64, lvLayout v1.LVLayout) (vol v1.KernelLvol, err error) {
	var vgName = pe.VgName
	var volExists, hasLinearLV bool
	var target lvm.LV

	// round up size by 4M
	klog.Infof("opening vg %s, allocate lv %s size=%d", vgName, name, size)

	// look for LV in vg
	volExists, hasLinearLV, target, err = isVolumeExistent(vgName, name)
	if err != nil {
		return
	}

	if lvLayout == "" {
		if hasLinearLV {
			lvLayout = v1.LVLayoutLinear
		} else {
			lvLayout = v1.LVLayoutStriped
		}
	}

	if !volExists {
		// If there is any linear volume, create linear LV.
		// Otherwise, create stripe LV.
		switch lvLayout {
		case v1.LVLayoutLinear:
			klog.Infof("create linear lv %s %d", name, size)
			// try linear LV
			_, err = lvm.LvmUtil.CreateLinearLV(vgName, name, lvm.LvOption{Size: size})
			if err != nil {
				klog.Errorf("Create LV %s failed: %+v", name, err)
				return
			}
		case v1.LVLayoutStriped:
			// round down the volume size by extends
			// CreateStripeLV always uses PVCount as stripe size
			// So the volume size must be a multiple of unit size (pvCount * extendSize)
			if pe.VgCache.PVCount > 0 {
				unitSize := uint64(pe.VgCache.PVCount) * pe.VgCache.ExtendSize
				size = (size / unitSize) * unitSize
			}
			klog.Infof("create striped lv %s %d", name, size)
			// try stripe LV
			_, err = lvm.LvmUtil.CreateStripeLV(vgName, name, size)
			if err != nil {
				klog.Errorf("failed to create stripe LV %s, err %+v.", name, err)
				return
			}
		default:
			err = fmt.Errorf("unsupported LV layout %q", lvLayout)
			return
		}

		klog.Infof("Created LV %s size %d", name, size)
	} else {
		klog.Infof("LV %s already exists", name)
		if target.SizeByte != size {
			err = fmt.Errorf("LV %s size is %d, but want %d", name, target.SizeByte, size)
			return
		}
	}

	// fill vol fileds
	vol.DevPath = fmt.Sprintf("/dev/%s/%s", vgName, name)
	vol.Name = name

	return
}

func (pe *LvmPoolEngine) createSnapshot(snapVol, originVol string, size uint64) (err error) {
	klog.Info("creating snapshot in vg %s", pe.VgName)

	// look for LV in vg
	var volExists, hasLinearLV bool
	var target lvm.LV
	volExists, hasLinearLV, target, err = isVolumeExistent(pe.VgName, snapVol)
	if err != nil {
		return
	}

	if !volExists {
		// If there is any linear volume, create linear LV.
		// Otherwise, create stripe LV.
		if hasLinearLV {
			klog.Infof("create linear snap %s %d", snapVol, size)
			// try linear LV
			err = lvm.LvmUtil.CreateSnapshotLinear(pe.VgName, snapVol, originVol, size)
			if err != nil {
				klog.Errorf("create linear snapshot %s failed: %+v", snapVol, err)
				return
			}
		} else {
			klog.Infof("create striped snap %s %d", snapVol, size)
			// try stripe LV
			err = lvm.LvmUtil.CreateSnapshotStripe(pe.VgName, snapVol, originVol, size)
			if err != nil {
				klog.Errorf("failed to create stripe snapshot %s, err %+v", snapVol, err)
				return
			}
		}
		klog.Infof("created snap %s size %d", snapVol, size)
	} else {
		klog.Infof("snapshot %s already exists", snapVol)
		if target.SizeByte != size {
			err = fmt.Errorf("snapshot %s size is %d, but want %d", snapVol, target.SizeByte, size)
			return
		}
	}

	return
}

func (pe *LvmPoolEngine) mergeSnapshot(snapName string) (err error) {
	var vgName = pe.VgName
	var snapVol, originVol lvm.LV
	var snapVolExist bool
	snapVolExist, _, snapVol, err = isVolumeExistent(vgName, snapName)
	if err != nil {
		return
	}
	// snapName must have a origin vol
	if snapVol.Origin == "" {
		err = fmt.Errorf("MergeSnapshot failed, %s is not a snapshot", snapName)
		return
	}

	// validate origin vol is not Open
	_, _, originVol, err = isVolumeExistent(vgName, snapVol.Origin)
	if err != nil {
		return
	}
	if originVol.LvDeviceOpen == lvm.LvDeviceOpen {
		err = fmt.Errorf("MergeSnapshot failed, origin vol (%s) of snapshot (%s) is opened", snapVol.Origin, snapName)
		return
	}

	if !snapVolExist {
		err = fmt.Errorf("snap vol %s not exists", snapName)
		return
	}

	klog.Info("start merging snap ", snapName, snapVol)

	err = lvm.LvmUtil.MergeSnapshot(vgName, snapName)
	if err != nil {
		klog.Error(err)
		return
	}

	return
}

func (pe *LvmPoolEngine) initialize(vgName string) (result v1.KernelLVM, err error) {
	var vgList []lvm.VG
	var found bool
	vgList, err = lvm.LvmUtil.ListVG()
	if err != nil {
		klog.Error(err)
		return
	}
	for _, item := range vgList {
		klog.Infof("found vg %s: %+v", item.Name, item)
		if item.Name == vgName {
			found = true
			totalBytes := item.TotalByte
			freeBytes := item.FreeByte
			result = v1.KernelLVM{
				Name:         item.Name,
				VgUUID:       item.UUID,
				Bytes:        totalBytes,
				ReservedLVol: pe.getReservedVols(),
				PVCount:      item.PVCount,
				ExtendSize:   item.ExtendSize,
				ExtendCount:  item.ExtendCount,
			}

			klog.Infof("found VG %s as StoragePool. TotalSpace: %d, FreeSpace: %d", item.Name, totalBytes, freeBytes)
		}
	}

	if !found {
		err = ErrNotFoundVG
		return
	}

	return
}

func (pe *LvmPoolEngine) getReservedVols() (vols []v1.KernelLVol) {
	// check reserved lvol
	nameSet := misc.NewEmptySet()
	lvs, err := lvm.LvmUtil.ListLVInVG(pe.VgName)
	if err != nil {
		klog.Fatal(err)
	}
	for _, item := range lvs {
		// command lvs on some nodes outputs duplicated lvol information.
		// so we have to remove duplicated lvol by lvol name
		if !nameSet.Contains(item.Name) {
			if strings.HasPrefix(item.Name, reservedLvolPrefix) {
				nameSet.Add(item.Name)
				klog.Infof("Found reserved lvol: %+v", item)
				vols = append(vols, v1.KernelLVol{
					Name:     item.Name,
					SizeByte: item.SizeByte,
					VGName:   item.VGName,
					DevPath:  item.DevPath,
					LvLayout: item.LvLayout,
				})
			}
		}
	}

	return
}

func isVolumeExistent(vgName, lvName string) (volExists, hasLinearLV bool, target lvm.LV, err error) {
	lvList, err := lvm.LvmUtil.ListLVInVG(vgName)
	if err != nil {
		klog.Errorf("ListLVInVG failed", err)
		return
	}

	for _, lv := range lvList {
		if lv.LvLayout == string(v1.LVLayoutLinear) {
			hasLinearLV = true
		}

		if lv.Name == lvName {
			volExists = true
			target = lv
			break
		}
	}
	return
}
