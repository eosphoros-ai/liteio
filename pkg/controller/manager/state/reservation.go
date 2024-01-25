package state

import (
	"fmt"
	"math"
	"sync"

	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	corev1 "k8s.io/api/core/v1"
)

type ReservationSetIface interface {
	Reserve(r ReservationIface)
	Unreserve(id string)
	Items() (list []ReservationIface)
	GetById(id string) (r ReservationIface, has bool)
}

type ReservationIface interface {
	ID() string
	Size() int64
	NamespacedName() string
}

type reservationSet struct {
	lock sync.Mutex
	// Reservations map, Reservation.ID => Reservation
	rMap map[string]ReservationIface
}

func NewReservationSet() *reservationSet {
	return &reservationSet{
		lock: sync.Mutex{},
		rMap: make(map[string]ReservationIface),
	}
}

func (rs *reservationSet) Reserve(r ReservationIface) {
	rs.lock.Lock()
	defer rs.lock.Unlock()
	rs.rMap[r.ID()] = r
}

func (rs *reservationSet) Unreserve(id string) {
	rs.lock.Lock()
	defer rs.lock.Unlock()
	delete(rs.rMap, id)
}

func (rs *reservationSet) Items() (list []ReservationIface) {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	for _, item := range rs.rMap {
		list = append(list, item)
	}

	return list
}

func (rs *reservationSet) GetById(id string) (r ReservationIface, has bool) {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	for _, item := range rs.rMap {
		if item.ID() == id {
			return item, true
		}
	}

	return nil, false
}

type reservation struct {
	id             string
	namespacedName string
	sizeByte       int64
}

func NewReservation(id string, size int64) ReservationIface {
	return &reservation{
		id:       id,
		sizeByte: size,
	}
}

func NewPvcReservation(pvc *corev1.PersistentVolumeClaim) ReservationIface {
	if pvc.DeletionTimestamp != nil {
		return nil
	}

	pvcName := fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name)
	size := int64(math.Round(pvc.Spec.Resources.Requests.Storage().AsApproximateFloat64()))
	resv := &reservation{
		id:             pvcName,
		namespacedName: pvcName,
		sizeByte:       size,
	}

	return resv
}

func (r *reservation) ID() string {
	return r.id
}

func (r *reservation) NamespacedName() string {
	return r.namespacedName
}

func (r *reservation) Size() int64 {
	return r.sizeByte
}

func getVolumeReservationID(vol *v1.AntstorVolume) (id string) {
	if resvId, has := vol.Annotations[v1.ReservationIDKey]; has {
		return resvId
	}

	if vol.Labels != nil {
		pvcNS := vol.Labels[v1.VolumeContextKeyPvcNS]
		pvcName := vol.Labels[v1.VolumeContextKeyPvcName]

		if pvcNS != "" && pvcName != "" {
			id = pvcNS + "/" + pvcName
		}
	}

	return
}
