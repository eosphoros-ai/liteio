package reconciler

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/reconciler/plugin"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/misc"
)

type AntstorDataControlReconcileHandler struct {
	client.Client
}

func (r *AntstorDataControlReconcileHandler) ResourceName() string {
	return "AntstorDataControl"
}

func (r *AntstorDataControlReconcileHandler) GetObject(req plugin.RequestContent) (obj runtime.Object, err error) {
	var dataControl v1.AntstorDataControl
	err = r.Client.Get(req.Ctx, req.Request.NamespacedName, &dataControl)
	return &dataControl, err
}

func (r *AntstorDataControlReconcileHandler) HandleDeletion(ctx *plugin.Context) (reuslt plugin.Result) {
	return
}

func (r *AntstorDataControlReconcileHandler) HandleReconcile(pCtx *plugin.Context) plugin.Result {
	var (
		dataControl, ok = pCtx.ReqCtx.Object.(*v1.AntstorDataControl)
		result          plugin.Result
		ctx             = pCtx.ReqCtx.Ctx
		err             error
		log             = pCtx.Log
	)

	if !ok {
		return plugin.Result{
			Error: fmt.Errorf("object is not *v1.AntstorDataControl, %#v", pCtx.ReqCtx.Object),
		}
	}

	// validate and mutate DataControl
	result = r.validateAndMutate(pCtx, dataControl)
	if result.NeedBreak() {
		return result
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
			return plugin.Result{
				Result: ctrl.Result{RequeueAfter: 20 * time.Second},
			}
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
				return plugin.Result{
					Result: ctrl.Result{RequeueAfter: 10 * time.Second},
				}
			} else {
				for _, item := range volGroup.Spec.Volumes {
					var vol v1.AntstorVolume
					err = r.Client.Get(ctx, client.ObjectKey{
						Namespace: item.VolId.Namespace,
						Name:      item.VolId.Name,
					}, &vol)
					if err != nil {
						log.Error(err, "fetching VolGroup failed")
						return plugin.Result{
							Result: ctrl.Result{RequeueAfter: 10 * time.Second},
						}
					}
					hostNode := dataControl.Spec.HostNode
					vol.Spec.HostNode = &hostNode
					err = r.Client.Update(ctx, &vol)
					if err != nil {
						log.Error(err, "updating Volume failed")
						return plugin.Result{
							Result: ctrl.Result{RequeueAfter: 10 * time.Second},
						}
					}
				}
			}
		}

		err = r.Client.Update(ctx, dataControl)
		if err != nil {
			log.Error(err, "updating DataControl failed")
			return plugin.Result{
				Result: ctrl.Result{RequeueAfter: 10 * time.Second},
			}
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
				return plugin.Result{
					Result: ctrl.Result{RequeueAfter: 10 * time.Second},
				}
			} else {
				for _, item := range volGroup.Spec.Volumes {
					var vol v1.AntstorVolume
					err = r.Client.Get(ctx, client.ObjectKey{
						Namespace: item.VolId.Namespace,
						Name:      item.VolId.Name,
					}, &vol)
					if err != nil {
						log.Error(err, "fetching VolGroup failed")
						return plugin.Result{
							Result: ctrl.Result{RequeueAfter: 10 * time.Second},
						}
					}
					if vol.Status.CSINodePubParams == nil || vol.Status.CSINodePubParams.TargetPath == "" {
						vol.Status.CSINodePubParams = dataControl.Status.CSINodePubParams
						err = r.Client.Status().Update(ctx, &vol)
						if err != nil {
							log.Error(err, "updating Volume Status failed")
							return plugin.Result{
								Result: ctrl.Result{RequeueAfter: 10 * time.Second},
							}
						}
					}
				}
			}
		}
	}

	return result
}

func (r *AntstorDataControlReconcileHandler) validateAndMutate(ctx *plugin.Context, dataControl *v1.AntstorDataControl) (result plugin.Result) {
	if !misc.InSliceString(string(dataControl.Spec.EngineType),
		[]string{string(v1.PoolModeKernelLVM), string(v1.PoolModeSpdkLVStore)}) {
		return plugin.Result{Error: fmt.Errorf("invalid type %s", dataControl.Spec.EngineType)}
	}

	return plugin.Result{}
}
