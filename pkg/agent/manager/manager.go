package manager

import (
	"context"
	"errors"
	"os"
	"time"

	"lite.io/liteio/pkg/agent/config"
	"lite.io/liteio/pkg/agent/metric"
	"lite.io/liteio/pkg/agent/pool"
	agentsync "lite.io/liteio/pkg/agent/sync"
	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"lite.io/liteio/pkg/controller/kubeutil"
	"lite.io/liteio/pkg/generated/clientset/versioned"
	"lite.io/liteio/pkg/spdk/hostnqn"
	"lite.io/liteio/pkg/util/runnable"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	PrefixObNvmf = "obnvmf"

	LeaseNamespace          = "obnvmf"
	AntstorDefaultNamespace = "obnvmf"
)

type Option struct {
	NodeID string
	// heartbeat interval, duration string
	HeartbeatInterval time.Duration
	// lvs size in byte
	LvsSize int64
	// lvs aio bdev file path
	LvsAioFilePath string
	// if true, use malloc bdev as LVS base bdev
	LvsMallocBdev bool
	// metric server listen address
	MetricListenAddr string
	// file path of config
	ConfigPath string
	// interval of metrics polling
	MetricIntervalSec int
}

type StoragePoolManager struct {
	// config of manager
	Opt Option
	// config of agent
	cfg config.Config
	// PoolService manages volumes, snapshots and SPDK targets
	// including Create/Delete LVM Volume/Snapshots and Create/Delete Target Service
	PoolService pool.StoragePoolServiceIface
	// sp is the StoragePool built by PoolBuilder
	sp *v1.StoragePool
	// kubeCli is used to read Node, Lease and other resources from APIServer
	kubeCli kubernetes.Interface
	// storeCli is used to read/write StoragePool, AntstorVolumes from APIServer
	storeCli versioned.Interface
	//
	runnableGroup *runnable.RunnableGroup
	// lister is used to list metric target components from AntstorVolume
	lister metric.MetricTargetListerIface
}

func NewStoragePoolManager(opt Option, kubeCli kubernetes.Interface, storeCli versioned.Interface) (spm *StoragePoolManager, err error) {
	spm = &StoragePoolManager{
		Opt:      opt,
		kubeCli:  kubeCli,
		storeCli: storeCli,
		lister:   metric.NewMetricInfoLister(),
	}

	// load config
	err = spm.setupConfig()
	if err != nil {
		klog.Error(err)
		return
	}

	// init hostnqn
	err = hostnqn.InitHostNQN(opt.NodeID)
	if err != nil {
		klog.Error(err)
		return
	}

	// init SPDK service and StorageVolume RPC service
	spm.PoolService, err = spm.newPoolService()
	if err != nil {
		klog.Error(err)
		return
	}

	spm.sp = spm.PoolService.GetStoragePool()
	// set node id
	spm.sp.Spec.NodeInfo.ID = spm.Opt.NodeID
	spm.sp.Name = spm.Opt.NodeID
	spm.sp.Namespace = v1.DefaultNamespace

	return
}

func (spm *StoragePoolManager) setupConfig() (err error) {
	var (
		mode     v1.PoolMode
		nodeInfo v1.NodeInfo
	)

	mode, nodeInfo, err = pool.NewPoolDetector(spm.Opt.NodeID, spm.kubeCli, spm.cfg.NodeKeys).DetectMode()
	if err != nil {
		return err
	}
	klog.Infof("set PoolMode to be %s, nodeInfo %s", mode, nodeInfo)

	spm.cfg, err = config.LoadFile(spm.Opt.ConfigPath)
	if err != nil {
		klog.Error(err)
		if !errors.Is(err, os.ErrNotExist) {
			return
		}
		klog.Infof("not found agent config from File %s, use default Storage config", spm.Opt.ConfigPath)
		spm.cfg.Storage = config.DefaultLVM
		if mode == v1.PoolModeSpdkLVStore {
			spm.cfg.Storage = config.DefaultLVS
		}
		klog.Infof("PoolMode %s, use default config %+v", mode, spm.cfg.Storage)
	}
	config.SetDefaults(&spm.cfg)
	spm.cfg.NodeInfo = nodeInfo

	return nil
}

func (spm *StoragePoolManager) newPoolService() (ps pool.StoragePoolServiceIface, err error) {
	// if LvsMallocBdev is true, pooling type should be LVS
	if spm.Opt.LvsMallocBdev {
		spm.cfg.Storage = config.DefaultLVS
		spm.cfg.Storage.Bdev = &config.SpdkBdev{
			Type: config.MemBdevType,
			Name: config.DefaultMallocBdevName,
			Size: uint64(spm.Opt.LvsSize),
		}
	}
	// if LvsAioFilePath has value, pooling type should be LVS
	if spm.Opt.LvsAioFilePath != "" {
		spm.cfg.Storage = config.DefaultLVS
		spm.cfg.Storage.Bdev = &config.SpdkBdev{
			Type:             config.AioBdevType,
			Name:             config.DefaultAioBdevName,
			Size:             uint64(spm.Opt.LvsSize),
			FilePath:         spm.Opt.LvsAioFilePath,
			CreateIfNotExist: true,
		}
	}
	klog.Infof("storage config is %+v", spm.cfg.Storage)

	ps, err = pool.NewPoolService(spm.cfg.Storage)

	// For lvstore pool mode, spdk service is necessary. The error should be returned and panic it.
	if spm.cfg.Storage.Pooling.Mode == v1.PoolModeSpdkLVStore {
		status := ps.SpdkWatcher().ReadStatus()
		if status.Error != nil {
			klog.Error(status.Error)
			return
		}
	}

	return
}

// Start run all services
func (spm *StoragePoolManager) Start() {
	var errCh = make(chan error)
	var ctx = context.Background()
	spm.runnableGroup = runnable.NewRunnableGroup(errCh)
	spm.runnableGroup.AddDefault(agentsync.NewMigrationReconciler(spm.Opt.NodeID, spm.storeCli, spm.PoolService.SpdkService()))
	spm.runnableGroup.AddDefault(agentsync.NewSnapshotSyncer(spm.storeCli, spm.PoolService))
	spm.runnableGroup.AddDefault(agentsync.NewVolumeSyncer(spm.storeCli, spm.PoolService, spm.lister))
	spm.runnableGroup.AddDefault(agentsync.NewDataControlReconciler(spm.Opt.NodeID, spm.storeCli))

	spm.runnableGroup.AddDefault(&HeartbeatService{
		Interval: spm.Opt.HeartbeatInterval,
		nodeID:   spm.Opt.NodeID,
		kubeCli:  spm.kubeCli,
		storeCli: spm.storeCli,
	})
	spm.runnableGroup.AddDefault(agentsync.NewPoolSyncer(spm.PoolService,
		spm.storeCli,
		kubeutil.NewKubeNodeInfoGetter(spm.kubeCli),
		spm.cfg))

	// init exporter collector
	if spm.Opt.MetricListenAddr != "" {
		spm.runnableGroup.AddDefault(metric.NewCollector(10*time.Second, spm.lister, spm.PoolService.SpdkService()))
	}

	spm.runnableGroup.Start(ctx)

	select {
	case <-ctx.Done():
		return
	case err := <-errCh:
		klog.Error(err)
		return
	}
}

// close manager, OfflineNodeStorage
func (spm *StoragePoolManager) Close() (err error) {
	klog.Info("stop spdk watcher")
	spm.PoolService.SpdkWatcher().Stop()
	klog.Info("stop runnable group")
	spm.runnableGroup.StopAndWait(context.Background())

	err = spm.garbageCollect()
	if err != nil {
		klog.Error(err)
	}

	return
}

// TODO:
func (spm *StoragePoolManager) garbageCollect() (err error) {
	return
}

/*
func (spm *StoragePoolManager) findStorageVolumesByNodeID() (vols []*v1.AntstorVolume, err error) {
	vols = make([]*v1.AntstorVolume, 0)

	volCli := spm.storeCli.VolumeV1().AntstorVolumes(AntstorDefaultNamespace)
	labelSelector := fmt.Sprintf("%s=%s", v1.TargetNodeIdLabelKey, spm.Opt.NodeID)
	list, err := volCli.List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		klog.Error(err)
		return
	}

	// WARNING: do not use "for range" to iterate the list.Items, when it will save pointer of each item.
	if len(list.Items) > 0 {
		for i := 0; i < len(list.Items); i++ {
			vols = append(vols, &list.Items[i])
		}
	}

	klog.Infof("Found %d vols in metadata", len(vols))
	for _, item := range vols {
		klog.Infof("In metadata: vol name=%s, id=%s", item.Name, item.Spec.Uuid)
	}

	return
}
*/
