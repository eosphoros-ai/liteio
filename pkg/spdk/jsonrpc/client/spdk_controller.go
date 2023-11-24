package client

import (
	"encoding/json"
	"errors"
)

const (
	TrTypePCIe = "PCIe"
)

type SpdkControllerIface interface {
	// bdev_nvme_get_controllers
	ListControllers() (list []ControllerInfo, err error)
	// bdev_nvme_attach_controller
	AttachController(req AttachControllerRequest) (names []string, err error)
	// bdev_nvme_detach_controller
	DetachController(req DetachControllerRequest) (err error)
}

type AttachControllerRequest struct {
	// NVMe-oF target trtype: e.g., rdma, pcie, tcp
	TrType string `json:"trtype"`
	// NVMe-oF target address: e.g., an ip address or BDF
	TrAddr string `json:"traddr"`
	// Name of the NVMe controller, prefix for each bdev name
	Name string `json:"name"`
	// optional
	// NVMe-oF target adrfam: ipv4, ipv6, ib, fc, intra_host
	AdrFam string `json:"adrfam,omitempty"`
	// NVMe-oF target trsvcid: port number
	TrSvcId string `json:"trsvcid,omitempty"`
	// NVMe-oF target subnqn
	SubNQN string `json:"subnqn,omitempty"`
	// NVMe-oF target hostnqn, NOT USED
	HostNQN string `json:"hostnqn,omitempty"`
	// NVMe-oF host address: ip address, NOT USED
	HostAddr string `json:"hostaddr,omitempty"`
	// NVMe-oF host trsvcid: port number, NOT USED
	HostSvcId string `json:"hostsvcid,omitempty"`
}

type DetachControllerRequest struct {
	Name string `json:"name"`
}

type TrInfo struct {
	TrType string `json:"trtype"`
	TrAddr string `json:"traddr"`
}

type ControllerInfo struct {
	Name      string `json:"name"`
	TrID      TrInfo `json:"trid"`
	ControlID string `json:"cntlid"`
}

func (s *SPDK) AttachController(req AttachControllerRequest) (names []string, err error) {
	bs, err := s.rawCli.Call("bdev_nvme_attach_controller", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(bs, &names)
	return
}

func (s *SPDK) ListControllers() (list []ControllerInfo, err error) {
	bs, err := s.rawCli.Call("bdev_nvme_get_controllers", nil)
	if err != nil {
		return
	}
	err = json.Unmarshal(bs, &list)
	return
}

func (s *SPDK) DetachController(req DetachControllerRequest) (err error) {
	bs, err := s.rawCli.Call("bdev_nvme_detach_controller", req)
	if err != nil {
		err = errors.New(err.Error() + string(bs))
		return
	}
	return
}
