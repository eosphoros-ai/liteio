package reconciler

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"github.com/go-logr/logr"
	uuid "github.com/satori/go.uuid"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

type VolumeMigrationReconciler struct {
	client.Client
	Log logr.Logger
}

func (r *VolumeMigrationReconciler) SetupWithManager(mgr ctrl.Manager) (err error) {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		For(&v1.VolumeMigration{}).
		Complete(r)
}

func (r *VolumeMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		err error
		log = r.Log.WithValues("Migration", req.NamespacedName)
		// migration obj
		migration v1.VolumeMigration
		// source volume
		srcVol v1.AntstorVolume
	)

	if err := r.Get(ctx, req.NamespacedName, &migration); err != nil {
		// When user deleted a volume, a request will be recieved.
		// However the volume does not exists. Therefore the code goes to here
		log.Error(err, "unable to fetch Migration")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Start handling Migration")

	// not handle delete request
	if migration.DeletionTimestamp != nil {
		log.Info("deleting Migration")
		// TODO:
		return ctrl.Result{}, nil
	}

	if migration.Status.Status == v1.MigrationStatusError {
		log.Info("status is error, not reconcile")
		return ctrl.Result{}, nil
	}

	if migration.Status.Phase == "" {
		migration.Status.Phase = v1.MigrationPhasePending
		migration.Status.Status = v1.MigrationStatusWorking
		return ctrl.Result{}, r.Status().Update(ctx, &migration)
	}

	// Phase 0: validate source volume, volume must be ready. type must be spdk
	err = r.Get(ctx, client.ObjectKey{
		Namespace: migration.Spec.SourceVolume.Namespace,
		Name:      migration.Spec.SourceVolume.Name,
	}, &srcVol)
	if err != nil {
		migration.Status.Message = fmt.Sprintf("invalid source volume %+v", err)
		migration.Status.Status = v1.MigrationStatusError
		log.Info("update migration status", "err", r.Status().Update(ctx, &migration))
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if srcVol.Spec.Type != v1.VolumeTypeSpdkLVol {
		migration.Status.Message = fmt.Sprintf("invalid source volume: type is %+s", srcVol.Spec.Type)
		migration.Status.Status = v1.MigrationStatusError
		log.Info("update migration status", "err", r.Status().Update(ctx, &migration))
		return ctrl.Result{}, nil
	}

	if srcVol.Spec.PositionAdvice == v1.MustLocal {
		migration.Status.Message = fmt.Sprintf("invalid source volume: postion advice is %+s", srcVol.Spec.PositionAdvice)
		migration.Status.Status = v1.MigrationStatusError
		log.Info("update migration status", "err", r.Status().Update(ctx, &migration))
		return ctrl.Result{}, nil
	}

	if srcVol.Status.Status != v1.VolumeStatusReady {
		migration.Status.Message = fmt.Sprintf("invalid source volume: status not ready %+v", srcVol.Status)
		log.Info("update migration status", "err", r.Status().Update(ctx, &migration))
		return ctrl.Result{}, nil
	}

	// fill out info of source volume
	if migration.Spec.SourceVolume.TargetNodeId == "" ||
		migration.Spec.SourceVolume.Spdk.SubsysNQN == "" ||
		migration.Spec.SourceVolume.HostNodeId == "" {
		migration.Spec.SourceVolume.TargetNodeId = srcVol.Spec.TargetNodeId
		migration.Spec.SourceVolume.HostNodeId = srcVol.Spec.HostNode.ID
		migration.Spec.SourceVolume.Spdk = *srcVol.Spec.SpdkTarget
		return ctrl.Result{}, r.Update(ctx, &migration)
	}

	// add labels to Migration
	if migration.Labels == nil {
		migration.Labels = make(map[string]string)
	}
	if migration.Labels[v1.MigrationLabelKeySourceNodeId] == "" {
		migration.Labels[v1.MigrationLabelKeySourceNodeId] = srcVol.Spec.TargetNodeId
		migration.Labels[v1.MigrationLabelKeyHostNodeId] = srcVol.Spec.HostNode.ID
		err = r.Update(ctx, &migration)
		return ctrl.Result{}, err
	}

	// Phase 1: Create destination volume.
	if migration.Status.Phase == v1.MigrationPhasePending {
		migration.Status.Phase = v1.MigrationPhaseCreatingVolume
		migration.Status.Status = v1.MigrationStatusWorking
		log.Info("update Phase to CreatingVolume")
		err = r.Status().Update(ctx, &migration)
		return ctrl.Result{}, err
	}
	// determine name of dest volume
	if migration.Spec.DestVolume.Name == "" {
		migration.Spec.DestVolume.Namespace = srcVol.Namespace
		migration.Spec.DestVolume.Name = "mig-" + uuid.NewV4().String()
		migration.Spec.DestVolume.HostNodeId = srcVol.Spec.HostNode.ID
		log.Info("save dest volume name", "name", migration.Spec.DestVolume.Name)

		err = r.Update(ctx, &migration)
		return ctrl.Result{}, err
	}

	// get dest volume
	var destVolume v1.AntstorVolume
	err = r.Get(ctx, client.ObjectKey{
		Namespace: migration.Spec.DestVolume.Namespace,
		Name:      migration.Spec.DestVolume.Name,
	}, &destVolume)
	if client.IgnoreNotFound(err) != nil {
		log.Error(err, "get destVolume failed")
		return ctrl.Result{}, err
	}

	if errors.IsNotFound(err) {
		// create
		destVolume = v1.AntstorVolume{
			// ObjectMeta: *srcVol.ObjectMeta.DeepCopy(),
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   migration.Spec.DestVolume.Namespace,
				Name:        migration.Spec.DestVolume.Name,
				Labels:      srcVol.ObjectMeta.Labels,
				Annotations: srcVol.ObjectMeta.Annotations,
			},
			Spec: v1.AntstorVolumeSpec{
				Uuid:           strings.TrimPrefix(migration.Spec.DestVolume.Name, "mig-"),
				Type:           srcVol.Spec.Type,
				SizeByte:       srcVol.Spec.SizeByte,
				PositionAdvice: srcVol.Spec.PositionAdvice,
				HostNode:       srcVol.Spec.HostNode.DeepCopy(),
				SpdkTarget: &v1.SpdkTarget{
					SubsysNQN: srcVol.Spec.SpdkTarget.SubsysNQN,
					NSUUID:    srcVol.Spec.SpdkTarget.NSUUID,
				},
			},
		}
		if destVolume.Annotations == nil {
			destVolume.Annotations = make(map[string]string)
		}
		if destVolume.Labels == nil {
			destVolume.Labels = make(map[string]string)
		}
		// remove label obnvmf/target-node-id
		delete(destVolume.Labels, v1.TargetNodeIdLabelKey)
		// NOTICE: Labels["uuid"] is same with srcVol on purpose. So the CSI controller can find dest volume by old uuid.
		destVolume.Name = migration.Spec.DestVolume.Name
		destVolume.Labels[v1.MigrationLabelKeySourceVolumeName] = srcVol.Name
		destVolume.Labels[v1.MigrationLabelKeyMigrationName] = migration.Name
		// dest volume cannot reside on the same node of src volume, b/c dest subsystem should have same NQN, NSUUID of src subsystem.
		destVolume.Annotations[v1.PoolLabelSelectorKey] = fmt.Sprintf("%s!=%s", v1.PoolLabelsNodeSnKey, srcVol.Spec.TargetNodeId)

		log.Info("creating dest volume", "name", destVolume.Name)
		err = r.Create(ctx, &destVolume)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	// wait status to be ready
	if destVolume.Status.Status != v1.VolumeStatusReady {
		log.Info("dest volume is not ready, wait 30s")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	if migration.Spec.DestVolume.TargetNodeId == "" || migration.Spec.DestVolume.Spdk.Address == "" {
		log.Info("dest volume is ready, save info to migration spec", "TargetNodeId", destVolume.Spec.TargetNodeId)
		migration.Spec.DestVolume.TargetNodeId = destVolume.Spec.TargetNodeId
		migration.Spec.DestVolume.Spdk = *destVolume.Spec.SpdkTarget

		err = r.Update(ctx, &migration)
		return ctrl.Result{}, err
	}

	// Phase 2: Setup syncing pipe. Host connects destination volume.
	// MigrationPipe is setup by node-disk-agent on srouce node
	// HostConnectStatus is set by host-nvme-mgr on host node
	if migration.Status.Phase == v1.MigrationPhaseCreatingVolume {
		migration.Status.Phase = v1.MigrationPhaseSetupPipe
		err = r.Status().Update(ctx, &migration)
		return ctrl.Result{}, err
	}

	if migration.Status.Phase == v1.MigrationPhaseSetupPipe {
		if migration.Spec.MigrationInfo.MigrationPipe.Status != v1.ConnectStatusConnected {
			log.Info("migration pipe is not ready, wait 30s")
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		if migration.Spec.MigrationInfo.HostConnectStatus != v1.ConnectStatusConnected {
			log.Info("host connection is not ready, wait 30s")
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	// Phase 3: Start migration job. Wait autoswitch to be success
	if migration.Status.Phase == v1.MigrationPhaseSetupPipe {
		migration.Status.Phase = v1.MigrationPhaseSyncing
		err = r.Status().Update(ctx, &migration)
		return ctrl.Result{}, err
	}

	if migration.Status.Phase == v1.MigrationPhaseSyncing {
		if migration.Spec.MigrationInfo.AutoSwitch.Status != v1.ResultStatusSuccess {
			log.Info("autoswitch is not success, wait 60s")
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}
	}

	// Phase 4: Clean job and connection.
	if migration.Status.Phase == v1.MigrationPhaseSyncing {
		migration.Status.Phase = v1.MigrationPhaseCleaning
		err = r.Status().Update(ctx, &migration)
		return ctrl.Result{}, err
	}
	if migration.Status.Phase == v1.MigrationPhaseCleaning {
		if migration.Spec.MigrationInfo.MigrationPipe.Status != v1.ConnectStatusDisconnected {
			log.Info("migration pipe is not disconnected, wait 30s")
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		if migration.Spec.MigrationInfo.HostConnectStatus != v1.ConnectStatusDisconnected {
			log.Info("host connection is not disconnected, wait 30s")
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	// Phase 5: Finish
	if migration.Status.Phase == v1.MigrationPhaseCleaning {
		migration.Status.Phase = v1.MigrationPhaseFinished
		migration.Status.Status = string(v1.MigrationPhaseFinished)
		err = r.Status().Update(ctx, &migration)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
