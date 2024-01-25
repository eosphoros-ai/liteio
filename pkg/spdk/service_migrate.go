package spdk

import (
	spdkrpc "lite.io/liteio/pkg/spdk/jsonrpc/client"
	"k8s.io/klog/v2"
)

type MigrateServiceIface interface {
	EnsureMigrationDestBdev(req AttachDestBdevRequest) (err error)
	DetachMigrationDestBdev(bdevName string) (err error)

	GetMigrationTask(req GetMigrationTaskRequest) (tasks []MigrateTask, err error)
	StartMigrationTask(req MigrateStartRequest) (err error)
	CleanMigrationTask(req MigrateStartRequest) (err error)

	SetMigrationConfig(req MigrateConfigRequest) (err error)
}

type MigrateTask spdkrpc.MigrateTask
type MigrateStartRequest spdkrpc.BdevMigrateStartRequest
type MigrateConfigRequest spdkrpc.BdevMigrateSetConfigRequest

type SpdkTargetInfo struct {
	NQN string
	// IPv4
	AddrFam string
	IPAddr  string
	// tcp
	TransType string
	// target service port
	SvcID string
}

type AttachDestBdevRequest struct {
	ControllerName string
	Target         SpdkTargetInfo
}

type GetMigrationTaskRequest spdkrpc.BdevMigrateQueryRequest

func (ss *SpdkService) EnsureMigrationDestBdev(req AttachDestBdevRequest) (err error) {
	ss.cli, err = ss.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	// NOTICE: after attaching remote subsystem, the bdev name is req.BdevName+"n1"
	var (
		bdevName      string = req.ControllerName + "n1"
		bdevs         []spdkrpc.Bdev
		attachedBdevs []string
	)

	bdevs, err = ss.cli.BdevGetBdevs(spdkrpc.BdevGetBdevsReq{
		BdevName: bdevName,
	})
	if err != nil {
		// only return error when error msg is "No such device"
		if !IsNotFoundDeviceError(err) {
			klog.Error(err)
			return
		}
	}

	if len(bdevs) > 0 {
		klog.Infof("EnsureMigrationDestBdev, bdev %s already exists", bdevName)
		return
	}

	// do create
	attachedBdevs, err = ss.cli.AttachController(spdkrpc.AttachControllerRequest{
		Name:    req.ControllerName,
		TrType:  req.Target.TransType,
		TrAddr:  req.Target.IPAddr,
		AdrFam:  req.Target.AddrFam,
		SubNQN:  req.Target.NQN,
		TrSvcId: req.Target.SvcID,
	})
	if err != nil {
		klog.Error(err)
		return
	}
	klog.Info("AttachController ", attachedBdevs)

	return
}

func (ss *SpdkService) DetachMigrationDestBdev(controllerName string) (err error) {
	ss.cli, err = ss.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	// get bdev
	var (
		bdevs    []spdkrpc.Bdev
		bdevName = controllerName + "n1"
	)

	bdevs, err = ss.cli.BdevGetBdevs(spdkrpc.BdevGetBdevsReq{
		BdevName: bdevName,
	})
	if err != nil {
		if !IsNotFoundDeviceError(err) {
			klog.Error(err)
			return
		}
	}

	if len(bdevs) == 0 {
		klog.Infof("DetachMigrationDestBdev, cannot find %s, consider it removed", bdevName)
		return nil
	}

	// do detach
	err = ss.cli.DetachController(spdkrpc.DetachControllerRequest{
		Name: controllerName,
	})
	if err != nil {
		klog.Error(err)
		return
	}

	return
}

func (ss *SpdkService) GetMigrationTask(req GetMigrationTaskRequest) (tasks []MigrateTask, err error) {
	ss.cli, err = ss.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	var (
		list []spdkrpc.MigrateTask
	)

	list, err = ss.cli.BdevMigrateQuery(spdkrpc.BdevMigrateQueryRequest(req))
	if err != nil {
		return nil, err
	}

	for _, item := range list {
		if item.SrcBdev == req.SrcBdev {
			tasks = append(tasks, MigrateTask(item))
		}
	}

	return
}

func (ss *SpdkService) StartMigrationTask(req MigrateStartRequest) (err error) {
	ss.cli, err = ss.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	err = ss.cli.BdevMigrateStart(spdkrpc.BdevMigrateStartRequest(req))
	return
}

func (ss *SpdkService) CleanMigrationTask(req MigrateStartRequest) (err error) {
	ss.cli, err = ss.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	// check if task exist
	var list []spdkrpc.MigrateTask
	var foundTask bool
	list, err = ss.cli.BdevMigrateQuery(spdkrpc.BdevMigrateQueryRequest{
		SrcBdev: req.SrcBdev,
	})
	for _, item := range list {
		foundTask = item.SrcBdev == req.SrcBdev && item.DstBdev == req.DstBdev
	}

	if foundTask {
		err = ss.cli.BdevMigrateCleanupTask(spdkrpc.BdevMigrateStartRequest(req))
		return
	}

	klog.Infof("not found MigrationTask by req %+v, consider it successfuly cleaned", req)

	return
}

func (ss *SpdkService) SetMigrationConfig(req MigrateConfigRequest) (err error) {
	ss.cli, err = ss.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	err = ss.cli.BdevMigrateSetConfig(spdkrpc.BdevMigrateSetConfigRequest(req))
	return
}
