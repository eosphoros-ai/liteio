package extender

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	sched "code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/scheduler"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/scheduler/filter"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/generated/clientset/versioned"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/misc"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type AntstorVolumeReconciler struct {
	// Client           client.Client
	AntstoreCli versioned.Interface
	State       state.StateIface
	Scheduler   sched.SchedulerIface
	// AutoAdjustHelper plugin.AdjustLocalStorageHelperIface
}

func (r *AntstorVolumeReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var (
		ns     = req.Namespace
		name   = req.Name
		log    = log.Log.WithName("ScheduleQueue").WithValues("key", req.NamespacedName)
		volume *v1.AntstorVolume
		err    error
		volCli = r.AntstoreCli.VolumeV1().AntstorVolumes(ns)
	)

	volume, err = volCli.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		log.Error(err, "fetching Volume failed")
		return reconcile.Result{}, err
	}

	if volume.Spec.Uuid == "" {
		err = fmt.Errorf("volume %s missing uuid", name)
		log.Error(err, "volume uuid is empty, not schedule")
		return reconcile.Result{RequeueAfter: time.Minute}, nil
	}

	var patch = client.MergeFrom(volume.DeepCopy())
	var stateObj = r.State
	var scheduler = r.Scheduler

	if volume.DeletionTimestamp != nil {
		// Remove InStateFinalizer Finalizer
		if len(volume.Finalizers) == 1 && misc.Contains(volume.Finalizers, v1.InStateFinalizer) {
			// remove from state
			log.Info("remove volume from state", "volID", volume.Spec.Uuid)
			err = r.State.UnbindAntstorVolume(volume.Spec.Uuid)
			if err != nil {
				log.Error(err, "UnbindAntstorVolume failed")
			}

			// remove all Finalizers
			volume.Finalizers = []string{}
			_, err = volCli.Update(ctx, volume, metav1.UpdateOptions{})
			if err != nil {
				log.Error(err, "remove Finalizers failed")
			}
			return reconcile.Result{}, err
		} else {
			log.Info("volume is pending to be removed", "Finalizers", volume.Finalizers, "RV", volume.ResourceVersion)
		}

		// if StoragePool not exists, unbind volume and delete volume
		// get storage pool
		var spCli = r.AntstoreCli.VolumeV1().StoragePools(v1.DefaultNamespace)
		var targetPool *v1.StoragePool
		targetPool, err = spCli.Get(ctx, volume.Spec.TargetNodeId, metav1.GetOptions{})
		// error is not NotFound, throw the error out
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Get StoragePool. An error occured", "nodeid", volume.Spec.TargetNodeId, "volName", req.NamespacedName)
			return reconcile.Result{}, err
		}

		// targetPool is not found. It was deleted from cluster, so remove the volume right away.
		if targetPool == nil {
			log.Info("cannot find pool in APIServer, remove volume Finalizers", "nodeid", volume.Spec.TargetNodeId, "volName", req.NamespacedName)
			log.Info("UnbindAntstorVolume", "result", r.State.UnbindAntstorVolume(volume.Spec.Uuid))
			volume.Finalizers = []string{}
			_, err = volCli.Update(ctx, volume, metav1.UpdateOptions{})
			if err != nil {
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		}

		/*
			If node failure happens, finalizer "antstor.alipay.com/lvm" and "antstor.alipay.com/spdk-tgt" will not be deleted by DiskAgent.
			Reconciler should figure out and delete finalizers.
			Conditin:
			1. past 20min since DeletionTimestamp
			2. Pool status is not ready
			3. length of volume.Finalizers > 1
		*/
		sinceDel := time.Since(volume.DeletionTimestamp.Time)
		isNotReady := targetPool.Status.Status != v1.PoolStatusReady
		log.Info("check condition to delete Volume", "sinceDel", sinceDel, "notReady", isNotReady)
		if sinceDel >= 20*time.Minute && isNotReady && len(volume.Finalizers) > 1 {
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
			_, err = volCli.Update(ctx, volume, metav1.UpdateOptions{})
			if err != nil {
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	if volume.Spec.TargetNodeId == "" {
		// if patching volume failed, it should re-do binding
		var bindedVol *v1.AntstorVolume
		bindedVol, err = stateObj.GetVolumeByID(volume.Spec.Uuid)
		if err == nil && bindedVol != nil && bindedVol.Spec.TargetNodeId != "" {
			log.Info("volume is already scheduled and bind to node", "nodeId", bindedVol.Spec.TargetNodeId)
			volume.Spec.TargetNodeId = bindedVol.Spec.TargetNodeId
			// err = r.Client.Patch(context.Background(), volume, patch)
			// volCli.Patch(ctx, volume.Name, types.MergePatchType, patch.Data(volume), metav1.PatchOptions{})
			_, err = volCli.Update(ctx, volume, metav1.UpdateOptions{})
			if err != nil {
				log.Error(err, "persist binding of volume failed")
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		}

		// do scehdule
		var nodeInfo v1.NodeInfo
		nodeInfo, err = scheduler.ScheduleVolume(stateObj.GetAllNodes(), volume)
		if filter.IsNoStoragePoolAvailable(err) {
			if !strings.Contains(volume.Status.Message, filter.NoStoragePoolAvailable) {
				volume.Status.Message = err.Error()
				volume.Status.Status = v1.VolumeStatusCreating
				_, errUpdate := volCli.UpdateStatus(ctx, volume, metav1.UpdateOptions{})
				if errUpdate != nil {
					log.Error(errUpdate, "update volume status failed")
				}
			}

			log.Error(err, "no Pool is fit for volume")
			// record event
			// r.EventRecorder.Event(&volume, corev1.EventTypeWarning, EventReasonSchedVolumeFailure, err.Error())

			// if r.AutoAdjustHelper != nil {
			// 	err = r.AutoAdjustHelper.AdjustWatermarkForRemoteVolume(volume)
			// 	if err != nil {
			// 		log.Error(err, "AdjustWatermarkForRemoteVolume failed")
			// 	}
			// }

			// not-nil err will cause requeue
			return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
		}
		if err != nil {
			log.Error(err, "schedule volume failed")
			// if err is not nil, reconcile will be triggered immediately
			return reconcile.Result{}, err
		}

		err = stateObj.BindAntstorVolume(volume.Spec.TargetNodeId, volume)
		if err != nil {
			log.Error(err, "bind volume to node failed", "nodeID", volume.Spec.TargetNodeId)
			return reconcile.Result{}, err
		}

		// save TargetNodeId, return, start doing a new reconcile
		// Code example from https://sdk.operatorframework.io/docs/building-operators/golang/references/client/#patch
		volume.Spec.TargetNodeId = nodeInfo.ID
		if volume.Labels == nil {
			volume.Labels = make(map[string]string)
		}
		volume.Labels[v1.TargetNodeIdLabelKey] = volume.Spec.TargetNodeId
		volume.Status.Status = v1.VolumeStatusCreating

		var patchJsonData []byte
		patchJsonData, err = patch.Data(volume)
		if err != nil {
			log.Error(err, "get patch data failed")
			return reconcile.Result{}, err
		}
		_, err = volCli.Patch(ctx, volume.Name, types.MergePatchType, patchJsonData, metav1.PatchOptions{})
		if err != nil {
			log.Error(err, "patching volume failed")
			return reconcile.Result{}, err
		}

		// TODO: should update status in Agent
		// err = r.Client.Status().Update(context.Background(), volume)
		_, err = volCli.UpdateStatus(ctx, volume, metav1.UpdateOptions{})
		if err != nil {
			log.Error(err, "patch volume status failed")
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, err
	} else {
		// volume was scheduled to TargetNodeId, check if Pool is in state
		// BindAntstorVolume one volume twice will not return error
		log.Info("BindAntstorVolume", "nodeId", volume.Spec.TargetNodeId)
		err = stateObj.BindAntstorVolume(volume.Spec.TargetNodeId, volume)
		if err != nil {
			log.Error(err, "binding volume failed")
			return reconcile.Result{}, err
		}

		// set TargetNodeIdLabelKey, so the agent will start creating the volume
		if _, has := volume.Labels[v1.TargetNodeIdLabelKey]; !has {
			volume.Labels[v1.TargetNodeIdLabelKey] = volume.Spec.TargetNodeId
			_, err = volCli.Update(ctx, volume, metav1.UpdateOptions{})
			if err != nil {
				log.Error(err, "add InStateFinalizer failed")
			}
			return reconcile.Result{}, err
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
			// after adding Finalizer, start a new reconciling
			// err = r.Client.Patch(context.Background(), volume, patch)
			_, err = volCli.Update(ctx, volume, metav1.UpdateOptions{})
			if err != nil {
				log.Error(err, "add InStateFinalizer failed")
				return reconcile.Result{}, err
			}
		}

		// if r.AutoAdjustHelper != nil && !volume.IsLocal() {
		// 	node, err := stateObj.GetNodeByNodeID(volume.Spec.TargetNodeId)
		// 	if err != nil {
		// 		log.Error(err, "find node failed")
		// 		return reconcile.Result{}, err
		// 	}
		// 	err = r.AutoAdjustHelper.RemoveAdjustingLabel(node.Pool)
		// 	if err != nil {
		// 		log.Error(err, "RemoveAdjustingLabel failed")
		// 		return reconcile.Result{}, err
		// 	}
		// }
	}

	return reconcile.Result{}, err
}
