package metric

import (
	"fmt"
	"sync"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
)

type itemIdentity struct {
	// device id (major_devid*256+minor_devid)
	devID uint64
	// device path of LVM LV
	devPath string
	// lvsName/lvolName
	lvolFullName string
	// nqn of subsystem
	subsysNQN string
}

type metricTarget struct {
	// node name
	nodeName string
	// namespaced name
	pvcNSedName string
	// uuid of volume
	volUUID string
	// metric target
	itemID itemIdentity
}

func (mt *metricTarget) id() string {
	return fmt.Sprintf("uuid:%s,dev:%s,lvol:%s,nqn:%s,", mt.volUUID, mt.itemID.devPath, mt.itemID.lvolFullName, mt.itemID.subsysNQN)
}

type MetricTargetListerIface interface {
	List() []metricTarget
	//
	AddObject(vol *v1.AntstorVolume)
	DeleteObject(volName string)
}

type MetricInfoLister struct {
	volumeMap map[string]*v1.AntstorVolume
	mutex     sync.Mutex
}

func NewMetricInfoLister() MetricTargetListerIface {
	return &MetricInfoLister{
		volumeMap: make(map[string]*v1.AntstorVolume),
	}
}

func (l *MetricInfoLister) List() (list []metricTarget) {
	list = make([]metricTarget, 0, len(l.volumeMap))
	for _, vol := range l.volumeMap {
		var info = metricTarget{
			nodeName:    vol.Spec.TargetNodeId,
			pvcNSedName: vol.Labels[v1.VolumeContextKeyPvcNS] + "/" + vol.Labels[v1.VolumePVNameLabelKey],
			volUUID:     vol.Spec.Uuid,
			itemID:      itemIdentity{},
		}

		// get device info
		// only consider antstorvolumes in lvm mode
		if vol.Spec.Type == v1.VolumeTypeKernelLVol && vol.Spec.KernelLvol != nil && vol.Spec.KernelLvol.DevPath != "" {
			devPath := vol.Spec.KernelLvol.DevPath
			// change devPath to device id, if err occurs just log and skip it
			var stat unix.Stat_t
			if err := unix.Stat(devPath, &stat); err != nil {
				klog.Error("get stat on %s failed: %v", devPath, err)
			}
			info.itemID.devID = uint64(stat.Rdev)
			info.itemID.devPath = devPath
		}

		// get lvol info
		// only consider antstorvolumes which have lvol, in spdk mode in other words
		if vol.Spec.Type == v1.VolumeTypeSpdkLVol && vol.Spec.SpdkLvol != nil {
			lvolFullName := fmt.Sprintf("%s/%s", vol.Spec.SpdkLvol.LvsName, vol.Spec.SpdkLvol.Name)
			info.itemID.lvolFullName = lvolFullName
		}

		// get subsys info
		// only consider antstorvolumes which have subsystem, in remote mode in other words
		if vol.Spec.SpdkTarget != nil {
			nqn := vol.Spec.SpdkTarget.SubsysNQN
			if nqn != "" {
				info.itemID.subsysNQN = nqn
			}
		}

		list = append(list, info)
	}
	klog.Infof("list %d metric targets", len(list))
	return
}

func (l *MetricInfoLister) AddObject(vol *v1.AntstorVolume) {
	if vol == nil {
		return
	}
	klog.Info("add one volume to lister: ", vol.Name)
	if _, ok := l.volumeMap[vol.Name]; ok {
		return
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.volumeMap[vol.Name] = vol
}

func (l *MetricInfoLister) DeleteObject(volName string) {
	klog.Info("delete one volume from lister: ", volName)
	l.mutex.Lock()
	defer l.mutex.Unlock()
	delete(l.volumeMap, volName)
}
