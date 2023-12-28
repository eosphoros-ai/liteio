package state

import (
	"sync"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/misc"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
)

type Node struct {
	// Info is a pointer of Pool.NodeInfo
	Info *v1.NodeInfo
	Pool *v1.StoragePool
	// Volumes in the Pool
	Volumes []*v1.AntstorVolume
	// FreeResource is calculated free resource of the Pool
	FreeResource corev1.ResourceList
	// 由于volumes数量不会多，所以可以采用遍历方式查询volume;
	// slice 的 append 操作可能是 并发不安全的，详见:
	// https://medium.com/@cep21/gos-append-is-not-always-thread-safe-a3034db7975
	volLock sync.Mutex
	// Reservations set
	resvSet ReservationSetIface
}

func NewNode(pool *v1.StoragePool) *Node {
	node := &Node{
		Info:    &pool.Spec.NodeInfo,
		Pool:    pool,
		Volumes: make([]*v1.AntstorVolume, 0),
		// init Reservations
		resvSet: NewReservationSet(),
	}

	node.FreeResource = node.GetFreeResourceNonLock()

	return node
}

func (n *Node) RemoteVolumesCount(ignoreAnnoSelector map[string]string) (cnt int) {
OUTER:
	for _, item := range n.Volumes {
		// ignore volumes by labels
		for k, v := range ignoreAnnoSelector {
			if val, has := item.Annotations[k]; has && val == v {
				continue OUTER
			}
		}

		if item.Spec.TargetNodeId != "" && item.Spec.HostNode != nil &&
			item.Spec.TargetNodeId != item.Spec.HostNode.ID {
			cnt++
		}
	}
	return
}

func (n *Node) AddVolume(vol *v1.AntstorVolume) (err error) {
	n.volLock.Lock()
	defer n.volLock.Unlock()

	var nodeID = n.Info.ID

	// delete reservation if volume has reservation id
	if resvID := getVolumeReservationID(vol); resvID != "" {
		n.resvSet.Unreserve(resvID)
	}

	// check duplicate
	for _, item := range n.Volumes {
		if item.Name == vol.Name {
			// Should check item.Spec.Type != vol.Spec.Type && item.Spec.Type != "Flexible" ?
			// Check if size not equal to each other
			if item.Spec.SizeByte != vol.Spec.SizeByte {
				klog.Errorf("ErrDuplicateVolume: vol %s already in node %s, but type or size not euqal. ExistingVol: %+v; AddVol: %+v", vol.Name, nodeID, item, vol)
				return ErrDuplicateVolume
			}
			// same vol is present, consider success, replace volume
			vol.Spec.TargetNodeId = n.Pool.Spec.NodeInfo.ID
			// save the newer volume
			*item = *vol.DeepCopy()
			klog.Infof("vol %s already in node %s. type and sizes equal to each other", vol.Name, nodeID)
			return
		}
	}

	n.Volumes = append(n.Volumes, vol)
	// volume reside on Node
	vol.Spec.TargetNodeId = n.Pool.Spec.NodeInfo.ID

	// update free resource
	n.FreeResource = n.GetFreeResourceNonLock()

	return
}

func (n *Node) RemoveVolumeByID(volID string) {
	n.volLock.Lock()
	defer n.volLock.Unlock()

	var idxToDel int
	var foundToDel bool
	for i, item := range n.Volumes {
		if item.Spec.Uuid == volID {
			foundToDel = true
			idxToDel = i
			break
		}
	}

	if foundToDel {
		copy(n.Volumes[idxToDel:], n.Volumes[idxToDel+1:])
		n.Volumes[len(n.Volumes)-1] = nil
		n.Volumes = n.Volumes[:len(n.Volumes)-1]

		// update free resource
		n.FreeResource = n.GetFreeResourceNonLock()
	} else {
		klog.Infof("Not found volID=%s on nodeID=%s, So consider it as removed", volID, n.Info.ID)
	}
}

func (n *Node) GetVolumeByID(volID string) (vol *v1.AntstorVolume, err error) {
	n.volLock.Lock()
	defer n.volLock.Unlock()

	for _, item := range n.Volumes {
		if item.Spec.Uuid == volID {
			vol = item
			break
		}
	}

	if vol == nil {
		err = ErrNotFoundVolumeByID
	}

	return
}

// GetAllocatedLocalBytes 获取已经分配的本地空间
func (n *Node) GetAllocatedLocalBytes() (size uint64) {
	for _, item := range n.Volumes {
		// sumup local volume size
		if item.Spec.HostNode != nil && item.Spec.TargetNodeId == item.Spec.HostNode.ID {
			size += item.GetTotalSize()
		}
	}
	return
}

// GetAllocatedRemoteBytes 获取已经分配的远程空间
func (n *Node) GetAllocatedRemoteBytes() (size uint64) {
	for _, item := range n.Volumes {
		// sumup remote volume size
		if item.Spec.HostNode != nil && item.Spec.TargetNodeId != item.Spec.HostNode.ID {
			size += item.GetTotalSize()
		}
	}
	return
}

// GetReservedVolBytes sum up size of reserved logical volume
func (n *Node) GetReservedVolBytes() (size uint64) {
	// minus reserved static lvol
	for _, item := range n.Pool.Spec.KernelLVM.ReservedLVol {
		size += item.SizeByte
	}
	return
}

// GetFreeResourceNonLock return free resource without lock
func (n *Node) GetFreeResourceNonLock() (free corev1.ResourceList) {
	free = make(corev1.ResourceList)
	for key, val := range n.Pool.Status.Capacity {
		free[key] = val.DeepCopy()
	}

	var (
		volResvIDs   = misc.NewEmptySet()
		toMunisBytes int64
	)

	// minus reserved static lvol
	for _, item := range n.Pool.Spec.KernelLVM.ReservedLVol {
		sizeByte := item.SizeByte
		if _, has := free[v1.ResourceDiskPoolByte]; has {
			toMunisBytes += int64(sizeByte)
		}
	}

	for _, vol := range n.Volumes {
		// minus (volume size + snap reserved size)
		sizeByte := vol.GetTotalSize()
		resvID := getVolumeReservationID(vol)
		if _, has := free[v1.ResourceDiskPoolByte]; has {
			volResvIDs.Add(resvID)
			toMunisBytes += int64(sizeByte)
		}

		/*
			if q, has := free[v1.ResourceVolumesCount]; has {
				q.Sub(*resource.NewQuantity(int64(1), resource.DecimalSI))
				free[v1.ResourceVolumesCount] = q
			}
		*/
	}

	// minus volume reservation
	for _, item := range n.resvSet.Items() {
		// if ID is in the resvIDs, do not count twice.
		resvID := item.ID()
		if !volResvIDs.Contains(resvID) {
			toMunisBytes += item.Size()
		} else {
			// the corresponding Volume is binded, so clean the Reservation
			n.resvSet.Unreserve(resvID)
		}
	}

	// do minus
	if q, has := free[v1.ResourceDiskPoolByte]; has {
		q.Sub(*resource.NewQuantity(toMunisBytes, resource.BinarySI))
		free[v1.ResourceDiskPoolByte] = q
	}

	return
}

// Reserve storage resource for Node
func (n *Node) Reserve(r ReservationIface) {
	// if volume is already binded, then skip reservation.
	var resvID = r.ID()
	for _, vol := range n.Volumes {
		if resvID == getVolumeReservationID(vol) {
			return
		}
	}

	// check free resource
	if free := n.FreeResource.Storage(); free != nil {
		if free.CmpInt64(r.Size()) < 0 {
			klog.Errorf("node %s have no enough disk pool space for reservation %s", n.Info.ID, resvID)
			return
		}
	}

	n.volLock.Lock()
	defer n.volLock.Unlock()

	n.resvSet.Reserve(r)
	// update free resource
	n.FreeResource = n.GetFreeResourceNonLock()
}

// Unreserve storage resource
func (n *Node) Unreserve(id string) {
	n.volLock.Lock()
	defer n.volLock.Unlock()

	n.resvSet.Unreserve(id)
	// update free resource
	n.FreeResource = n.GetFreeResourceNonLock()
}

func (n *Node) GetReservation(id string) (r ReservationIface, has bool) {
	return n.resvSet.GetById(id)
}
