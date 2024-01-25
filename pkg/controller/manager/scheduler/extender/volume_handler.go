package extender

import (
	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"lite.io/liteio/pkg/controller/manager/state"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// for AntstorVolume
func (se *SchedulerExtender) onAntstorVolumeAdd(obj interface{}) {
	var err error
	var vol, ok = obj.(*v1.AntstorVolume)
	if !ok {
		klog.Errorf("cannot convert to StoragePool: %v", obj)
		return
	}

	klog.Info("add AntstorVolume ", vol.Name, vol.ResourceVersion)

	if vol.Spec.TargetNodeId != "" && vol.DeletionTimestamp == nil {
		// check if StragePool exist
		var errGetPool error
		_, errGetPool = se.State.GetStoragePoolByNodeID(vol.Spec.TargetNodeId)
		if errGetPool != nil && state.IsNotFoundNodeError(errGetPool) {
			// create new Node for State
			klog.Infof("not found node %s, create a new node", vol.Spec.TargetNodeId)
			se.State.SetStoragePool(&v1.StoragePool{
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

		err = se.State.BindAntstorVolume(vol.Spec.TargetNodeId, vol)
		if err != nil {
			klog.Error(err)
		}
	}
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Error(err)
		return
	}
	if ns, name, err := cache.SplitMetaNamespaceKey(key); err == nil {
		se.volumeQueue.Add(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: ns,
				Name:      name,
			},
		})
	}
}

func (se *SchedulerExtender) onAntstorVolumeUpdate(old, new interface{}) {
	var vol, ok = new.(*v1.AntstorVolume)
	if !ok {
		klog.Errorf("cannot convert to AntstorVolume: %v", new)
		return
	}
	klog.Infof("AntstorVolume %s updated RV(%s)", vol.Name, vol.ResourceVersion)

	key, err := cache.MetaNamespaceKeyFunc(new)
	if err != nil {
		klog.Error(err)
		return
	}
	if ns, name, err := cache.SplitMetaNamespaceKey(key); err == nil {
		se.volumeQueue.Add(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: ns,
				Name:      name,
			},
		})
	}
}

func (se *SchedulerExtender) onAntstorVolumeDelete(obj interface{}) {
	var vol, ok = obj.(*v1.AntstorVolume)
	if !ok {
		klog.Errorf("cannot convert to AntstorVolume: %v", obj)
		return
	}
	klog.Infof("AntstorVolume %s deleted RV(%s)", vol.Name, vol.ResourceVersion)

	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Error(err)
		return
	}
	if ns, name, err := cache.SplitMetaNamespaceKey(key); err == nil {
		se.volumeQueue.Add(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: ns,
				Name:      name,
			},
		})
	}
}
