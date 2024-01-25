package spdk

import (
	"fmt"
	"os"
	"testing"

	spdkmock "lite.io/liteio/pkg/generated/mocks/spdk"
	"lite.io/liteio/pkg/spdk/jsonrpc/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSpdkServiceAioBdev(t *testing.T) {
	fakeCli := spdkmock.NewSPDKClientIface(t)
	fakeCli.On("NVMFGetTransports").Return(nil, nil).
		On("NVMFCreateTransport", mock.Anything).Return(true, nil).
		On("NVMFGetSubsystems", mock.Anything).Return(nil, nil).
		On("BdevGetBdevs", mock.Anything).Return(nil, nil).
		On("BdevAioCreate", mock.Anything).Return(func(req client.BdevAioCreateReq) string {
		return req.BdevName
	}, nil)

	svc, err := NewSpdkService(SpdkServiceConfig{
		CliGenFn: func() (client.SPDKClientIface, error) {
			return fakeCli, nil
		},
	})
	assert.NoError(t, err)
	assert.NotNil(t, svc)

	port, err := svc.idAlloc.NextID()
	assert.NoError(t, err)
	assert.Equal(t, MinSvcID, port)

	err = svc.idAlloc.SyncFromTruth()
	assert.NoError(t, err)

	err = svc.CreateAioBdev(AioBdevCreateRequest{
		BdevName:  "bdevName",
		DevPath:   "/dev/vg/lv",
		BlockSize: 512,
	})
	assert.Error(t, err)

	fileName := "/tmp/mock-dev-file"
	_, err = os.Create(fileName)
	assert.NoError(t, err)
	defer os.Remove(fileName)
	err = svc.CreateAioBdev(AioBdevCreateRequest{
		"bdevName", fileName, 512,
	})
	assert.NoError(t, err)

	err = svc.DeleteAioBdev(AioBdevDeleteRequest{"bdevName"})
	assert.NoError(t, err)
}

func TestSpdkService(t *testing.T) {
	svc, fakeCli := newSpdkServiceWithFakeClient(t)
	fakeCli.On("BdevGetBdevs", mock.Anything).Return([]Bdev{
		Bdev{
			Name: "test-bdev",
			UUID: "uuid-xxx",
		},
	}, nil)

	list, err := svc.BdevGetBdevs(BdevGetBdevsReq{BdevName: "test-bdev"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(list))
}

func newSpdkServiceWithFakeClient(t *testing.T) (*SpdkService, *spdkmock.SPDKClientIface) {
	fakeCli := spdkmock.NewSPDKClientIface(t)
	fakeCli.On("NVMFGetTransports").Return(nil, nil).
		On("NVMFCreateTransport", mock.Anything).Return(true, nil).
		On("NVMFGetSubsystems", mock.Anything).Return(nil, nil)

	svc, _ := NewSpdkService(SpdkServiceConfig{
		CliGenFn: func() (client.SPDKClientIface, error) {
			return fakeCli, nil
		},
	})

	return svc, fakeCli
}

func TestIsNotFound(t *testing.T) {
	assert.False(t, IsNotFoundDeviceError(nil))
	assert.False(t, IsNotFoundDeviceError(fmt.Errorf("xxx")))
	assert.True(t, IsNotFoundDeviceError(client.RPCError{Code: client.ErrorCodeNoDevice}))
}

func TestParseLvolFullName(t *testing.T) {
	lvs, lvol := parseLvolFullName("lvs/lvol")
	assert.Equal(t, "lvs", lvs)
	assert.Equal(t, "lvol", lvol)

	lvs, lvol = parseLvolFullName("lvs/")
	assert.Equal(t, "lvs", lvs)
	assert.Equal(t, "", lvol)

	lvs, lvol = parseLvolFullName("")
	assert.Equal(t, "", lvs)
	assert.Equal(t, "", lvol)

	lvs, lvol = parseLvolFullName("lvs/lvol/xxx")
	assert.Equal(t, "lvs", lvs)
	assert.Equal(t, "lvol/xxx", lvol)
}
