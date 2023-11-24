package spdk

import (
	"errors"
	"fmt"
	"strings"

	"code.alipay.com/dbplatform/node-disk-controller/pkg/spdk/jsonrpc/client"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/misc"
	"k8s.io/klog/v2"
)

const (
	SpdkController = "SPKD_Controller"

	DefaultSockFile = "/usr/tmp/spdk.sock"

	BdevRaidName = "antstor_raid0"
	LVStoreName  = "antstor_lvstore"

	MinSvcID = 4510
	MaxSvcID = 4610
)

var (
	// SpdkService implements SpdkServiceIface
	_ SpdkServiceIface = &SpdkService{}
	// ErrorSpdkConnectionLost represents error when connecting to spdk JSON-RPC service
	ErrorSpdkConnectionLost = fmt.Errorf("SpdkConnectionLost")
)

type BdevGetBdevsReq = client.BdevGetBdevsReq
type BdevGetIostatReq = client.BdevGetIostatReq
type Bdev = client.Bdev
type BdevIostats = client.BdevIostats

type SpdkServiceIface interface {
	AioServiceIface
	TargetServiceIface
	Reconnector
	LVolServiceIface
	MigrateServiceIface
	SpdkVersionIface
	MallocServiceIface
	BdevServiceIface
}

type Reconnector interface {
	Reconnect() (err error)
}

type BdevServiceIface interface {
	BdevGetBdevs(req BdevGetBdevsReq) (list []Bdev, err error)
	BdevGetIostat(req BdevGetIostatReq) (result BdevIostats, err error)
}

type ClientGeneratorFnType func() (client.SPDKClientIface, error)

func NewWithDefaultSock() (client.SPDKClientIface, error) {
	rawCli, err := client.NewClient(DefaultSockFile)
	if err != nil {
		klog.Error(err)
		return nil, ErrorSpdkConnectionLost
	}
	return client.NewSPDK(rawCli), nil
}

type SpdkServiceConfig struct {
	CliGenFn     ClientGeneratorFnType
	AllowAnyHost bool
}

type SpdkService struct {
	// cliGenFn ClientGeneratorFnType
	Cfg     SpdkServiceConfig
	cli     client.SPDKClientIface
	idAlloc *SvcIdAllocator
}

func NewSpdkService(cfg SpdkServiceConfig) (svc *SpdkService, err error) {
	if cfg.CliGenFn == nil {
		return nil, fmt.Errorf("cliGen cannot be nil")
	}

	svc = &SpdkService{
		Cfg: cfg,
		idAlloc: &SvcIdAllocator{
			cursor: MinSvcID,
			inUse:  misc.NewEmptySet(),
			minId:  MinSvcID,
			maxId:  MaxSvcID,
		},
	}

	err = svc.Reconnect()
	return
}

// Reconnect try to connect to spdk service by func cliGen. It passes connected client to SpdkService and SvcIdAllocator.
func (svc *SpdkService) Reconnect() (err error) {
	svc.cli, err = svc.Cfg.CliGenFn()
	if err != nil {
		return
	}
	svc.idAlloc.subsysReader = svc.cli

	err = svc.InitTransport()
	if err != nil {
		klog.Error(err)
		return
	}
	err = svc.idAlloc.SyncFromTruth()
	return
}

func (svc *SpdkService) BdevGetBdevs(req BdevGetBdevsReq) (list []Bdev, err error) {
	list, err = svc.cli.BdevGetBdevs(req)
	if err != nil {
		klog.Error(err)
	}
	return
}

func (svc *SpdkService) BdevGetIostat(req BdevGetIostatReq) (iostats BdevIostats, err error) {
	var cli client.SPDKClientIface
	cli, err = svc.client()
	if err != nil {
		err = fmt.Errorf("client is nil, try to reconnect failed, %w", err)
		klog.Error(err)
		return
	}
	iostats, err = cli.BdevGetIostat(req)
	if err != nil {
		err = fmt.Errorf("get bdev iostat failed, %w", err)
		klog.Error(err)
	}
	return
}

func (svc *SpdkService) client() (client.SPDKClientIface, error) {
	if svc.cli == nil {
		err := svc.Reconnect()
		return svc.cli, err
	}
	return svc.cli, nil
}

func IsNotFoundDeviceError(err error) bool {
	// return strings.Contains(err.Error(), "No such device")
	var rpcErr client.RPCError
	if errors.As(err, &rpcErr) {
		return rpcErr.Code == client.ErrorCodeNoDevice
	}
	return false
}

func parseLvolFullName(name string) (lvs, lvol string) {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) == 2 {
		lvs = parts[0]
		lvol = parts[1]
	} else {
		lvol = parts[0]
	}

	return
}
