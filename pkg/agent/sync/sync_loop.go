package sync

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type ResourceSyncerFunc func(name string) (err error)

type SyncLoop struct {
	Name string
	// watcher
	ListWatcher *cache.ListWatch
	WatchObject runtime.Object
	// sync func
	SyncFn ResourceSyncerFunc

	// queue component
	Indexer  cache.Indexer
	Queue    workqueue.RateLimitingInterface
	Informer cache.Controller
}

func NewSyncLoop(name string, watcher *cache.ListWatch, obj runtime.Object, syncFn ResourceSyncerFunc) *SyncLoop {
	return &SyncLoop{
		Name:        name,
		ListWatcher: watcher,
		WatchObject: obj,
		SyncFn:      syncFn,
	}
}

func (sl *SyncLoop) RunLoop(quitCh <-chan struct{}) {
	// create the workqueue
	sl.Queue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	// Bind the workqueue to a cache with the help of an informer. This way we make sure that
	// whenever the cache is updated, the pod key is added to the workqueue.
	// Note that when we finally process the item from the workqueue, we might see a newer version
	// of the Pod than the version which was responsible for triggering the update.
	sl.Indexer, sl.Informer = cache.NewIndexerInformer(sl.ListWatcher, sl.WatchObject, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				sl.Queue.Add(key)
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				sl.Queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			// IndexerInformer uses a delta queue, therefore for deletes we have to use this
			// key function.
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				sl.Queue.Add(key)
			}
		},
	}, cache.Indexers{})

	defer runtimeutil.HandleCrash()
	// Let the workers stop when we are done
	defer sl.Queue.ShutDown()
	klog.Info(sl.Name, " Starting Queue and Informer")

	go sl.Informer.Run(quitCh)

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(quitCh, sl.Informer.HasSynced) {
		runtimeutil.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

	go wait.Until(sl.runSyncer, time.Second, quitCh)

	<-quitCh
	klog.Info(sl.Name, " Quit SyncLoop")
}

func (sl *SyncLoop) runSyncer() {
	for sl.processNextItem() {
	}
}

func (sl *SyncLoop) processNextItem() bool {
	// Wait until there is a new item in the working queue
	key, quit := sl.Queue.Get()
	if quit {
		klog.Info(sl.Name, " Quit processNextItem")
		return false
	}
	klog.Info("process key ", key)

	// Tell the queue that we are done with processing this key. This unblocks the key for other workers
	// This allows safe parallel processing because two pods with the same key are never processed in
	// parallel.
	defer sl.Queue.Done(key)

	if nsName, ok := key.(string); ok {
		// create or delete volume
		err := sl.SyncFn(nsName)
		if err == nil {
			// Forget about the #AddRateLimited history of the key on every successful synchronization.
			// This ensures that future processing of updates for this key is not delayed because of
			// an outdated error history.
			sl.Queue.Forget(key)
		} else {
			retryCnt := sl.Queue.NumRequeues(key)
			if retryCnt > 5 {
				klog.Errorf("key %s has retry too many times %d, delay it for 30s", key, retryCnt)
				sl.Queue.AddAfter(key, 30*time.Second)
			} else {
				// Re-enqueue the key rate limited. Based on the rate limiter on the
				// queue and the re-enqueue history, the key will be processed later again.
				sl.Queue.AddRateLimited(key)
			}
			// runtime.HandleError(err)
			klog.Error(key, err)
		}
	} else {
		klog.Errorf("key is not string, %#v", key)
		time.Sleep(time.Second)
	}

	return true
}
