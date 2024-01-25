package reconciler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	coorv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"lite.io/liteio/pkg/controller/kubeutil"
	"lite.io/liteio/pkg/controller/manager/config"
	"lite.io/liteio/pkg/controller/manager/reconciler/plugin"
	"lite.io/liteio/pkg/controller/manager/state"
	"lite.io/liteio/pkg/util/misc"
)

const (
	LeaseNamespace = "obnvmf"
)

var (
	// if last heartbeat is 3 min ago, consider node lost
	nodeExpireDuration = 3 * time.Minute
	// if last heartbeat is 24h ago, consider node offline
	nodeOfflineExpireDuration = 24 * time.Hour
)

type StoragePoolReconcileHandler struct {
	client.Client

	Cfg      config.Config
	State    state.StateIface
	PoolUtil kubeutil.StoragePoolUpdater
	KubeCli  kubernetes.Interface
}

func (r *StoragePoolReconcileHandler) ResourceName() string {
	return "StoragePool"
}

func (r *StoragePoolReconcileHandler) GetObject(req plugin.RequestContent) (obj runtime.Object, err error) {
	var pool v1.StoragePool
	err = r.Client.Get(req.Ctx, req.Request.NamespacedName, &pool)
	return &pool, err
}

func (r *StoragePoolReconcileHandler) HandleDeletion(pCtx *plugin.Context) (result plugin.Result) {
	result.Result, result.Error = r.handleDeletion(pCtx)
	return
}

func (r *StoragePoolReconcileHandler) HandleReconcile(pCtx *plugin.Context) (result plugin.Result) {
	var (
		log = pCtx.Log
		ctx = pCtx.ReqCtx.Ctx
		sp  *v1.StoragePool
		ok  bool
	)

	if sp, ok = pCtx.ReqCtx.Object.(*v1.StoragePool); !ok {
		result.Error = fmt.Errorf("object is not *v1.AntstorVolumeGroup, %#v", pCtx.ReqCtx.Object)
		return
	}

	// TODO: move to webhook
	// validate and mutate StoragePool
	result = r.validateAndMutate(sp, log)
	if result.NeedBreak() {
		return
	}

	// save StoragePool to State
	result = r.saveToState(sp, log)
	if result.NeedBreak() {
		return
	}

	// check
	err := r.checkHeartbeat(ctx, sp, log)
	if err != nil {
		log.Error(err, "checking heartbeat with error")
		result.Error = err
		return
	}

	// check if node is deleted
	result = r.processNodeOffline(sp, log)
	if result.NeedBreak() {
		return
	}

	// requeue StoragePool every 5 minutes, to check heartbeat and check Node status
	return plugin.Result{
		Result: ctrl.Result{RequeueAfter: 5 * time.Minute},
	}
}

// checkHeartbeat check Lease and update StoragePool's status
func (r *StoragePoolReconcileHandler) checkHeartbeat(ctx context.Context, sp *v1.StoragePool, log logr.Logger) (err error) {
	var (
		leaseCli = r.KubeCli.CoordinationV1().Leases(LeaseNamespace)
		lease    *coorv1.Lease
		//
		durationSinceLastHB   time.Duration
		lostHB, shouldOffline bool
	)

	// if Agent has not created a Lease after registering a StoragePool, SP's status cannot be updated here.
	// This situation could be avoided if the initialized status of StoragePool is Unknown.
	lease, err = leaseCli.Get(ctx, sp.Name, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "cannot get Lease")
		return
	}

	durationSinceLastHB = time.Since(lease.Spec.RenewTime.Time)
	lostHB = durationSinceLastHB > nodeExpireDuration
	shouldOffline = durationSinceLastHB > nodeOfflineExpireDuration
	log.Info("lease info", "sinceLastHB", durationSinceLastHB, "islostHB", lostHB)

	node, err := r.State.GetNodeByNodeID(lease.Name)
	if err != nil {
		log.Error(err, "cannot find node in State")
		_, err = r.KubeCli.CoreV1().Nodes().Get(context.Background(), lease.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			log.Info("node not exists, so delete lease of node")
			err = leaseCli.Delete(context.Background(), lease.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Error(err, "deleting lease failed")
			}
		}
	}

	// heartbeat is recovered, update status to ready
	if !lostHB && (node.Pool.Status.Status == v1.PoolStatusUnknown || node.Pool.Status.Status == v1.PoolStatusOffline) {
		log.Info("Setting node status to Ready")
		err = r.PoolUtil.UpdateStoragePoolStatus(node.Pool, v1.PoolStatusReady)
		if err != nil {
			log.Error(err, "updating SP status failed")
		}
	}

	// lost heartbeat, update status to unknown
	if lostHB && node.Pool.Status.Status == v1.PoolStatusReady {
		log.Info("Setting node status to Unknown")
		err = r.PoolUtil.UpdateStoragePoolStatus(node.Pool, v1.PoolStatusUnknown)
		if err != nil {
			log.Error(err, "updating SP status failed")
		}
	}

	// lost HB for too long, set status to offline
	if shouldOffline {
		log.Info("Setting node status to Offline")
		err = r.PoolUtil.UpdateStoragePoolStatus(node.Pool, v1.PoolStatusOffline)
		if err != nil {
			log.Error(err, "updating SP status failed")
		}
	}

	return
}

func (r *StoragePoolReconcileHandler) handleDeletion(pCtx *plugin.Context) (result reconcile.Result, err error) {
	var (
		log  = pCtx.Log
		ctx  = pCtx.ReqCtx.Ctx
		sp   *v1.StoragePool
		ok   bool
		node *state.Node
	)
	if sp, ok = pCtx.ReqCtx.Object.(*v1.StoragePool); !ok {
		err = fmt.Errorf("object is not *v1.AntstorVolumeGroup, %#v", pCtx.ReqCtx.Object)
		return
	}
	var name = sp.Name

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
			err = r.Client.Status().Update(ctx, sp)
			if err != nil {
				log.Error(err, "updating StoragePool status failed")
			}
		}
		return reconcile.Result{}, err
	}

	// remove from State
	err = r.State.RemoveStoragePool(sp.Name)
	if err != nil {
		log.Error(err, "RemoveStoragePool failed")
	}

	if len(sp.Finalizers) > 0 {
		log.Info("remove finalizers of StoragePool")
		// remove Finalizers
		sp.Finalizers = []string{}
		err = r.Client.Update(ctx, sp)
		if err != nil {
			log.Error(err, "removing finalizer failed")
		}
	}

	return ctrl.Result{}, nil
}

func (r *StoragePoolReconcileHandler) saveToState(sp *v1.StoragePool, log logr.Logger) (result plugin.Result) {
	var patch = client.MergeFrom(sp.DeepCopy())
	var err error
	var node *state.Node

	r.State.SetStoragePool(sp)

	if !misc.InSliceString(v1.InStateFinalizer, sp.Finalizers) {
		sp.Finalizers = append(sp.Finalizers, v1.InStateFinalizer)
		// do update in APIServer
		log.Info("inject InStateFinalizer to pool")
		err = r.Patch(context.Background(), sp, patch)
		if err != nil {
			log.Error(err, "Update StoragePool failed")
		}
		return plugin.Result{
			Break: true,
			Error: err,
		}
	}

	// try to add reservation by config
	node, err = r.State.GetNodeByNodeID(sp.Name)
	if err != nil {
		log.Error(err, "GetNodeByNodeID error")
	}
	for _, item := range r.Cfg.Scheduler.NodeReservations {
		if node != nil {
			if _, has := node.GetReservation(item.ID); !has {
				node.Reserve(state.NewReservation(item.ID, item.Size))
			}
		}
	}

	return plugin.Result{}
}

func (r *StoragePoolReconcileHandler) validateAndMutate(sp *v1.StoragePool, log logr.Logger) (result plugin.Result) {
	var err error
	var patch = client.MergeFrom(sp.DeepCopy())

	// validation
	if sp.Spec.NodeInfo.ID == "" {
		err = fmt.Errorf("invalid SotragePool, it has no value of NodeInfo.ID")
		log.Error(err, "invalid pool")
		return plugin.Result{
			Error: err,
		}
	}

	// mutation
	// 1. labels cannot be empty. if empty, json patch labels will fail
	if len(sp.Labels) == 0 {
		sp.Labels = make(map[string]string)
		sp.Labels[v1.PoolLabelsNodeSnKey] = sp.Name
		err = r.Patch(context.Background(), sp, patch)
		if err != nil {
			log.Error(err, "Update StoragePool Labels failed")
		}
		return plugin.Result{
			Break: true,
			Error: err,
		}
	}

	// 2. init status and capacity
	if sp.Status.Status == "" || len(sp.Status.Capacity) == 0 {
		// default status is unknown
		sp.Status.Status = v1.PoolStatusUnknown
		// init capacity
		if sp.Status.Capacity == nil {
			sp.Status.Capacity = make(corev1.ResourceList)
		}
		quant := resource.NewQuantity(sp.GetVgTotalBytes(), resource.BinarySI)
		sp.Status.Capacity[v1.ResourceDiskPoolByte] = *quant

		log.Info("update pool status and capacity", "status", sp.Status)
		err = r.Status().Update(context.Background(), sp)
		if err != nil {
			log.Error(err, "update StoragePool/Status failed")
		}
		return plugin.Result{
			Break: true,
			Error: err,
		}
	}

	return plugin.Result{}
}

func (r *StoragePoolReconcileHandler) processNodeOffline(sp *v1.StoragePool, log logr.Logger) (result plugin.Result) {
	/*
		Question: If node does not exist, is it safe to delete the associating StoragePool;
		Condition:
			1. obj.CreationTimestamp is longer than 10min
			2. StoragePool has no AntstorVolume on it
			3. StoragePool status is not ready
			4. Node is deleted
		Action: delete StoragePool
	*/
	sinceCreation := time.Since(sp.CreationTimestamp.Time)
	isNotReady := sp.Status.Status != v1.PoolStatusReady

	// list volumes by node id from cached client
	volList, err := kubeutil.CacheListAnstorVolumeByNodeID(r.Client, sp.Name)
	if err != nil {
		log.Error(err, "FindVolumesByNodeID failed")
		return plugin.Result{Error: err}
	}
	log.Info("check condition to delete StoragePool", "sinceCreation", sinceCreation, "notReady", isNotReady, "vol count", len(volList.Items))

	if len(volList.Items) > 0 {
		for _, item := range volList.Items {
			log.Info("StoragePool has volume left", "volName", item.Name)
		}
	}

	if len(volList.Items) == 0 && isNotReady && sinceCreation > 10*time.Minute {
		var nodeNotFound bool
		var node corev1.Node
		err = r.Client.Get(context.Background(), client.ObjectKey{
			Name: sp.Name,
		}, &node)
		nodeNotFound = errors.IsNotFound(err)
		if err != nil && !nodeNotFound {
			log.Error(err, "get node failed")
			return plugin.Result{Error: err}
		}

		if nodeNotFound {
			log.Info("deleting StoragePool because node does not exist anymore")
			err = r.Delete(context.Background(), sp)
			if err != nil {
				log.Error(err, "deleting StoragePool failed")
				return plugin.Result{Error: err}
			}
		}
	}

	return plugin.Result{}
}
