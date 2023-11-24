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
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/kubeutil"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/reconciler/plugin"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/misc"
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

type StoragePoolReconciler struct {
	client.Client
	plugin.Plugable
	//
	Log logr.Logger
	//
	// NodeGetter kubeutil.NodeInfoGetterIface
	State    state.StateIface
	PoolUtil kubeutil.StoragePoolUpdater
	KubeCli  kubernetes.Interface
	//
	Lock misc.ResourceLockIface
}

// SetupWithManager sets up the controller with the Manager.
func (r *StoragePoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// setup indexer
	// mgr.GetFieldIndexer().IndexField(context.Background(), obj client.Object, field string, extractValue client.IndexerFunc)

	var concurrency = 1
	if r.Lock != nil {
		concurrency = 4
	}
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: concurrency,
		}).
		For(&v1.StoragePool{}).
		Complete(r)
}

func (r *StoragePoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		resourceID = req.NamespacedName.String()
		log        = r.Log.WithValues("StoragePool", resourceID)
		sp         v1.StoragePool
		err        error
		result     plugin.Result
		pCtx       = &plugin.Context{
			Client:  r.Client,
			KubeCli: r.KubeCli,
			Ctx:     ctx,
			Object:  &sp,
			Request: req,
			Log:     log,
			State:   r.State,
		}
	)

	// try get lock by id (ns/name)
	if !r.Lock.TryAcquire(resourceID) {
		log.Info("cannot get lock of the storagepool, skip reconciling.")
		return ctrl.Result{}, nil
	}
	defer r.Lock.Release(resourceID)

	if err := r.Get(ctx, req.NamespacedName, &sp); err != nil {
		// When user deleted a volume, a request will be recieved.
		// However the volume does not exists. Therefore the code goes to here
		log.Error(err, "unable to fetch StoragePool")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		if errors.IsNotFound(err) {
			// remove SP from State
			log.Info("cannot find StoragePool in apiserver, so remove it from State", "error", r.State.RemoveStoragePool(req.Name))
		}

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// not handle delete request
	if sp.DeletionTimestamp != nil {
		// run plugins
		for _, plugin := range r.Plugable.Plugins() {
			plugin.HandleDeletion(pCtx)
		}
		return r.handleDeletion(ctx, sp, log)
	}

	// TODO: move to webhook
	// validate and mutate StoragePool
	result = r.validateAndMutate(sp, log)
	if result.NeedBreak() {
		return result.Result, result.Error
	}

	// save StoragePool to State
	result = r.saveToState(sp, log)
	if result.NeedBreak() {
		return result.Result, result.Error
	}

	// check
	err = r.checkHeartbeat(ctx, sp, log)
	if err != nil {
		log.Error(err, "checking heartbeat with error")
	}

	// check if node is deleted
	result = r.processNodeOffline(sp, log)
	if result.NeedBreak() {
		return result.Result, result.Error
	}

	// run plugins
	for _, plugin := range r.Plugable.Plugins() {
		result = plugin.Reconcile(pCtx)
		if result.NeedBreak() {
			return result.Result, result.Error
		}
	}

	// requeue StoragePool every 5 minutes, to check heartbeat and check Node status
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// checkHeartbeat check Lease and update StoragePool's status
func (r *StoragePoolReconciler) checkHeartbeat(ctx context.Context, sp v1.StoragePool, log logr.Logger) (err error) {
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

func (r *StoragePoolReconciler) handleDeletion(ctx context.Context, sp v1.StoragePool, log logr.Logger) (result reconcile.Result, err error) {
	var node *state.Node
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
			err = r.Client.Status().Update(ctx, &sp)
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
		err = r.Client.Update(ctx, &sp)
		if err != nil {
			log.Error(err, "removing finalizer failed")
		}
	}

	return ctrl.Result{}, nil
}

func (r *StoragePoolReconciler) saveToState(sp v1.StoragePool, log logr.Logger) (result plugin.Result) {
	var patch = client.MergeFrom(sp.DeepCopy())
	var err error

	r.State.SetStoragePool(&sp)

	if !misc.InSliceString(v1.InStateFinalizer, sp.Finalizers) {
		sp.Finalizers = append(sp.Finalizers, v1.InStateFinalizer)
		// do update in APIServer
		log.Info("inject InStateFinalizer to pool")
		err = r.Patch(context.Background(), &sp, patch)
		if err != nil {
			log.Error(err, "Update StoragePool failed")
		}
		return plugin.Result{
			Break: true,
			Error: err,
		}
	}

	return plugin.Result{}
}

func (r *StoragePoolReconciler) validateAndMutate(sp v1.StoragePool, log logr.Logger) (result plugin.Result) {
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
		err = r.Patch(context.Background(), &sp, patch)
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
		err = r.Status().Update(context.Background(), &sp)
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

func (r *StoragePoolReconciler) processNodeOffline(sp v1.StoragePool, log logr.Logger) (result plugin.Result) {
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
			err = r.Delete(context.Background(), &sp)
			if err != nil {
				log.Error(err, "deleting StoragePool failed")
				return plugin.Result{Error: err}
			}
		}
	}

	return plugin.Result{}
}

/*
// TODO: move to agent
func (r *StoragePoolReconciler) fillNodeInfo(sp v1.StoragePool, log logr.Logger, newNodeInfo v1.NodeInfo) (needReturn bool, result reconcile.Result, err error) {
	var patch = client.MergeFrom(sp.DeepCopy())
	if kubeutil.IsNodeInfoDifferent(sp.Spec.NodeInfo, newNodeInfo) {
		sp.Spec.NodeInfo = newNodeInfo
		sp.Spec.Addresses = []corev1.NodeAddress{
			{
				Type:    corev1.NodeInternalIP,
				Address: newNodeInfo.IP,
			},
		}
		// do update in APIServer
		log.Info("update NodeInfo of pool")
		err = r.Patch(context.Background(), &sp, patch)
		if err != nil {
			log.Error(err, "update StoragePool failed")
			return true, ctrl.Result{}, err
		}
		return true, ctrl.Result{}, nil
	}

	return false, ctrl.Result{}, nil
}

*/
