package misc

import (
	"sync"

	"k8s.io/apimachinery/pkg/util/sets"
)

type ResourceLockIface interface {
	TryAcquire(resourceID string) bool
	Release(resourceID string)
}

// ResourceLocks implements a map with atomic operations. It stores a set of all resource IDs
// with an ongoing operation.
type ResourceLocks struct {
	locks sets.String
	mux   sync.Mutex
}

func NewResourceLocks() *ResourceLocks {
	return &ResourceLocks{
		locks: sets.NewString(),
	}
}

// TryAcquire tries to acquire the lock for operating on resourceID and returns true if successful.
// If another operation is already using resourceID, returns false.
func (lock *ResourceLocks) TryAcquire(resourceID string) bool {
	lock.mux.Lock()
	defer lock.mux.Unlock()
	if lock.locks.Has(resourceID) {
		return false
	}
	lock.locks.Insert(resourceID)
	return true
}

func (lock *ResourceLocks) Release(resourceID string) {
	lock.mux.Lock()
	defer lock.mux.Unlock()
	lock.locks.Delete(resourceID)
}
