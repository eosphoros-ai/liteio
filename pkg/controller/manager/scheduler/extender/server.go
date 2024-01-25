package extender

import (
	"context"
	"time"

	"lite.io/liteio/pkg/controller/kubeutil"
	"lite.io/liteio/pkg/controller/manager/config"
	sched "lite.io/liteio/pkg/controller/manager/scheduler"
	"lite.io/liteio/pkg/controller/manager/state"
	"lite.io/liteio/pkg/generated/clientset/versioned"
	antstorinformers "lite.io/liteio/pkg/generated/informers/externalversions"
	"k8s.io/client-go/kubernetes"
	clientgocache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type SchedulerExtender struct {
	// EnableExtenderMode, if true, enables HTTP service for predicate, priority and bind
	EnableExtenderMode bool
	// State presents the whole state of Pools and Volumes
	State state.StateIface
	// Scheduler
	Scheduler sched.SchedulerIface
	// AutoAdjustHelper
	// AutoAdjustHelper plugin.AdjustLocalStorageHelperIface

	NodeUpdater kubeutil.NodeUpdaterIface
	PoolUtil    kubeutil.StoragePoolUpdater

	hbMgr *HeartbeatManager

	antstoreCli      versioned.Interface
	antstorInformers antstorinformers.SharedInformerFactory

	// synced func of all informers
	informersSynced []clientgocache.InformerSynced
	//
	volumeQueue workqueue.RateLimitingInterface
	poolQueue   workqueue.RateLimitingInterface
}

func NewSchedulerExtender(
	antstoreCli versioned.Interface,
	kubeClient kubernetes.Interface,
	state state.StateIface,
	// autoAdjustHelper plugin.AdjustLocalStorageHelperIface,
	nodeUpdater kubeutil.NodeUpdaterIface,
	poolUtil kubeutil.StoragePoolUpdater,
) (se *SchedulerExtender) {

	antstorInformer := antstorinformers.NewSharedInformerFactory(antstoreCli, 30*time.Minute)
	// item order in []InformerSynced is the loading order of resources
	informersSynced := make([]clientgocache.InformerSynced, 0)

	spInformer := antstorInformer.Volume().V1().StoragePools().Informer()
	informersSynced = append(informersSynced, spInformer.HasSynced)

	avInformer := antstorInformer.Volume().V1().AntstorVolumes().Informer()
	informersSynced = append(informersSynced, avInformer.HasSynced)

	// create the workqueue
	volumeQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	poolQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	hbMgr := NewHeartbeatManager(state, poolUtil, kubeClient)

	se = &SchedulerExtender{
		State:            state,
		Scheduler:        sched.NewScheduler(config.Config{}),
		antstoreCli:      antstoreCli,
		antstorInformers: antstorInformer,
		informersSynced:  informersSynced,
		volumeQueue:      volumeQueue,
		poolQueue:        poolQueue,
		// AutoAdjustHelper: autoAdjustHelper,
		NodeUpdater: nodeUpdater,
		PoolUtil:    poolUtil,
		hbMgr:       hbMgr,
	}

	// bind event handler
	spInformer.AddEventHandler(clientgocache.ResourceEventHandlerFuncs{
		AddFunc:    se.onStoragePoolAdd,
		UpdateFunc: se.onStoragePoolUpdate,
		DeleteFunc: se.onStoragePoolDelete,
	})

	avInformer.AddEventHandler(clientgocache.ResourceEventHandlerFuncs{
		AddFunc:    se.onAntstorVolumeAdd,
		UpdateFunc: se.onAntstorVolumeUpdate,
		DeleteFunc: se.onAntstorVolumeDelete,
	})

	return
}

func (se *SchedulerExtender) Start(ctx context.Context) (err error) {
	// Let the workers stop when we are done
	defer se.volumeQueue.ShutDown()
	defer se.poolQueue.ShutDown()

	se.antstorInformers.Start(ctx.Done())

	stopChan := ctx.Done()
	// wait all resources synced
	if ok := clientgocache.WaitForNamedCacheSync("NvmfScheduler", stopChan, se.informersSynced...); !ok {
		klog.Fatal("failed to wait for all informer caches to be synced")
	}

	// run pool worker
	poolReconciler := &StoragePoolReconciler{
		AntstoreCli: se.antstoreCli,
		State:       se.State,
		PoolUtil:    se.PoolUtil,
		NodeUpdater: se.NodeUpdater,
	}
	volReconciler := &AntstorVolumeReconciler{
		AntstoreCli: se.antstoreCli,
		State:       se.State,
		Scheduler:   se.Scheduler,
		// AutoAdjustHelper: se.AutoAdjustHelper,
	}

	go kubeutil.NewSimpleController("Sched-Pool", se.poolQueue, poolReconciler).Start(ctx)
	go kubeutil.NewSimpleController("Sched-Volume", se.volumeQueue, volReconciler).Start(ctx)
	go se.hbMgr.Start(ctx)

	<-stopChan
	klog.Info("quit scheduler")
	return
}
