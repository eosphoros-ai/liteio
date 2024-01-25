package reconciler

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"lite.io/liteio/pkg/controller/manager/reconciler/plugin"
	sched "lite.io/liteio/pkg/controller/manager/scheduler"
	"lite.io/liteio/pkg/controller/manager/scheduler/filter"
	"lite.io/liteio/pkg/controller/manager/state"
	"lite.io/liteio/pkg/util/misc"
)

type AntstorVolumeGroupReconcileHandler struct {
	client.Client

	State     state.StateIface
	Scheduler sched.SchedulerIface
}

func (r *AntstorVolumeGroupReconcileHandler) ResourceName() string {
	return "AntstorVolumeGroup"
}

func (r *AntstorVolumeGroupReconcileHandler) GetObject(req plugin.RequestContent) (obj runtime.Object, err error) {
	var volGroup v1.AntstorVolumeGroup
	err = r.Client.Get(req.Ctx, req.Request.NamespacedName, &volGroup)
	return &volGroup, err
}

func (r *AntstorVolumeGroupReconcileHandler) HandleDeletion(pCtx *plugin.Context) (result plugin.Result) {
	result.Result, result.Error = r.handleDeletion(pCtx)
	return
}

func (r *AntstorVolumeGroupReconcileHandler) HandleReconcile(pCtx *plugin.Context) (result plugin.Result) {
	var (
		volGroup *v1.AntstorVolumeGroup
		ok       bool
	)
	if volGroup, ok = pCtx.ReqCtx.Object.(*v1.AntstorVolumeGroup); !ok {
		result.Error = fmt.Errorf("object is not *v1.AntstorVolumeGroup, %#v", pCtx.ReqCtx.Object)
		return
	}

	// validate and mutate VolumeGroup
	result = r.validateAndMutate(pCtx, volGroup)
	if result.NeedBreak() {
		return
	}

	// sync volume
	result = r.syncVolumes(pCtx, volGroup)
	if result.NeedBreak() {
		return
	}

	result = r.rollbackUnrecoverable(pCtx, volGroup)
	if result.NeedBreak() {
		return
	}

	// schedule volumes for volume group
	result = r.scheduleVolGroup(pCtx, volGroup)
	if result.NeedBreak() {
		return
	}

	// if volumes are not all ready, reconcile the volGroup
	result = r.waitVolumesReady(pCtx, volGroup)
	if result.NeedBreak() {
		return
	}

	return
}

func (r *AntstorVolumeGroupReconcileHandler) handleDeletion(ctx *plugin.Context) (result reconcile.Result, err error) {
	var (
		log      = ctx.Log
		volGroup *v1.AntstorVolumeGroup
		ok       bool
	)
	if volGroup, ok = ctx.ReqCtx.Object.(*v1.AntstorVolumeGroup); !ok {
		err = fmt.Errorf("object is not *v1.AntstorVolumeGroup, %#v", ctx.ReqCtx.Object)
		return
	}

	// TODO: wait until data control is deleted
	if val, has := volGroup.Labels[v1.DataControlNameKey]; has {
		var dc v1.AntstorDataControl
		var key = client.ObjectKey{
			Namespace: v1.DefaultNamespace,
			Name:      val,
		}
		err = r.Client.Get(ctx.ReqCtx.Ctx, key, &dc)
		log.Info("try to find datacontrol", "key", key, "err", err)
		if !errors.IsNotFound(err) {
			log.Info("wait datacontrol to be deleted, retry in 20 second", "key", key)
			return reconcile.Result{RequeueAfter: 20 * time.Second}, nil
		}
	}

	if misc.InSliceString(v1.VolumesFinalizer, volGroup.Finalizers) {
		// delete all volumes
		for _, volMeta := range volGroup.Spec.Volumes {
			var volume v1.AntstorVolume
			var key = client.ObjectKey{
				Namespace: volMeta.VolId.Namespace,
				Name:      volMeta.VolId.Name,
			}
			err = r.Client.Get(ctx.ReqCtx.Ctx, key, &volume)
			if errors.IsNotFound(err) {
				log.Info("volume is deleted", "vol", key)
			} else {
				err = r.Client.Delete(ctx.ReqCtx.Ctx, &volume)
				if err != nil {
					log.Error(err, "delete vol failed. retry in 10 sec", "vol", key)
					return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
				}

				log.Info("wait volume to be deleted, retry in 10 sec", "vol", key)
				return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
			}
		}

		// remove finalizer
		var toDelIdx int
		for idx, item := range volGroup.Finalizers {
			if item == v1.VolumesFinalizer {
				toDelIdx = idx
				break
			}
		}
		volGroup.Finalizers = append(volGroup.Finalizers[:toDelIdx], volGroup.Finalizers[toDelIdx+1:]...)

		// update object
		err = r.Client.Update(ctx.ReqCtx.Ctx, volGroup)
		if err != nil {
			log.Error(err, "update volumegroup failed")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *AntstorVolumeGroupReconcileHandler) validateAndMutate(ctx *plugin.Context, volGroup *v1.AntstorVolumeGroup) (result plugin.Result) {
	var err error
	var minQ = volGroup.Spec.DesiredVolumeSpec.SizeRange.Min
	var maxQ = volGroup.Spec.DesiredVolumeSpec.SizeRange.Max

	// validation
	switch volGroup.Spec.DesiredVolumeSpec.SizeSymmetry {
	case v1.Symmetric:
		// Symmetric VolumeGroup must have DesiredCount
		if volGroup.Spec.DesiredVolumeSpec.DesiredCount == 0 {
			err = fmt.Errorf("invalid Symmetric VolumeGroup, DesiredCount is 0")
			return plugin.Result{
				Error: err,
			}
		}

		// size range is correct
		if maxQ.IsZero() {
			err = fmt.Errorf("invalid VolumeGroup, SizeRange.Max is 0")
			return plugin.Result{
				Error: err,
			}
		}
		if !minQ.IsZero() {
			singleVolSize := volGroup.Spec.TotalSize / int64(volGroup.Spec.DesiredVolumeSpec.DesiredCount)
			if minQ.CmpInt64(singleVolSize) > 0 {
				err = fmt.Errorf("invalid Symmetric VolumeGroup, SizeRange.Min %s is larger than singleVolSize %d", minQ.String(), singleVolSize)
				return plugin.Result{
					Error: err,
				}
			}
		}
	case v1.Asymmetric:
		if volGroup.Spec.DesiredVolumeSpec.DesiredCount != 0 {
			err = fmt.Errorf("invalid Asymmetric VolumeGroup, DesiredCount is not 0")
			return plugin.Result{
				Error: err,
			}
		}

	default:
		err = fmt.Errorf("invalid SizeSymmetry %s", volGroup.Spec.DesiredVolumeSpec.SizeSymmetry)
		return plugin.Result{
			Error: err,
		}
	}

	// uuid must not be empty
	if volGroup.Spec.Uuid == "" {
		err = fmt.Errorf("invalid VolumeGroup, uuid is empty")
		return plugin.Result{
			Error: err,
		}
	}

	return plugin.Result{}
}

// rollbackUnrecoverable rollback unrecoverable error of Volumes
// unrecoverable error example: no enough free space, node failure, e.g.
func (r *AntstorVolumeGroupReconcileHandler) rollbackUnrecoverable(ctx *plugin.Context, volGroup *v1.AntstorVolumeGroup) (result plugin.Result) {
	var (
		log      = ctx.Log
		err      error
		rollback bool
	)

	if volGroup.Status.Status == v1.VolumeStatusReady {
		log.Info("status is ready, quit rollbackUnrecoverable")
		return plugin.Result{}
	}

	// check status
	for idx, item := range volGroup.Status.VolumeStatus {
		// check volume creation error
		// reasons may be: 1. real free space is not enough. 2. node failure, volume creation cannot happen
		// TODO: if scheduling failed, or node failure
		if strings.Contains(item.Message, filter.NoStoragePoolAvailable) {
			var volId = volGroup.Spec.Volumes[idx]
			// delete volume
			var vol v1.AntstorVolume
			err = r.Client.Get(ctx.ReqCtx.Ctx, client.ObjectKey{
				Namespace: volId.VolId.Namespace,
				Name:      volId.VolId.Name,
			}, &vol)
			if errors.IsNotFound(err) {
				log.Info("not found volume, consider it deleted", "vol", volId)
			} else {
				sinceCreation := time.Since(vol.CreationTimestamp.Time)
				if sinceCreation > time.Minute {
					log.Info("deleting volume", "vol", volId, "sinceCreation", sinceCreation)
					err = r.Client.Delete(ctx.ReqCtx.Ctx, &vol)
					if err != nil {
						log.Error(err, "delete volume failed", "vol", volId)
					}
				} else {
					log.Info("failed to sched volume, wait 1 min to del it", "vol", volId, "sinceCreation", sinceCreation)
					continue
				}
			}

			log.Info("rollback scheduled volume", "volUUID", item.UUID, "msg", item.Message)
			rollback = true
			// action: remove TargetNode, Size of [index], remove according Volume object.
			volGroup.Spec.Volumes[idx].TargetNodeName = ""
			volGroup.Spec.Volumes[idx].Size = 0

			// clear values of VolumeStatus[index]
			volGroup.Status.VolumeStatus[idx].Message = ""
			volGroup.Status.VolumeStatus[idx].Status = ""
		}
	}

	// update volumegroup spec
	if rollback {
		err = r.Client.Update(ctx.ReqCtx.Ctx, volGroup)
		if err != nil {
			return plugin.Result{Error: err}
		}
		return plugin.Result{Break: true}
	}

	return plugin.Result{}
}

func (r *AntstorVolumeGroupReconcileHandler) scheduleVolGroup(ctx *plugin.Context, volGroup *v1.AntstorVolumeGroup) (result plugin.Result) {
	var (
		err          error
		log          = ctx.Log
		scheduler    = r.Scheduler
		volGroupCopy = volGroup.DeepCopy()
	)

	if volGroup.Status.Status == v1.VolumeStatusReady {
		log.Info("status is ready, quit scheduleVolGroup")
		return plugin.Result{}
	}

	ctx.Log.Info("scheduling VolumeGroup", "totalSize", volGroup.Spec.TotalSize)
	err = scheduler.ScheduleVolumeGroup(r.State.GetAllNodes(), volGroup)
	if err != nil {
		// TODO: update status
		log.Error(err, "sched volumegroup failed, retry in 1 min")
		volGroup.Status.Message = err.Error()
		updateErr := r.Status().Update(ctx.ReqCtx.Ctx, volGroup)
		if updateErr != nil {
			log.Error(updateErr, err.Error())
		}
		return plugin.Result{Break: true, Result: ctrl.Result{RequeueAfter: time.Minute}}
	}

	// if volumes are scheduled, persist volumegroup
	if !reflect.DeepEqual(volGroupCopy.Spec.Volumes, volGroup.Spec.Volumes) {
		if !misc.InSliceString(v1.VolumesFinalizer, volGroup.Finalizers) {
			volGroup.Finalizers = append(volGroup.Finalizers, v1.VolumesFinalizer)
		}
		err = r.Client.Update(ctx.ReqCtx.Ctx, volGroup)
		if err != nil {
			log.Error(err, "update volumegroup failed")
			return plugin.Result{Error: err}
		}
		return plugin.Result{Break: true}
	}

	return
}

func (r *AntstorVolumeGroupReconcileHandler) syncVolumes(ctx *plugin.Context, volGroup *v1.AntstorVolumeGroup) (result plugin.Result) {
	// vol group is not scheduled yet
	if len(volGroup.Spec.Volumes) == 0 {
		return plugin.Result{}
	}

	var (
		log           = ctx.Log
		scheduledSize int64
		err           error
		//
		volNotAllReady bool
		//
		copyVolGroup = volGroup.DeepCopy()
	)

	if volGroup.Status.Status == v1.VolumeStatusReady {
		log.Info("status is ready, quit syncVolumes")
		return plugin.Result{}
	}

	for _, item := range volGroup.Spec.Volumes {
		scheduledSize += item.Size
	}

	// sync status of volume group
	// fetch volume status from APIServer, save them to status of VolumeGroup
	volGroup.Status.VolumeStatus = make([]v1.VolumeTargetStatus, len(volGroup.Spec.Volumes))
	for idx, item := range volGroup.Spec.Volumes {
		if item.VolId.UUID != "" && item.VolId.Name != "" && item.VolId.Namespace != "" {
			var vol v1.AntstorVolume
			err = r.Client.Get(context.Background(), client.ObjectKey{
				Namespace: item.VolId.Namespace,
				Name:      item.VolId.Name,
			}, &vol)
			if err != nil {
				// ctx.Log.Error(err, "cannot find volume object, writing volumes may have error", "volName", item.Name)
				// if vol is scheduled and vol is not found, create volume
				if (item.Size > 0 && item.TargetNodeName != "") && errors.IsNotFound(err) {
					newVol := newVolume(volGroup, item.VolId, item.Size, item.TargetNodeName)
					errCreate := r.Client.Create(ctx.ReqCtx.Ctx, newVol)
					if errCreate == nil || errors.IsAlreadyExists(errCreate) {
						log.Info("successfully created volume", "vol", item.VolId)
					} else {
						log.Error(errCreate, "create volume failed", "vol", item.VolId)
					}
				}
				// set status to empty
				volGroup.Status.VolumeStatus[idx].UUID = item.VolId.UUID
				volGroup.Status.VolumeStatus[idx].Status = v1.VolumeStatusCreating
				volGroup.Status.VolumeStatus[idx].Message = ""
				volGroup.Status.VolumeStatus[idx].SpdkTarget = nil

				volNotAllReady = true
			} else {
				volGroup.Status.VolumeStatus[idx] = v1.VolumeTargetStatus{
					UUID:       item.VolId.UUID,
					SpdkTarget: vol.Spec.SpdkTarget,
					Status:     vol.Status.Status,
					Message:    vol.Status.Message,
				}

				if vol.Status.Status != v1.VolumeStatusReady {
					volNotAllReady = true
				}
			}
		}
	}

	if scheduledSize >= volGroup.Spec.TotalSize && !volNotAllReady {
		volGroup.Status.Status = v1.VolumeStatusReady
	} else {
		volGroup.Status.Status = v1.VolumeStatusCreating
	}

	// if status is changed, update status
	if !reflect.DeepEqual(volGroup.Status, copyVolGroup.Status) {
		err = r.Client.Status().Update(ctx.ReqCtx.Ctx, volGroup)
		if err != nil {
			log.Error(err, "update volumegroup status failed")
			return plugin.Result{Error: err}
		}
		log.Info("update volgroup status, requeue after 10 sec", "newStatus", volGroup.Status, "oldStatus", copyVolGroup.Status)
		return plugin.Result{Break: true, Result: ctrl.Result{RequeueAfter: 10 * time.Second}}
	}

	return
}

func (r *AntstorVolumeGroupReconcileHandler) waitVolumesReady(ctx *plugin.Context, volGroup *v1.AntstorVolumeGroup) (result plugin.Result) {
	var (
		log            = ctx.Log
		volNotAllReady bool
	)

	for _, item := range volGroup.Status.VolumeStatus {
		if item.Status != v1.VolumeStatusReady {
			volNotAllReady = true
			break
		}
	}

	if volGroup.Status.Status != v1.VolumeStatusReady || volNotAllReady {
		log.Info("volumes are not all ready, retry in 1 min")
		return plugin.Result{
			Break:  true,
			Result: ctrl.Result{RequeueAfter: time.Minute},
		}
	}

	return
}

func newVolume(volGroup *v1.AntstorVolumeGroup, volId v1.EntityIdentity, volSize int64, volTgtNode string) *v1.AntstorVolume {
	blockOwnerDel := true
	labels := misc.CopyLabel(volGroup.Spec.DesiredVolumeSpec.Labels)
	annos := misc.CopyLabel(volGroup.Spec.DesiredVolumeSpec.Annotations)
	labels[v1.VolumeGroupNameLabelKey] = volGroup.Name

	newVolume := &v1.AntstorVolume{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   volId.Namespace,
			Name:        volId.Name,
			Labels:      labels,
			Annotations: annos,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: v1.GroupVersion.String(),
					Kind:       v1.AntstorVolumeGroupKind,
					Name:       volGroup.Name,
					UID:        volGroup.UID,
					// foregroud delete VolumeGroup will block
					BlockOwnerDeletion: &blockOwnerDel,
				},
			},
		},
		Spec: v1.AntstorVolumeSpec{
			Uuid:     volId.UUID,
			Type:     v1.VolumeTypeFlexible,
			SizeByte: uint64(volSize),
			// set required afifnity to the pool
			PoolAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      v1.PoolLabelsNodeSnKey,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{volTgtNode},
								},
							},
						},
					},
				},
			},
			HostNode: &v1.NodeInfo{
				// TODO: wait data control scheduled?
			},
		},
	}

	// copy Annotations
	for key, val := range volGroup.Annotations {
		if strings.HasPrefix(key, "obnvmf") {
			newVolume.Annotations[key] = val
		}
	}

	return newVolume
}
