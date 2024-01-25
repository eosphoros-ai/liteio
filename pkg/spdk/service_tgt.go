package spdk

import (
	"fmt"
	"strconv"

	"lite.io/liteio/pkg/spdk/jsonrpc/client"
	"k8s.io/klog/v2"
)

type TargetCreateRequest struct {
	BdevName   string
	TargetInfo Target
}

type TargetCreateResponse struct {
	NQN string
	// SvcID is needed for metadata
	SvcID string
}

type SubsystemStatResp struct {
	SubsysName       string
	BytesRead        uint64
	NumReadOps       uint64
	BytesWrite       uint64
	NumWriteOps      uint64
	ReadLatencyTime  uint64 // unit: us
	WriteLatencyTime uint64 // unit: us
	TimeInQueue      uint64 // unit: us
}

type Target struct {
	NQN          string
	SerialNumber string
	// namespace uuid
	NSUUID    string
	TransAddr string
	TransType string
	// SvcID is needed for recovering target
	SvcID   string
	AddrFam string
}

type SubsystemAddHostRequest struct {
	NQN     string
	HostNQN string
}

type Subsystem = client.Subsystem

type TargetServiceIface interface {
	CreateTarget(req TargetCreateRequest) (resp Target, err error)
	DeleteTarget(nqn string) (err error)
	GetTargetStats() (result []SubsystemStatResp, err error)
	SubsysAddHost(req SubsystemAddHostRequest) (err error)
	// GetSubsystemByNQN
	GetSubsystemByNQN(nqn string) (subsys Subsystem, err error)
}

func (ss *SpdkService) CreateTarget(req TargetCreateRequest) (result Target, err error) {
	ss.cli, err = ss.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	klog.Infof("creating spdk target. req is %+v", req)

	var (
		nqn, bdevName, serialNumber, nsUUID, transAddr, transType string
		svcID, addrFam                                            string
		// parameter for nvmf_create_subsystem
		allowAnyHost = ss.Cfg.AllowAnyHost
	)
	nqn = req.TargetInfo.NQN
	bdevName = req.BdevName
	serialNumber = req.TargetInfo.SerialNumber
	nsUUID = req.TargetInfo.NSUUID
	transAddr = req.TargetInfo.TransAddr
	transType = req.TargetInfo.TransType
	svcID = req.TargetInfo.SvcID
	addrFam = req.TargetInfo.AddrFam

	result = req.TargetInfo
	result.NQN = nqn

	// nvmf_get_subsystems 检查 nqn 是否已经存在,
	var foundSubsystem, bdevInUse bool
	var modelNumber = nsUUID
	if modelNumber == "" {
		modelNumber = SpdkController
	}
	if transType == "" {
		transType = client.TransportTypeTCP
	}

	list, err := ss.cli.NVMFGetSubsystems()
	if err != nil {
		return
	}
	var subsystem client.Subsystem
	for _, subsys := range list {
		if nqn == subsys.NQN {
			subsystem = subsys
			foundSubsystem = true
			result.NQN = nqn
			for _, laddr := range subsys.ListenAddresses {
				// find svc id by transport type of request
				if laddr.TrType == transType {
					result.SvcID = laddr.TrSvcID
				}
			}
		} else {
			// 检查 bdev 是否被其他 subsystem 使用
			for _, ns := range subsys.Namespaces {
				if ns.BdevName == bdevName {
					bdevInUse = true
				}
			}
		}
	}
	if bdevInUse {
		err = fmt.Errorf("bdev %s is used by other subsystem", bdevName)
		return
	}

	if foundSubsystem {
		if len(subsystem.Namespaces) == 0 {
			// 添加 bdev
			_, err = ss.cli.NVMFSubsystemAddNS(client.NVMFSubsystemAddNSReq{
				NQN: nqn,
				Namespace: client.NamespaceForAddNS{
					BdevName: bdevName,
					UUID:     nsUUID,
				},
			})
			if err != nil {
				klog.Error(err)
				return
			}
		}

		if len(subsystem.ListenAddresses) == 0 {
			// 添加listener
			if svcID == "" {
				svcIDInt, errSvcID := ss.idAlloc.NextID()
				if errSvcID != nil {
					err = errSvcID
					return
				}
				svcID = strconv.Itoa(svcIDInt)
			}
			result.SvcID = svcID
			result.TransType = transType
			_, err = ss.cli.NVMFSubsystemAddListener(client.NVMFSubsystemAddListenerReq{
				NQN: nqn,
				ListenAddress: client.ListenAddress{
					TrType:  transType,
					AdrFam:  client.AddressFamily(addrFam),
					TrAddr:  transAddr,
					TrSvcID: svcID,
				},
			})
			if err != nil {
				klog.Error(err)
				return
			}

		}
	}

	if !foundSubsystem {
		_, err = ss.cli.NVMFCreateSubsystem(client.NVMFCreateSubsystemReq{
			NQN:          nqn,
			AllowAnyHost: allowAnyHost,
			SerialNumber: serialNumber,
			ModelNumber:  modelNumber,
		})
		if err != nil {
			klog.Error(err)
			return
		}
		result.NQN = nqn

		// 添加 bdev
		_, err = ss.cli.NVMFSubsystemAddNS(client.NVMFSubsystemAddNSReq{
			NQN: nqn,
			Namespace: client.NamespaceForAddNS{
				BdevName: bdevName,
				UUID:     nsUUID,
			},
		})
		if err != nil {
			klog.Error(err)
			return
		}

		// 添加listener
		if svcID == "" {
			svcIDInt, errSvcID := ss.idAlloc.NextID()
			if errSvcID != nil {
				err = errSvcID
				return
			}
			svcID = strconv.Itoa(svcIDInt)
		}
		result.SvcID = svcID
		result.TransType = transType
		listenerReq := client.NVMFSubsystemAddListenerReq{
			NQN: nqn,
			ListenAddress: client.ListenAddress{
				TrType:  transType,
				AdrFam:  client.AddressFamily(addrFam),
				TrAddr:  transAddr,
				TrSvcID: result.SvcID,
			},
		}
		klog.Infof("Calling NVMFSubsystemAddListener req=%+v", listenerReq)
		_, err = ss.cli.NVMFSubsystemAddListener(listenerReq)
		if err != nil {
			klog.Error(err)
			return
		}

	}

	err = ss.idAlloc.SyncFromTruth()
	if err != nil {
		klog.Error(err)
	} else {
		klog.Infof("successfully created spdk target %s", result.NQN)
	}

	return
}

func (ss *SpdkService) DeleteTarget(nqn string) (err error) {
	ss.cli, err = ss.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	// check if susbsystem exists
	var foundSubsystem, result bool
	list, err := ss.cli.NVMFGetSubsystems()
	if err != nil {
		return
	}
	for _, item := range list {
		if nqn == item.NQN {
			foundSubsystem = true
		}
	}

	if !foundSubsystem {
		klog.Infof("nqn %s not exists, consider deleting successfully", nqn)
		result = true
		return
	} else {
		defer ss.idAlloc.SyncFromTruth()
		result, err = ss.cli.NVMFDeleteSubsystem(client.NVMFDeleteSubsystemReq{
			NQN: nqn,
		})
		klog.Infof("delete subsystem %s result %t err %+v", nqn, result, err)
	}

	return
}

func (ss *SpdkService) GetTargetStats() (result []SubsystemStatResp, err error) {
	var (
		stats    client.SubsystemStat
		statsMap = make(map[string]SubsystemStatResp)
	)

	ss.cli, err = ss.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	stats, err = ss.cli.NVMFGetStats()
	if err != nil {
		klog.Error("get subsystem stat failed", err)
		return
	}

	tickRate := stats.TickRate
	for _, group := range stats.PollGroups {
		for _, subsys := range group.Subsystems {
			subsystemStat := statsMap[subsys.Ios.SubsysName]
			subsystemStat.SubsysName = subsys.Ios.SubsysName
			subsystemStat.BytesRead += subsys.Ios.BytesRead
			subsystemStat.NumReadOps += subsys.Ios.NumReadOps
			subsystemStat.BytesWrite += subsys.Ios.BytesWritten
			subsystemStat.NumWriteOps += subsys.Ios.NumWriteOps
			subsystemStat.ReadLatencyTime += subsys.Ios.ReadLatencyTicks * 1000000 / tickRate
			subsystemStat.WriteLatencyTime += subsys.Ios.WriteLatencyTicks * 1000000 / tickRate
			subsystemStat.TimeInQueue = subsys.Ios.TimeInQueue * 1000000 / tickRate
			statsMap[subsys.Ios.SubsysName] = subsystemStat
			// klog.Info("subsystem: ", subsys.Ios, " total: ", statsMap[subsys.Ios.SubsysName])
		}
	}
	for _, resp := range statsMap {
		result = append(result, resp)
	}
	klog.Info("resp: ", result)
	return
}

func (ss *SpdkService) SubsysAddHost(req SubsystemAddHostRequest) (err error) {
	ss.cli, err = ss.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	var result bool
	result, err = ss.cli.NVMFSubsystemAddHost(client.NVMFSubsystemAddHostReq{
		NQN:     req.NQN,
		HostNQN: req.HostNQN,
	})
	if err != nil {
		klog.Error("subsystem addhost failed", err)
		return
	}

	if !result {
		err = fmt.Errorf("subsystem addhost failed, %t %w", result, err)
	}

	return
}

func (ss *SpdkService) GetSubsystemByNQN(nqn string) (subsys Subsystem, err error) {
	ss.cli, err = ss.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	var (
		list  []client.Subsystem
		found bool
	)
	list, err = ss.cli.NVMFGetSubsystems()
	if err != nil {
		klog.Error("get subsystem failed", err)
		return
	}

	for _, item := range list {
		if nqn == item.NQN {
			subsys = item
			found = true
		}
	}

	if !found {
		err = fmt.Errorf("not found subsystem %s", nqn)
	}

	return
}
