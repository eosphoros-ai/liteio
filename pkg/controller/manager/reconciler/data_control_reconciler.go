package reconciler

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/reconciler/plugin"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/misc"
)

type AntstorDataControlReconciler struct {
	client.Client
	plugin.Plugable

	Log   logr.Logger
	State state.StateIface
}

// SetupWithManager sets up the controller with the Manager.
func (r *AntstorDataControlReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		For(&v1.AntstorDataControl{}).
		Complete(r)
}

func (r *AntstorDataControlReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		resourceID  = req.NamespacedName.String()
		log         = r.Log.WithValues("DataControl", resourceID)
		dataControl v1.AntstorDataControl
		err         error
		result      plugin.Result
		pCtx        = &plugin.Context{
			Client:  r.Client,
			Ctx:     ctx,
			Object:  &dataControl,
			Request: req,
			Log:     log,
			State:   r.State,
		}
	)

	if err = r.Get(ctx, req.NamespacedName, &dataControl); err != nil {
		// When user deleted a volume, a request will be recieved.
		// However the volume does not exists. Therefore the code goes to here
		log.Error(err, "unable to fetch DataControl")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		if errors.IsNotFound(err) {
			// remove SP from State
			log.Info("cannot find DataControl in apiserver")
		}

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// not handle delete request
	if dataControl.DeletionTimestamp != nil {
		// run plugins
		for _, plugin := range r.Plugable.Plugins() {
			plugin.HandleDeletion(pCtx)
		}
		return r.handleDeletion(pCtx, &dataControl)
	}

	// validate and mutate DataControl
	result = r.validateAndMutate(pCtx, &dataControl)
	if result.NeedBreak() {
		return result.Result, result.Error
	}

	// sync volume group status

	// wait all volumes are ready
	for _, item := range dataControl.Spec.VolumeGroups {
		var vg v1.AntstorVolumeGroup
		var key = client.ObjectKey{
			Namespace: item.Namespace,
			Name:      item.Name,
		}
		err = r.Client.Get(ctx, key, &vg)
		if err != nil {
			log.Error(err, "fetching VolumeGroup failed", "name", key)
		}

		if vg.Status.Status != v1.VolumeStatusReady {
			log.Info("VolumeGroup is not ready yet, retry in 20 sec", "name", key, "status", vg.Status.Status)
			return ctrl.Result{RequeueAfter: 20 * time.Second}, nil
		}
	}

	// sched DataControl
	if dataControl.Spec.TargetNodeId == "" {
		log.Info("try to schedule DataControl")
		// if type is LVM, set the DataControl node to Host node
		switch dataControl.Spec.EngineType {
		case v1.PoolModeKernelLVM:
			// set targetNode to host
			dataControl.Spec.TargetNodeId = dataControl.Spec.HostNode.ID
			dataControl.Labels[v1.TargetNodeIdLabelKey] = dataControl.Spec.HostNode.ID
			log.Info("schedule DataControl to node", "nodeid", dataControl.Spec.HostNode.ID)
		case v1.PoolModeSpdkLVStore:
			log.Info("DataControl type SpdkLVStore is not supported yet")
		}

		// get volgroups and update hostnode of Volumes
		// TODO: updating hostnode if host is changed
		for _, item := range dataControl.Spec.VolumeGroups {
			var volGroup v1.AntstorVolumeGroup
			err = r.Client.Get(ctx, client.ObjectKey{
				Namespace: item.Namespace,
				Name:      item.Name,
			}, &volGroup)
			if err != nil {
				log.Error(err, "fetching VolGroup failed")
				return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
			} else {
				for _, item := range volGroup.Spec.Volumes {
					var vol v1.AntstorVolume
					err = r.Client.Get(ctx, client.ObjectKey{
						Namespace: item.VolId.Namespace,
						Name:      item.VolId.Name,
					}, &vol)
					if err != nil {
						log.Error(err, "fetching VolGroup failed")
						return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
					}
					hostNode := dataControl.Spec.HostNode
					vol.Spec.HostNode = &hostNode
					err = r.Client.Update(ctx, &vol)
					if err != nil {
						log.Error(err, "updating Volume failed")
						return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
					}
				}
			}
		}

		err = r.Client.Update(ctx, &dataControl)
		if err != nil {
			log.Error(err, "updating DataControl failed")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}

	if dataControl.Status.CSINodePubParams != nil {
		// TODO: updating csi params
		for _, item := range dataControl.Spec.VolumeGroups {
			var volGroup v1.AntstorVolumeGroup
			err = r.Client.Get(ctx, client.ObjectKey{
				Namespace: item.Namespace,
				Name:      item.Name,
			}, &volGroup)
			if err != nil {
				log.Error(err, "fetching VolGroup failed")
				return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
			} else {
				for _, item := range volGroup.Spec.Volumes {
					var vol v1.AntstorVolume
					err = r.Client.Get(ctx, client.ObjectKey{
						Namespace: item.VolId.Namespace,
						Name:      item.VolId.Name,
					}, &vol)
					if err != nil {
						log.Error(err, "fetching VolGroup failed")
						return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
					}
					if vol.Status.CSINodePubParams == nil || vol.Status.CSINodePubParams.TargetPath == "" {
						vol.Status.CSINodePubParams = dataControl.Status.CSINodePubParams
						err = r.Client.Status().Update(ctx, &vol)
						if err != nil {
							log.Error(err, "updating Volume Status failed")
							return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
						}
					}
				}
			}
		}
	}

	// run plugins
	for _, plugin := range r.Plugable.Plugins() {
		result = plugin.Reconcile(pCtx)
		if result.NeedBreak() {
			return result.Result, result.Error
		}
	}

	return ctrl.Result{}, nil
}

func (r *AntstorDataControlReconciler) handleDeletion(ctx *plugin.Context, dataControl *v1.AntstorDataControl) (result reconcile.Result, err error) {
	// resouce cleaning is done in agent

	return ctrl.Result{}, nil
}

func (r *AntstorDataControlReconciler) validateAndMutate(ctx *plugin.Context, dataControl *v1.AntstorDataControl) (result plugin.Result) {
	if !misc.InSliceString(string(dataControl.Spec.EngineType),
		[]string{string(v1.PoolModeKernelLVM), string(v1.PoolModeSpdkLVStore)}) {
		return plugin.Result{Error: fmt.Errorf("invalid type %s", dataControl.Spec.EngineType)}
	}

	return plugin.Result{}
}
