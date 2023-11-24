package kubeutil

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type SimpleController struct {
	Name       string
	Queue      workqueue.RateLimitingInterface
	Reconciler reconcile.Reconciler
}

func NewSimpleController(name string, queue workqueue.RateLimitingInterface, reconciler reconcile.Reconciler) *SimpleController {
	sc := &SimpleController{
		Name:       name,
		Queue:      queue,
		Reconciler: reconciler,
	}

	return sc
}

// Start blocks until context is done
func (sc *SimpleController) Start(ctx context.Context) {
	// processNextWorkItem blocks at Queue.Get()
	// processNextWorkItem should run in a new goroutine, so Start could quit immediately after Context is done.
	go func() {
		klog.Infof("SimpleController %s start processing loop", sc.Name)
		for sc.processNextWorkItem(ctx) {
		}
		klog.Infof("SimpleController %s quit processing loop", sc.Name)
	}()
	<-ctx.Done()
	sc.Queue.ShutDown()

	klog.Infof("SimpleController %s quit", sc.Name)
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the reconcileHandler.
func (sc *SimpleController) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := sc.Queue.Get()
	if shutdown {
		// Stop working
		return false
	}

	// We call Done here so the workqueue knows we have finished
	// processing this item. We also must remember to call Forget if we
	// do not want this work item being re-queued. For example, we do
	// not call Forget if a transient error occurs, instead the item is
	// put back on the workqueue and attempted again after a back-off
	// period.
	defer sc.Queue.Done(obj)

	sc.reconcileHandler(ctx, obj)

	// if context is done, shutdown queue, then the for loop is done.
	// select {
	// case <-ctx.Done():
	// 	sc.Queue.ShutDown()
	// default:
	// }

	return true
}

func (sc *SimpleController) reconcileHandler(ctx context.Context, obj interface{}) {
	// Update metrics after processing each item
	// reconcileStartTS := time.Now()

	// Make sure that the object is a valid request.
	req, ok := obj.(reconcile.Request)
	if !ok {
		// As the item in the workqueue is actually invalid, we call
		// Forget here else we'd go into a loop of attempting to
		// process a work item that is invalid.
		sc.Queue.Forget(obj)
		klog.Errorf("Queue item was not a Request: %#v", obj)
		// Return true, don't take a break
		return
	}

	// RunInformersAndControllers the syncHandler, passing it the Namespace/Name string of the
	// resource to be synced.
	result, err := sc.Reconciler.Reconcile(ctx, req)
	switch {
	case err != nil:
		sc.Queue.AddRateLimited(req)
		klog.Error(err, "Reconciler error")
	case result.RequeueAfter > 0:
		// The result.RequeueAfter request will be lost, if it is returned
		// along with a non-nil error. But this is intended as
		// We need to drive to stable reconcile loops before queuing due
		// to result.RequestAfter
		sc.Queue.Forget(obj)
		sc.Queue.AddAfter(req, result.RequeueAfter)
	case result.Requeue:
		sc.Queue.AddRateLimited(req)
	default:
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		sc.Queue.Forget(obj)
	}
}

// a widly used EventHandler
func CommonResourceEventHandlerFuncs(queue workqueue.RateLimitingInterface) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err != nil {
				klog.Error(err)
				return
			}
			ns, name, err := cache.SplitMetaNamespaceKey(key)
			if err != nil {
				klog.Error(err)
				return
			}
			queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: ns,
			}})
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err != nil {
				klog.Error(err)
				return
			}
			ns, name, err := cache.SplitMetaNamespaceKey(key)
			if err != nil {
				klog.Error(err)
				return
			}
			queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: ns,
			}})
		},
		DeleteFunc: func(obj interface{}) {
			// IndexerInformer uses a delta queue, therefore for deletes we have to use this
			// key function.
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err != nil {
				klog.Error(err)
				return
			}
			ns, name, err := cache.SplitMetaNamespaceKey(key)
			if err != nil {
				klog.Error(err)
				return
			}
			queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: ns,
			}})
		},
	}
}
