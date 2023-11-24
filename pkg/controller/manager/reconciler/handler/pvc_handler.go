package handler

import (
	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

var (
	_ cache.ResourceEventHandler = &PVCEventHandler{}
	_ handler.EventHandler       = &PVCEventHandler{}
)

type PVCEventHandler struct {
	State state.StateIface
	// StorageClassCache is a sc lister
	StorageClassCache CachedStorageClassIface
}

func NewPVCEventHandler(state state.StateIface, kubeCli kubernetes.Interface) *PVCEventHandler {
	return &PVCEventHandler{
		State:             state,
		StorageClassCache: NewCachedStorageClass(kubeCli),
	}
}

// Create implements EventHandler.
func (e *PVCEventHandler) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	if evt.Object == nil {
		klog.Error(nil, "CreateEvent received with no metadata", "event", evt)
		return
	}

	// Add PVC to State
	e.OnAdd(evt.Object)

	// hacking: add a prefix to namespace, indicating the object is an PVC
	// q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
	// 	Namespace: pvcNsPrefix + ns,
	// 	Name:      name,
	// }})
}

// Update implements EventHandler.
func (e *PVCEventHandler) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	e.OnUpdate(evt.ObjectOld, evt.ObjectNew)
}

// Delete implements EventHandler.
func (e *PVCEventHandler) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	if evt.Object == nil {
		klog.Error(nil, "DeleteEvent received with no metadata", "event", evt)
		return
	}
	e.OnDelete(evt.Object)
}

// Generic implements EventHandler.
func (e *PVCEventHandler) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	if evt.Object == nil {
		klog.Error(nil, "GenericEvent received with no metadata", "event", evt)
		return
	}
	e.OnUpdate(evt.Object, evt.Object)
}

func (e *PVCEventHandler) isAntstorMustLocalPVCBound(pvc *corev1.PersistentVolumeClaim) (nodeName string, bound bool) {
	if pvc == nil || pvc.Spec.StorageClassName == nil {
		return
	}

	sc, err := e.StorageClassCache.GetByName(*pvc.Spec.StorageClassName)
	if err != nil {
		klog.Error(err)
		return
	}

	var hasAnn bool
	nodeName, hasAnn = pvc.Annotations[v1.K8SAnnoSelectedNode]
	bound = hasAnn && sc.Parameters[v1.StorageClassParamPositionAdvice] == string(v1.MustLocal) && sc.Provisioner == v1.StorageClassProvisioner
	return
}

func (e *PVCEventHandler) OnAdd(obj interface{}) {
	// Add PVC to State
	var pvc, ok = obj.(*corev1.PersistentVolumeClaim)
	if !ok {
		klog.Errorf("cannot convert to PVC %#v", obj)
		return
	}

	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Error(err)
		return
	}
	// ns, name, err := cache.SplitMetaNamespaceKey(key)

	if nodeName, bound := e.isAntstorMustLocalPVCBound(pvc); bound {
		// check if StragePool exist
		var (
			err  error
			node *state.Node
		)

		_, err = e.State.GetStoragePoolByNodeID(nodeName)
		if err != nil && state.IsNotFoundNodeError(err) {
			// create new Node for State
			klog.Infof("not found node %s, create a new node", nodeName)
			e.State.SetStoragePool(&v1.StoragePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
				},
				Spec: v1.StoragePoolSpec{
					NodeInfo: v1.NodeInfo{
						ID: nodeName,
					},
				},
			})
		}

		node, err = e.State.GetNodeByNodeID(nodeName)
		if err != nil {
			klog.Error(err)
		} else {
			klog.Infof("reserve PVC %s to pool %s", key, nodeName)
			resv := state.NewPvcReservation(pvc)
			node.Reserve(resv)
		}
	}
}

func (e *PVCEventHandler) OnUpdate(oldObj, newObj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(newObj)
	if err != nil {
		klog.Error(err)
		return
	}

	klog.Info("PVC is updated: ", key)
}

func (e *PVCEventHandler) OnDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Error(err)
		return
	}
	klog.Infof("delete Reservation of PVC %s from State", key)

	if pvc, ok := obj.(*corev1.PersistentVolumeClaim); ok {
		nodeName, _ := e.isAntstorMustLocalPVCBound(pvc)
		if nodeName != "" {
			node, err := e.State.GetNodeByNodeID(nodeName)
			if err != nil {
				klog.Error("failed to remove Reservation from node ", nodeName)
			} else {
				klog.Infof("Unreserve ID %s from node %s", key, nodeName)
				node.Unreserve(key)
			}
		}
	}
}
