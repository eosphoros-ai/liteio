package handler

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/config"
)

var (
	_ cache.ResourceEventHandler = &NodeEventHandler{}
	_ handler.EventHandler       = &NodeEventHandler{}
)

type NodeEventHandler struct {
	Cfg config.Config
}

// Create implements EventHandler.
func (e *NodeEventHandler) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	if !validateNode(&e.Cfg, evt.Object) {
		return
	}

	// Node name is same with StoragePool name
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: v1.DefaultNamespace,
		Name:      evt.Object.GetName(),
	}})
}

// Update implements EventHandler.
func (e *NodeEventHandler) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	if !validateNode(&e.Cfg, evt.ObjectNew) {
		return
	}

	// Node name is same with StoragePool name
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: v1.DefaultNamespace,
		Name:      evt.ObjectNew.GetName(),
	}})
}

// Delete implements EventHandler.
func (e *NodeEventHandler) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	if !validateNode(&e.Cfg, evt.Object) {
		return
	}

	// Node name is same with StoragePool name
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: v1.DefaultNamespace,
		Name:      evt.Object.GetName(),
	}})
}

// Generic implements EventHandler.
func (e *NodeEventHandler) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	if !validateNode(&e.Cfg, evt.Object) {
		return
	}

	// Node name is same with StoragePool name
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: v1.DefaultNamespace,
		Name:      evt.Object.GetName(),
	}})
}

func (e *NodeEventHandler) FilterObject(obj interface{}) bool {
	if metaObj, ok := obj.(client.Object); ok {
		return validateNode(&e.Cfg, metaObj)
	}

	return false
}

func (e *NodeEventHandler) OnAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Error(err)
		return
	}
	klog.Info("Node is added: ", key)
}

func (e *NodeEventHandler) OnUpdate(oldObj, newObj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(newObj)
	if err != nil {
		klog.Error(err)
		return
	}
	klog.Info("Node is updated: ", key)
}

func (e *NodeEventHandler) OnDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Error(err)
		return
	}
	klog.Infof("delete Node %s", key)
}

func validateNode(cfg *config.Config, obj client.Object) bool {
	if obj == nil {
		klog.Error(nil, "NodeEvent received with nil object")
		return false
	}

	// no config of NodeCacheSelector means appcet all nodes events
	if len(cfg.Scheduler.NodeCacheSelector) == 0 {
		return true
	}

	selector := labels.SelectorFromSet(labels.Set(cfg.Scheduler.NodeCacheSelector))
	if selector.Matches(labels.Set(obj.GetLabels())) {
		klog.Info("matched node to cache: ", obj.GetName())
		return true
	}

	return false
}
