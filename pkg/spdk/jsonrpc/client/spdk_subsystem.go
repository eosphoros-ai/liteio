package client

import "encoding/json"

type SpdkSubsystemReader interface {
	// nvmf_get_subsystems
	NVMFGetSubsystems() (result []Subsystem, err error)
}

type SpdkSubsystemIface interface {
	SpdkSubsystemReader
	// nvmf_delete_subsystem
	NVMFDeleteSubsystem(req NVMFDeleteSubsystemReq) (result bool, err error)
	// nvmf_create_subsystem
	NVMFCreateSubsystem(req NVMFCreateSubsystemReq) (result bool, err error)
	// nvmf_subsystem_add_ns
	NVMFSubsystemAddNS(req NVMFSubsystemAddNSReq) (nsID int, err error)
	// nvmf_subsystem_add_listener
	NVMFSubsystemAddListener(req NVMFSubsystemAddListenerReq) (result bool, err error)
	// framework_get_subsystems
	FrameworkGetSubsystems() (result []FrameworkGetSubsystemsItem, err error)
	// nvmf_get_stats
	NVMFGetStats() (result SubsystemStat, err error)
	// nvmf_subsystem_add_host
	NVMFSubsystemAddHost(req NVMFSubsystemAddHostReq) (result bool, err error)
}

// nvmf_get_subsystems
func (s *SPDK) NVMFGetSubsystems() (list []Subsystem, err error) {
	result, err := s.rawCli.Call("nvmf_get_subsystems", nil)
	if err != nil {
		return
	}

	err = json.Unmarshal(result, &list)
	return
}

func (s *SPDK) NVMFCreateSubsystem(req NVMFCreateSubsystemReq) (res bool, err error) {
	result, err := s.rawCli.Call("nvmf_create_subsystem", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(result, &res)
	return
}

// nvmf_subsystem_add_ns
func (s *SPDK) NVMFSubsystemAddNS(req NVMFSubsystemAddNSReq) (nsID int, err error) {
	result, err := s.rawCli.Call("nvmf_subsystem_add_ns", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(result, &nsID)
	return
}

// nvmf_subsystem_add_listener
func (s *SPDK) NVMFSubsystemAddListener(req NVMFSubsystemAddListenerReq) (res bool, err error) {
	result, err := s.rawCli.Call("nvmf_subsystem_add_listener", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(result, &res)
	return
}

// nvmf_delete_subsystem
func (s *SPDK) NVMFDeleteSubsystem(req NVMFDeleteSubsystemReq) (res bool, err error) {
	result, err := s.rawCli.Call("nvmf_delete_subsystem", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(result, &res)
	return
}

// framework_get_subsystems
func (s *SPDK) FrameworkGetSubsystems() (result []FrameworkGetSubsystemsItem, err error) {
	bs, err := s.rawCli.Call("framework_get_subsystems", nil)
	if err != nil {
		return
	}

	err = json.Unmarshal(bs, &result)
	return
}

func (s *SPDK) NVMFGetStats() (result SubsystemStat, err error) {
	bs, err := s.rawCli.Call("nvmf_get_stats", nil)
	if err != nil {
		return
	}
	err = json.Unmarshal(bs, &result)
	return
}

func (s *SPDK) NVMFSubsystemAddHost(req NVMFSubsystemAddHostReq) (res bool, err error) {
	bs, err := s.rawCli.Call("nvmf_subsystem_add_host", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(bs, &res)
	return
}
