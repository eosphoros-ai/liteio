package filter

import (
	"strconv"
	"strings"
	"sync"
)

const (
	ReasonPoolFreeSize      = "PoolFreeSize"
	ReasonSpdkUnhealthy     = "SpdkUnhealthy"
	ReasonRemoteVolMaxCount = "RemoteVolMaxCount"
	ReasonPositionNotMatch  = "PositionNotMatch"
	ReasonVolTypeNotMatch   = "VolTypeNotMatch"
	ReasonDataConflict      = "DataConflict"
	ReasonNodeAffinity      = "NodeAffinity"
	ReasonPoolAffinity      = "PoolAffinity"
	ReasonPoolUnschedulable = "PoolUnschedulable"
	ReasonReservationSize   = "ReservationTooSmall"

	NoStoragePoolAvailable = "NoStoragePoolAvailable"
	//
	CtxErrKey = "globalError"
)

type MergedError struct {
	// reason -> count
	reasons map[string]int
	lock    sync.Mutex
}

func NewMergedError() *MergedError {
	return &MergedError{
		reasons: map[string]int{},
	}
}

func (e *MergedError) Error() string {
	var s strings.Builder
	s.WriteString(NoStoragePoolAvailable + ": ")

	e.lock.Lock()
	defer e.lock.Unlock()
	for reason, cnt := range e.reasons {
		s.WriteString(reason + ": " + strconv.Itoa(cnt) + ", ")
	}

	return s.String()
}

func (e *MergedError) AddReason(reason string) {
	e.lock.Lock()
	defer e.lock.Unlock()

	if cnt, has := e.reasons[reason]; has {
		e.reasons[reason] = cnt + 1
	} else {
		e.reasons[reason] = 1
	}
}

func IsNoStoragePoolAvailable(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), NoStoragePoolAvailable)
}
