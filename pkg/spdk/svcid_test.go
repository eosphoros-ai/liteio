package spdk

import (
	"testing"

	spdkmock "lite.io/liteio/pkg/generated/mocks/spdk"
	"lite.io/liteio/pkg/util/misc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSvcIDAllocator(t *testing.T) {
	cli := spdkmock.NewSPDKClientIface(t)
	cli.Mock.On("NVMFGetSubsystems", mock.Anything).Return(nil, nil)
	maxID := MinSvcID + 10

	alloc := SvcIdAllocator{
		cursor:       MinSvcID,
		minId:        MinSvcID,
		maxId:        maxID,
		inUse:        misc.NewEmptySet(),
		subsysReader: cli,
	}

	err := alloc.SyncFromTruth()
	assert.NoError(t, err)

	for i := MinSvcID; i <= maxID; i++ {
		id, err := alloc.NextID()
		assert.NoError(t, err)
		assert.Equal(t, i, id)
	}

	id, err := alloc.NextID()
	assert.Error(t, err, id)

	alloc.FreeID(MinSvcID)
	id, err = alloc.NextID()
	assert.NoError(t, err)
	assert.Equal(t, MinSvcID, id)
}
