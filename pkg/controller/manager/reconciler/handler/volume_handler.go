package handler

import (
	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	_ handler.EventHandler = &VolumeEventHandler{}
)

// VolumeEventHandler implements EventHandler
type VolumeEventHandler struct {
	State state.StateIface
}

// Create implements EventHandler.
func (e *VolumeEventHandler) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	if evt.Object == nil {
		klog.Error(nil, "CreateEvent received with no metadata", "event", evt)
		return
	}

	var (
		ns, name, objKind string
		// object being watched
		obj = evt.Object
	)
	ns = obj.GetNamespace()
	name = obj.GetName()
	objKind = obj.GetObjectKind().GroupVersionKind().Kind
	// EqualFold ignores letter case; objKind maybe empty; strings.EqualFold(objKind, v1.AntstorVolumeKind) maybe false

	// Add AntstorVolume to State
	klog.Infof("volume create event: kind=%s %s/%s", objKind, ns, name)
	var vol, ok = obj.(*v1.AntstorVolume)
	if !ok {
		klog.Errorf("cannot convert to AntstorVolume %#v", obj)
		return
	}

	if vol.Spec.TargetNodeId != "" && vol.DeletionTimestamp == nil {
		// check if StragePool exist
		var err error
		_, err = e.State.GetStoragePoolByNodeID(vol.Spec.TargetNodeId)
		if err != nil && state.IsNotFoundNodeError(err) {
			// create empty StoragePool in State
			klog.Infof("not found node %s, create a new node", vol.Spec.TargetNodeId)
			e.State.SetStoragePool(&v1.StoragePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: vol.Spec.TargetNodeId,
				},
				Spec: v1.StoragePoolSpec{
					NodeInfo: v1.NodeInfo{
						ID: vol.Spec.TargetNodeId,
					},
				},
			})
		}
		klog.Infof("add AntstorVolume %s/%s to StoragePool %s", ns, name, vol.Spec.TargetNodeId)
		err = e.State.BindAntstorVolume(vol.Spec.TargetNodeId, vol)
		if err != nil {
			klog.Error(err)
		}
	}

	// hacking: add a prefix to namespace, indicating the object is an AntstorVolume
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}})
}

// Update implements EventHandler.
func (e *VolumeEventHandler) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	var (
		ns, name string
		// object being watched
		obj = evt.ObjectNew
	)
	name = obj.GetName()
	ns = obj.GetNamespace()
	klog.Infof("volume update event:  %s/%s", ns, name)

	switch {
	case evt.ObjectNew != nil:
		q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: ns,
			Name:      name,
		}})
	case evt.ObjectOld != nil:
		q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: evt.ObjectOld.GetNamespace(),
			Name:      evt.ObjectOld.GetName(),
		}})
	default:
		klog.Error(nil, "UpdateEvent received with no metadata", "event", evt)
	}
}

// Delete implements EventHandler.
func (e *VolumeEventHandler) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	if evt.Object == nil {
		klog.Error(nil, "DeleteEvent received with no metadata", "event", evt)
		return
	}
	var (
		ns, name string
		// object being watched
		obj = evt.Object
	)
	name = obj.GetName()
	ns = obj.GetNamespace()
	klog.Infof("volume delete event:  %s/%s", ns, name)

	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}})
}

// Generic implements EventHandler.
func (e *VolumeEventHandler) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	if evt.Object == nil {
		klog.Error(nil, "GenericEvent received with no metadata", "event", evt)
		return
	}
	var (
		ns, name string
		// object being watched
		obj = evt.Object
	)
	name = obj.GetName()
	ns = obj.GetNamespace()

	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}})
}
