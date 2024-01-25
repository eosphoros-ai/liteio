package reconciler

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"lite.io/liteio/pkg/controller/manager/reconciler/plugin"
	sched "lite.io/liteio/pkg/controller/manager/scheduler"
	"lite.io/liteio/pkg/controller/manager/scheduler/filter"
	"lite.io/liteio/pkg/controller/manager/state"
	"lite.io/liteio/pkg/generated/clientset/versioned"
	"lite.io/liteio/pkg/util/misc"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	EventReasonDeleteSpdkFailure  = "DelSpdkFailure"
	EventReasonDeleteLvmFailure   = "DelLvmFailure"
	EventReasonSchedVolumeFailure = "SchedVolumeFailure"
	EventReasonCreateLvmFailure   = "CreateLvmFailure"
	EventReasonCreateSpdkFailure  = "CreateSpdkFailure"
)

type AntstorVolumeReconcileHandler struct {
	client.Client

	KubeCli kubernetes.Interface
	State   state.StateIface
	// if Scheduler is nil, Reconciler will not schedule Volume
	Scheduler   sched.SchedulerIface
	AntstoreCli versioned.Interface
	// EventRecorder
	// EventRecorder record.EventRecorder
}

func (r *AntstorVolumeReconcileHandler) ResourceName() string {
	return "AntstorVolume"
}

func (r *AntstorVolumeReconcileHandler) GetObject(req plugin.RequestContent) (obj runtime.Object, err error) {
	var vol v1.AntstorVolume
	err = r.Client.Get(req.Ctx, req.Request.NamespacedName, &vol)
	return &vol, err
}

func (r *AntstorVolumeReconcileHandler) HandleDeletion(pCtx *plugin.Context) (result plugin.Result) {
	result = r.handleDeletion(pCtx)
	return
}

func (r *AntstorVolumeReconcileHandler) HandleReconcile(pCtx *plugin.Context) (result plugin.Result) {
	var (
		ctx    = pCtx.ReqCtx.Ctx
		log    = pCtx.Log
		volume *v1.AntstorVolume
		ok     bool
	)
	if volume, ok = pCtx.ReqCtx.Object.(*v1.AntstorVolume); !ok {
		result.Error = fmt.Errorf("object is not *v1.AntstorVolume, %#v", pCtx.ReqCtx.Object)
		return
	}

	log.Info("Start handle Volume", "status", volume.Status)
	// check if stop reconcling
	if volume.Spec.StopReconcile {
		return
	}

	// validate and mutate volume
	result = r.validateAndMutate(ctx, volume, log)
	if result.NeedBreak() {
		return
	}

	// schedule volume
	result = r.scheduleVolume(ctx, volume, log)
	if result.NeedBreak() {
		return
	}

	return
}

func (r *AntstorVolumeReconcileHandler) handleDeletion(pCtx *plugin.Context) (result plugin.Result) {
	var (
		err    error
		ctx    = pCtx.ReqCtx.Ctx
		log    = pCtx.Log
		volume *v1.AntstorVolume
		ok     bool
	)
	if volume, ok = pCtx.ReqCtx.Object.(*v1.AntstorVolume); !ok {
		err = fmt.Errorf("object is not *v1.AntstorVolume, %#v", pCtx.ReqCtx.Object)
		return
	}

	// 1. if there is only InStateFinalizer Finalizer left, unbind and delete volume
	if len(volume.Finalizers) == 1 && misc.Contains(volume.Finalizers, v1.InStateFinalizer) {
		// remove from state
		log.Info("remove volume from state", "volID", volume.Spec.Uuid)
		err = r.State.UnbindAntstorVolume(volume.Spec.Uuid)
		if err != nil {
			log.Error(err, "UnbindAntstorVolume failed")
		}

		// remove all Finalizers
		volume.Finalizers = []string{}
		err = r.Client.Update(ctx, volume)
		if err != nil {
			log.Error(err, "remove Finalizers failed")
		}
		return plugin.Result{Error: err}
	}

	// 2. if StoragePool does not exist, unbind and delete volume
	var targetPool *v1.StoragePool
	targetPool, err = r.AntstoreCli.VolumeV1().StoragePools(v1.DefaultNamespace).Get(ctx, volume.Spec.TargetNodeId, metav1.GetOptions{})
	// error is not NotFound, throw the error out
	if client.IgnoreNotFound(err) != nil {
		log.Error(err, "Get StoragePool. An error occured", "nodeid", volume.Spec.TargetNodeId)
		return plugin.Result{Error: err}
	}
	// targetPool is not found. It was deleted from cluster, so remove the volume right away.
	if targetPool == nil {
		log.Info("cannot find pool in APIServer, remove volume Finalizers", "nodeid", volume.Spec.TargetNodeId, "UnbindAntstorVolume error", r.State.UnbindAntstorVolume(volume.Spec.Uuid))
		volume.Finalizers = []string{}
		err = r.Client.Update(ctx, volume)
		if err != nil {
			log.Error(err, "remove Finalizers failed")
		}
		return plugin.Result{Error: err}
	}

	// 3. handle node failure
	/*
		If node failure happens, finalizer "antstor.alipay.com/lvm" and "antstor.alipay.com/spdk-tgt" will not be deleted by DiskAgent.
		Reconciler should figure out and delete finalizers on following conditions:
		1. 20 min past since DeletionTimestamp
		2. StoragePool status is not ready. It means DiskAgent is not updating Lease.
		3. length of Finalizers > 1
	*/
	sinceDel := time.Since(volume.DeletionTimestamp.Time)
	isPoolNotReady := targetPool.Status.Status != v1.PoolStatusReady
	log.Info("check condition to delete Volume", "sinceDel", sinceDel, "poolUnhealthy", isPoolNotReady)
	if sinceDel >= 20*time.Minute && isPoolNotReady && len(volume.Finalizers) > 1 {
		log.Info("volume is stuck at deleting lvm and spdk, because node is unhealthy. remove some finalizer.", "nodeid", volume.Spec.TargetNodeId)
		// only remove v1.KernelLVolFinalizer and v1.SpdkTargetFinalizer
		var finalizers = make([]string, 0, len(volume.Finalizers))
		for _, item := range volume.Finalizers {
			if item == v1.KernelLVolFinalizer || item == v1.SpdkTargetFinalizer {
				continue
			}
			finalizers = append(finalizers, item)
		}

		volume.Finalizers = finalizers
		err = r.Client.Update(ctx, volume)
		if err != nil {
			log.Error(err, "update volume Finalizer failed")
		}
		return plugin.Result{Error: err}
	}

	log.Info("volume is pending to be removed, requeue after 2min", "Finalizers", volume.Finalizers, "RV", volume.ResourceVersion)
	return plugin.Result{
		Result: reconcile.Result{RequeueAfter: 2 * time.Minute},
	}
}

func (r *AntstorVolumeReconcileHandler) validateAndMutate(ctx context.Context, volume *v1.AntstorVolume, log logr.Logger) (result plugin.Result) {
	if volume.Spec.Uuid == "" {
		log.Error(nil, "volume uuid is empty, break reconcilinig")
		return plugin.Result{
			Result: ctrl.Result{RequeueAfter: 2 * time.Minute},
		}
	}

	// TODO: check if spec.PositionAdvice value equals to PVC.Annotations[custom.k8s.alipay.com/sds-position-advice]

	var (
		stateObj       = r.State
		patch          = client.MergeFrom(volume.DeepCopy())
		foundUuidLabel bool
		err            error
	)

	// if uuid is not set in Labels, set the uuid label
	_, foundUuidLabel = volume.Labels[v1.UuidLabelKey]
	if !foundUuidLabel && volume.Spec.Uuid != "" {
		volume.Labels[v1.UuidLabelKey] = volume.Spec.Uuid
		// after adding Label, start a new reconciling
		err := r.Client.Patch(context.Background(), volume, patch)
		if err != nil {
			log.Error(err, "add uuid label failed")
			return plugin.Result{
				Error: err,
			}
		}
		return plugin.Result{
			Break: true,
			Error: err,
		}
	}

	if volume.Status.Status == "" {
		volume.Status.Status = v1.VolumeStatusCreating
		err := r.Status().Update(ctx, volume)
		return plugin.Result{
			Break: true,
			Error: err,
		}
	}

	// validate lv layout value
	if val, has := volume.Annotations[v1.LvLayoutAnnoKey]; has {
		if !misc.InSliceString(val, []string{"", string(v1.LVLayoutLinear), string(v1.LVLayoutStriped)}) {
			volume.Status.Message = fmt.Sprintf("invalide value of Anno obnvmf/lv-layout=%s", val)
			err := r.Client.Status().Update(context.Background(), volume)
			if err != nil {
				log.Error(err, "update status message failed")
				return plugin.Result{
					Error: err,
				}
			}
			return plugin.Result{
				Break: true,
				Error: err,
			}
		}
	}

	// bind volume to state
	if volume.Spec.TargetNodeId != "" {
		var updated bool
		// volume was scheduled to TargetNodeId, check if Pool is in state
		// BindAntstorVolume one volume twice will not return error
		log.Info("bind volume to node", "nodeId", volume.Spec.TargetNodeId)
		err = stateObj.BindAntstorVolume(volume.Spec.TargetNodeId, volume)
		if err != nil {
			log.Error(err, "binding volume failed")
			return plugin.Result{
				Error: err,
			}
		}

		// set TargetNodeIdLabelKey, so the agent will start creating the volume
		if _, has := volume.Labels[v1.TargetNodeIdLabelKey]; !has {
			volume.Labels[v1.TargetNodeIdLabelKey] = volume.Spec.TargetNodeId
			updated = true
		}

		// add InStateFinalizer to volume
		var foundStateFinalizer bool
		for _, item := range volume.Finalizers {
			if item == v1.InStateFinalizer {
				foundStateFinalizer = true
				break
			}
		}
		if !foundStateFinalizer {
			volume.Finalizers = append(volume.Finalizers, v1.InStateFinalizer)
			updated = true
		}

		// update Volume
		if updated {
			err = r.Client.Patch(context.Background(), volume, patch)
			if err != nil {
				log.Error(err, "patch volume failed")
				return plugin.Result{
					Error: err,
				}
			}
		}
	}

	return plugin.Result{}
}

func (r *AntstorVolumeReconcileHandler) scheduleVolume(ctx context.Context, volume *v1.AntstorVolume, log logr.Logger) (result plugin.Result) {
	var (
		scheduler = r.Scheduler
		stateObj  = r.State
		patch     = client.MergeFrom(volume.DeepCopy())
		nodeInfo  v1.NodeInfo
		boundVol  *v1.AntstorVolume
		err       error
	)

	if scheduler != nil && volume.Spec.TargetNodeId == "" {
		// check if the volume is in State
		boundVol, err = stateObj.GetVolumeByID(volume.Spec.Uuid)
		if err != nil {
			log.Error(err, "state getting volume by uuid failed")
		}
		if err == nil && boundVol != nil && boundVol.Spec.TargetNodeId != "" {
			log.Info("volume is already scheduled and bind to node", "nodeId", boundVol.Spec.TargetNodeId)
			volume.Spec.TargetNodeId = boundVol.Spec.TargetNodeId
			err = r.Client.Update(ctx, volume)
			if err != nil {
				log.Error(err, "persist binding of volume failed")
				return plugin.Result{Error: err}
			}
			// break scheduling
			return plugin.Result{
				Break: true,
			}
		}

		// do scehdule
		nodeInfo, err = scheduler.ScheduleVolume(stateObj.GetAllNodes(), volume)
		if filter.IsNoStoragePoolAvailable(err) {
			if !strings.Contains(volume.Status.Message, filter.NoStoragePoolAvailable) {
				volume.Status.Message = err.Error()
				volume.Status.Status = v1.VolumeStatusCreating
				errUpdate := r.Client.Status().Update(ctx, volume)
				if errUpdate != nil {
					log.Error(errUpdate, "update volume status failed")
				}
			}

			log.Error(err, "no Pool is fit for volume")
			// record event
			// r.EventRecorder.Event(&volume, corev1.EventTypeWarning, EventReasonSchedVolumeFailure, err.Error())

			// non-nil err will cause requeue
			return plugin.Result{
				Break:  true,
				Result: reconcile.Result{RequeueAfter: 20 * time.Second},
			}
		}
		if err != nil {
			log.Error(err, "schedule volume failed")
			// if err is not nil, reconcile will be triggered immediately
			return plugin.Result{Error: err}
		}

		// save binding to state
		log.Info("volume is scheduled to node", "nodeId", nodeInfo.ID)
		err = stateObj.BindAntstorVolume(nodeInfo.ID, volume)
		if err != nil {
			log.Error(err, "bind volume to node failed", "nodeID", nodeInfo.ID)
			return plugin.Result{Error: err}
		}

		// save TargetNodeId, return, start doing a new reconcile
		// Code example from https://sdk.operatorframework.io/docs/building-operators/golang/references/client/#patch
		if volume.Labels == nil {
			volume.Labels = make(map[string]string)
		}
		volume.Spec.TargetNodeId = nodeInfo.ID
		volume.Labels[v1.TargetNodeIdLabelKey] = volume.Spec.TargetNodeId
		volume.Status.Status = v1.VolumeStatusCreating
		err = r.Client.Patch(ctx, volume, patch)
		if err != nil {
			log.Error(err, "patching volume failed")
			return plugin.Result{Error: err}
		}

		return plugin.Result{Break: true}
	}

	return
}
