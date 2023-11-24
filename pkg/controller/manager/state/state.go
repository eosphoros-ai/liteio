package state

import (
	"errors"
	"fmt"
	"sync"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
)

var (
	ErrDuplicateVolume    = errors.New("DuplicatedVolume")
	ErrNotFoundVolumeByID = errors.New("ErrNotFoundVolumeByID")
	ErrNotFoundNode       = errors.New("NotFoundNode")

	_ StateIface = &state{}
)

type StateIface interface {
	GetAllNodes() (list []*Node)

	// pool
	GetNodeByNodeID(nodeID string) (node *Node, err error)
	GetStoragePoolByNodeID(nodeID string) (pool *v1.StoragePool, err error)
	SetStoragePool(pool *v1.StoragePool)
	RemoveStoragePool(nodeID string) (err error)
	UpdateStoragePoolStatus(nodeID string, status v1.PoolStatus) (err error)

	// volume
	GetVolumeByID(volID string) (vol *v1.AntstorVolume, err error)
	FindVolumesByNodeID(nodeID string) (vols []*v1.AntstorVolume, err error)
	BindAntstorVolume(nodeID string, vol *v1.AntstorVolume) (err error)
	UnbindAntstorVolume(volID string) (err error)
}

type state struct {
	// id -> node
	NodeMap map[string]*Node
	// volume 索引: volumeID -> nodeID
	volIDMap map[string]string
	lock     sync.RWMutex
}

func NewState() StateIface {
	return &state{
		NodeMap:  make(map[string]*Node),
		volIDMap: make(map[string]string),
	}
}

func (s *state) GetAllNodes() (list []*Node) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	list = make([]*Node, 0, len(s.NodeMap))
	for _, item := range s.NodeMap {
		list = append(list, item)
	}
	return
}

func (s *state) FindVolumesByNodeID(nodeID string) (vols []*v1.AntstorVolume, err error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	node, has := s.NodeMap[nodeID]
	if has {
		vols = node.Volumes
	} else {
		err = newNotFoundNodeError(nodeID)
	}

	return
}

func (s *state) GetNodeByNodeID(nodeID string) (node *Node, err error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	var has bool
	node, has = s.NodeMap[nodeID]
	if !has {
		err = newNotFoundNodeError(nodeID)
		return
	}

	return
}

func (s *state) GetStoragePoolByNodeID(nodeID string) (pool *v1.StoragePool, err error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	node, has := s.NodeMap[nodeID]
	if has && node != nil {
		pool = node.Pool
	}

	if pool == nil {
		err = newNotFoundNodeError(nodeID)
	}

	return
}

// SetStoragePool update Pool in Node.
func (s *state) SetStoragePool(pool *v1.StoragePool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	klog.Infof("Update pool=%s/%s", pool.Namespace, pool.Name)

	if pool.Status.Capacity == nil {
		pool.Status.Capacity = make(corev1.ResourceList)
	}

	var hasStorageCapacity bool
	_, hasStorageCapacity = pool.Status.Capacity[v1.ResourceDiskPoolByte]

	// set pool status capacity according to Pool type
	totalVgBytes := pool.GetVgTotalBytes()
	if totalVgBytes > 0 && !hasStorageCapacity {
		quant := resource.NewQuantity(totalVgBytes, resource.BinarySI)
		pool.Status.Capacity[v1.ResourceDiskPoolByte] = *quant
	}

	if val, has := s.NodeMap[pool.Spec.NodeInfo.ID]; has {
		// update fields
		val.Pool = pool.DeepCopy()
		val.Info = &val.Pool.Spec.NodeInfo
		val.FreeResource = val.GetFreeResourceNonLock()
	} else {
		s.NodeMap[pool.Spec.NodeInfo.ID] = NewNode(pool.DeepCopy())
	}
}

func (s *state) UpdateStoragePoolStatus(nodeID string, status v1.PoolStatus) (err error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	val, has := s.NodeMap[nodeID]
	if has {
		val.Pool.Status.Status = status
	}
	return
}

func (s *state) RemoveStoragePool(nodeID string) (err error) {
	s.lock.Lock()
	delete(s.NodeMap, nodeID)
	s.lock.Unlock()
	return
}

func (s *state) BindAntstorVolume(nodeID string, vol *v1.AntstorVolume) (err error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	node, has := s.NodeMap[nodeID]
	if !has {
		err = newNotFoundNodeError(nodeID)
		return
	}

	// ID cannot be empty
	if vol.Spec.Uuid == "" {
		err = fmt.Errorf("volume name=%s, volume UUID cannot be empty", vol.Name)
		return
	}

	err = node.AddVolume(vol.DeepCopy())
	if err != nil {
		return
	}

	// index node
	s.volIDMap[vol.Spec.Uuid] = nodeID

	return
}

func (s *state) UnbindAntstorVolume(volID string) (err error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	nodeID, has := s.volIDMap[volID]
	if !has {
		err = fmt.Errorf("cannot find Node by volID %s", volID)
		return
	}

	node, has := s.NodeMap[nodeID]
	if !has {
		err = fmt.Errorf("not found node by id %s", nodeID)
		return
	}

	node.RemoveVolumeByID(volID)

	// remove index
	delete(s.volIDMap, volID)

	return
}

func (s *state) GetVolumeByID(volID string) (vol *v1.AntstorVolume, err error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	nodeID, has := s.volIDMap[volID]
	if !has {
		err = fmt.Errorf("cannot find Node by volID %s", volID)
		return
	}

	node, has := s.NodeMap[nodeID]
	if !has {
		err = fmt.Errorf("not found node by id %s", nodeID)
		return
	}

	return node.GetVolumeByID(volID)
}

func newNotFoundNodeError(id string) error {
	return fmt.Errorf("%w by id %s", ErrNotFoundNode, id)
}

func IsNotFoundNodeError(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, ErrNotFoundNode)
}

/*
func (s *state) GetFreePortForSPDK(nodeID string) (port int, err error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	node, has := s.NodeMap[nodeID]
	if !has {
		err = newNotFoundNodeError(nodeID)
		return
	}

	var usedPorts []int
	var min = SpdkSvcIDRange[0]
	var max = SpdkSvcIDRange[1]

	for _, item := range node.Volumes {
		if item.Spec.SpdkTarget != nil {
			port, errAtoi := strconv.Atoi(item.Spec.SpdkTarget.SvcID)
			if errAtoi != nil {
				klog.Errorf("parse %s to int error, vol=%+v, err=%+v", item.Spec.SpdkTarget.SvcID, *item, errAtoi)
				continue
			}
			usedPorts = append(usedPorts, port)
		}
	}

	for i := min; i < max; i++ {
		if !misc.InSliceInt(i, usedPorts) {
			return i, nil
		}
	}

	return -1, nil
}

*/
