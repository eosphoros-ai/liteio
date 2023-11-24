package pool

import (
	"time"

	"code.alipay.com/dbplatform/node-disk-controller/pkg/agent/config"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/agent/pool/engine"
	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/spdk"
	"k8s.io/klog/v2"
)

var (
	_ StoragePoolServiceIface = &PoolService{}
)

type PoolService struct {
	mode v1.PoolMode
	pool *v1.StoragePool
	bld  PoolBuilderIface

	// poolEng is a PoolEngine, responsable for managing logical volumes
	poolEng engine.PoolEngineIface
	// spdkWatcher watches spdk tgt service
	spdkWatcher *SpdkWatcher
	// for spdk lvstore pooling
	spdk spdk.SpdkServiceIface
	// expose public access to bdev
	access AccessIface
}

func NewPoolService(cfg config.StorageStack) (ps *PoolService, err error) {
	var (
		mode     v1.PoolMode = cfg.Pooling.Mode
		spdkSvc  spdk.SpdkServiceIface
		poolInfo engine.StaticInfo
		poolEng  engine.PoolEngineIface
		builder  = NewPoolBuilder()
	)

	// create SpdkService
	spdkSvc, err = spdk.NewSpdkService(spdk.SpdkServiceConfig{
		CliGenFn:     spdk.NewWithDefaultSock,
		AllowAnyHost: false,
	})
	if err != nil {
		// For LVM pool mode, spdk service is used to create Target subsystem. Without spdk service, local disk could still work.
		// Therefore, we ignore that the spdk service is empty.
		if err == spdk.ErrorSpdkConnectionLost {
			klog.Errorf("spdk controller init failed, err %+v. spdk recovery service will try to reconnect socket file", err)
			err = nil
		} else {
			klog.Error(err)
			return
		}
	}

	switch mode {
	case v1.PoolModeKernelLVM:
		poolEng = engine.NewLvmPoolEngine(cfg.Pooling.Name)
	case v1.PoolModeSpdkLVStore:
		poolEng = engine.NewSpdkLvsPoolEngine(cfg.Pooling.Name, spdkSvc)
	}

	poolInfo, err = builder.WithMode(mode).
		WithConfig(cfg).
		WithPoolEngine(poolEng).
		WithSpdkService(spdkSvc).
		Build()
	if err != nil {
		klog.Error(err)
		return
	}

	// create PoolService
	ps = &PoolService{
		spdk:        spdkSvc,
		mode:        mode,
		pool:        &v1.StoragePool{},
		poolEng:     poolEng,
		bld:         builder,
		spdkWatcher: NewSpdkWatcher(time.Minute, spdkSvc),
		access:      NewSpdkAccess(spdkSvc),
	}

	if poolInfo.LVM != nil {
		ps.pool.Spec.KernelLVM = *poolInfo.LVM
	}
	if poolInfo.LVS != nil {
		ps.pool.Spec.SpdkLVStore = *poolInfo.LVS
		// TODO add bdf for nvme connecting
		// ps.pool.Annotations[""] = "bdf"
	}

	// start watching Spdk tgt status
	go ps.spdkWatcher.Watch()

	return
}

func (ps *PoolService) GetStoragePool() *v1.StoragePool {
	return ps.pool
}

func (ps *PoolService) Mode() (mode v1.PoolMode) {
	return ps.mode
}

func (ps *PoolService) SpdkService() spdk.SpdkServiceIface {
	return ps.spdk
}

func (ps *PoolService) SpdkWatcher() *SpdkWatcher {
	return ps.spdkWatcher
}

func (ps *PoolService) Access() AccessIface {
	return ps.access
}

func (ps *PoolService) PoolEngine() engine.PoolEngineIface {
	return ps.poolEng
}
