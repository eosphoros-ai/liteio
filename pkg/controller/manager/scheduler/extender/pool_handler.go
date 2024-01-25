package extender

import (
	"context"
	"fmt"
	"strings"

	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"lite.io/liteio/pkg/controller/kubeutil"
	"lite.io/liteio/pkg/controller/manager/state"
	"lite.io/liteio/pkg/generated/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	rt "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// for StoragePool
func (se *SchedulerExtender) onStoragePoolAdd(obj interface{}) {
	sp, ok := obj.(*v1.StoragePool)
	if !ok {
		klog.Errorf("cannot convert to StoragePool: %v", obj)
		return
	}

	klog.Infof("add StoragePool %s", sp.Name)
	se.State.SetStoragePool(sp)

	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Error(err)
		return
	}
	if ns, name, err := cache.SplitMetaNamespaceKey(key); err == nil {
		se.poolQueue.Add(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: ns,
				Name:      name,
			},
		})
	}

}

func (se *SchedulerExtender) onStoragePoolUpdate(old, new interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(new)
	if err != nil {
		klog.Error(err)
		return
	}
	klog.Infof("update StoragePool %s", key)
	if ns, name, err := cache.SplitMetaNamespaceKey(key); err == nil {
		se.poolQueue.Add(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: ns,
				Name:      name,
			},
		})
	}
}

func (se *SchedulerExtender) onStoragePoolDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Error(err)
		return
	}
	klog.Infof("try to delete StoragePool %s from State", key)
	if ns, name, err := cache.SplitMetaNamespaceKey(key); err == nil {
		se.poolQueue.Add(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: ns,
				Name:      name,
			},
		})
	}
}

type StoragePoolReconciler struct {
	AntstoreCli versioned.Interface
	State       state.StateIface
	NodeUpdater kubeutil.NodeUpdaterIface
	PoolUtil    kubeutil.StoragePoolUpdater
}

func (r *StoragePoolReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var (
		ns    = req.Namespace
		name  = req.Name
		sp    *v1.StoragePool
		log   = rt.Log.WithName("Scheduler").WithName("StoragePool").WithValues("name", req.NamespacedName)
		spCli = r.AntstoreCli.VolumeV1().StoragePools(ns)
		err   error
	)

	sp, err = spCli.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// remove SP from State
			r.State.RemoveStoragePool(sp.Name)
			log.Info("cannot find Pool in apiserver")
			return reconcile.Result{}, nil
		}
		log.Error(err, "fetch StoragePool failed")
		return reconcile.Result{}, err
	}

	if sp.Spec.NodeInfo.ID == "" {
		err = fmt.Errorf("StoragePool has no value of .Spec.NodeInfo.ID")
		log.Error(err, "invalid SotragePool")
		return reconcile.Result{}, err
	}

	// handle deletion
	if sp.DeletionTimestamp != nil {
		var node *state.Node
		node, err = r.State.GetNodeByNodeID(name)
		if err != nil {
			log.Error(err, "not found node in State")
		}

		if node != nil && len(node.Volumes) > 0 {
			var volNames []string
			for _, item := range node.Volumes {
				volNames = append(volNames, item.Namespace+"/"+item.Name)
			}
			if !strings.HasPrefix(sp.Status.Message, "Volumes left on node") {
				sp.Status.Message = "Volumes left on node: " + strings.Join(volNames, ", ")
				_, err = spCli.UpdateStatus(ctx, sp, metav1.UpdateOptions{})
				if err != nil {
					log.Error(err, "updating StoragePool status failed")
				}
			}
			return reconcile.Result{}, err
		}

		err = r.State.RemoveStoragePool(sp.Name)
		if err != nil {
			log.Error(err, "RemoveStoragePool failed")
		}

		if len(sp.Finalizers) > 0 {
			log.Info("remove finalizers of StoragePool")
			// remove Finalizers
			sp.Finalizers = []string{}
			_, err = spCli.Update(ctx, sp, metav1.UpdateOptions{})
			if err != nil {
				log.Error(err, "removing finalizer failed")
			}
		}
	}

	log.Info("update StragePool to State")
	r.State.SetStoragePool(sp)

	// handle event to sync local storage of Node resource
	// if val, has := sp.Labels[v1.PoolEventSyncNodeLocalStorageKey]; has {
	// 	// enable reporing local-storage
	// 	/*
	// 		if val == "true" {
	// 			var node *state.Node
	// 			node, err = r.State.GetNodeByNodeID(sp.Name)
	// 			if err != nil {
	// 				log.Error(err, "GetNodeByNodeID failed", "sn", sp.Name)
	// 			}
	// 			// update Pool labels in memory
	// 			node.Pool.Labels = sp.Labels
	// 			localBytes := filter.CalculateLocalStorageCapacity()
	// 			log.Info("report local-storage to node", "node", sp.Name, "local-storage", localBytes)

	// 			// update node/status and pool's local-storage label; remove event key
	// 			_, err = r.NodeUpdater.ReportLocalDiskResource(sp.Name, localBytes)
	// 			if err != nil {
	// 				log.Error(err, "ReportLocalDiskResource failed", "sn", sp.Name)
	// 				return reconcile.Result{}, err
	// 			}

	// 			err = r.PoolUtil.SavePoolLocalStorageMark(sp, localBytes)
	// 			if err != nil {
	// 				log.Error(err, "SavePoolLocalStorage failed", "sn", sp.Name)
	// 				return reconcile.Result{}, err
	// 			}

	// 			return reconcile.Result{}, err
	// 		}
	// 	*/

	// 	// disable reporting local-storage
	// 	if val == "false" {
	// 		// remove node/status and pool's local-storage label; remove event key
	// 		_, err = r.NodeUpdater.ReportLocalDiskResource(sp.Name, 0)
	// 		if err != nil {
	// 			log.Error(err, "ReportLocalDiskResource failed", "sn", sp.Name)
	// 			return reconcile.Result{}, err
	// 		}
	// 		err = r.PoolUtil.RemovePoolLocalStorageMark(sp)
	// 		if err != nil {
	// 			log.Error(err, "SavePoolLocalStorage failed", "sn", sp.Name)
	// 			return reconcile.Result{}, err
	// 		}
	// 		return reconcile.Result{}, err
	// 	}
	// }

	return reconcile.Result{}, err
}
