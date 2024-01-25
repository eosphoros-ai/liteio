package spdk

import (
	"fmt"
	"strconv"
	"sync"

	"lite.io/liteio/pkg/spdk/jsonrpc/client"
	"lite.io/liteio/pkg/util/misc"
)

type SvcIdAllocator struct {
	// cursor increases from minId to maxId
	cursor int
	inUse  misc.Set
	lock   sync.Mutex
	minId  int
	maxId  int
	// subsysReader can list all subsys from spdk
	subsysReader client.SpdkSubsystemReader
}

func (a *SvcIdAllocator) FreeID(svcId int) {
	// lock for concurrency
	a.lock.Lock()
	defer a.lock.Unlock()

	a.inUse.Remove(strconv.Itoa(svcId))
}

func (a *SvcIdAllocator) NextID() (int, error) {
	// lock for concurrency
	a.lock.Lock()
	defer a.lock.Unlock()
	var found bool
	var count int

	if a.inUse.Size() == 0 {
		a.cursor = a.minId
		a.inUse.Add(strconv.Itoa(a.cursor))
		return a.cursor, nil
	}

	for !found {
		a.cursor += 1
		// In TCP/IP mode, svc_id represents a TCP port. Port range is from minId to maxId.
		if a.cursor > a.maxId {
			a.cursor = a.minId
		}
		// need to record these used port.
		// 需要记录使用了哪些 port, 不可重复, 否则会出现创建listener失败
		svcID := strconv.Itoa(a.cursor)
		if !a.inUse.Contains(svcID) {
			found = true
		}
		// 判断count，是否已经从Min->Max遍历完成
		count++
		if count >= (a.maxId - a.minId + 1) {
			err := fmt.Errorf("cannot find available port (svcID) from %d to %d", a.minId, a.maxId)
			return 0, err
		}
	}

	if found {
		a.inUse.Add(strconv.Itoa(a.cursor))
	}

	return a.cursor, nil
}

func (a *SvcIdAllocator) SyncFromTruth() (err error) {
	if a.subsysReader == nil {
		return ErrorSpdkConnectionLost
	}

	a.lock.Lock()
	defer a.lock.Unlock()

	// empty the set
	a.inUse = misc.NewEmptySet()

	// collect svcID in use
	list, err := a.subsysReader.NVMFGetSubsystems()
	if err != nil {
		return
	}
	for _, item := range list {
		for _, listener := range item.ListenAddresses {
			a.inUse.Add(listener.TrSvcID)
		}
	}

	return
}
