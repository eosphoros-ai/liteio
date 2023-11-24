package client

import (
	"encoding/json"
)

var (
	_ SPDKClientIface = &SPDK{}
)

const (
	ErrorCodeNoDevice = -19
)

type SPDKClientIface interface {
	GetRawClient() JsonRpcClientIface
	// RpcGetMethods rpc_get_methods
	RpcGetMethods() ([]string, error)

	// nvmf_get_transports
	NVMFGetTransports() (result []Transport, err error)
	// nvmf_create_transport
	NVMFCreateTransport(req NVMFCreateTransportReq) (result bool, err error)

	//bdev_get_bdevs
	BdevGetBdevs(req BdevGetBdevsReq) (result []Bdev, err error)
	// bdev_get_iostat
	BdevGetIostat(req BdevGetIostatReq) (iostats BdevIostats, err error)

	// BdevAioCreate bdev_aio_create, return the name of bdev
	BdevAioCreate(req BdevAioCreateReq) (name string, err error)
	// bdev_aio_delete
	BdevAioDelete(req BdevAioDeleteReq) (result bool, err error)
	// bdev_aio_resize
	BdevAioResize(req BdevAioResizeReq) (result bool, err error)

	// framework_get_config
	FrameworkGetConfig(req FrameworkGetConfigReq) (result []FrameworkGetConfigItem, err error)

	// spdk_get_version
	GetSpdkVersion() (ver SpdkVersion, err error)

	SpdkLvolIface
	SpdkSubsystemIface
	SpdkControllerIface
	SpdkBdevRaidIface
	SpdkMigrateIface
	SpdkMallocIface
}

type SPDK struct {
	rawCli JsonRpcClientIface
}

func NewSPDK(rawCli JsonRpcClientIface) *SPDK {
	return &SPDK{
		rawCli: rawCli,
	}
}

func (s *SPDK) GetRawClient() JsonRpcClientIface {
	return s.rawCli
}

// nvmf_get_transports
func (s *SPDK) NVMFGetTransports() (list []Transport, err error) {
	result, err := s.rawCli.Call("nvmf_get_transports", nil)
	if err != nil {
		return
	}

	err = json.Unmarshal(result, &list)
	return
}

// bdev_get_bdevs
func (s *SPDK) BdevGetBdevs(req BdevGetBdevsReq) (list []Bdev, err error) {
	result, err := s.rawCli.Call("bdev_get_bdevs", req)
	if err != nil {
		return
	}

	err = json.Unmarshal(result, &list)
	return
}

func (s *SPDK) BdevGetIostat(req BdevGetIostatReq) (iostats BdevIostats, err error) {
	result, err := s.rawCli.Call("bdev_get_iostat", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(result, &iostats)
	return
}

func (s *SPDK) RpcGetMethods() (methods []string, err error) {
	result, err := s.rawCli.Call("rpc_get_methods", nil)
	if err != nil {
		return
	}

	err = json.Unmarshal(result, &methods)
	return
}

func (s *SPDK) BdevAioCreate(req BdevAioCreateReq) (name string, err error) {
	result, err := s.rawCli.Call("bdev_aio_create", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(result, &name)
	return
}

func (s *SPDK) NVMFCreateTransport(req NVMFCreateTransportReq) (res bool, err error) {
	result, err := s.rawCli.Call("nvmf_create_transport", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(result, &res)
	return
}

// bdev_aio_delete
func (s *SPDK) BdevAioDelete(req BdevAioDeleteReq) (res bool, err error) {
	result, err := s.rawCli.Call("bdev_aio_delete", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(result, &res)
	return
}

func (s *SPDK) BdevAioResize(req BdevAioResizeReq) (res bool, err error) {
	result, err := s.rawCli.Call("bdev_aio_resize", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(result, &res)
	return
}

// framework_get_config
func (s *SPDK) FrameworkGetConfig(req FrameworkGetConfigReq) (result []FrameworkGetConfigItem, err error) {
	bs, err := s.rawCli.Call("framework_get_config", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(bs, &result)
	return
}

// spdk_get_version
func (s *SPDK) GetSpdkVersion() (ver SpdkVersion, err error) {
	bs, err := s.rawCli.Call("spdk_get_version", nil)
	if err != nil {
		return
	}
	err = json.Unmarshal(bs, &ver)
	return
}
