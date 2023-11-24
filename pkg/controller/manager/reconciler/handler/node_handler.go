package handler

import (
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

var (
	_ cache.ResourceEventHandler = &NodeEventHandler{}
)

type NodeEventHandler struct {
	Cfg config.Config
}

func (e *NodeEventHandler) FilterObject(obj interface{}) bool {
	/*
		key, err := cache.MetaNamespaceKeyFunc(obj)
		if err != nil {
			return false
		}

		_, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return false
		}
	*/

	if len(e.Cfg.Scheduler.NodeCacheSelector) == 0 {
		return true
	}

	if node, ok := obj.(*corev1.Node); ok {
		selector := labels.SelectorFromSet(labels.Set(e.Cfg.Scheduler.NodeCacheSelector))
		if selector.Matches(labels.Set(node.Labels)) {
			klog.Info("matched node to cache: ", node.Name)
			return true
		}
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
