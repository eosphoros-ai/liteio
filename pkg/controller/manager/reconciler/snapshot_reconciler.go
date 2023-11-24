package reconciler

import (
	"context"
	"fmt"
	"strconv"
	"time"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/generated/clientset/versioned"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

const (
	SnapshotCreateFailure = "SnapshotCreateFailure"
	SnapshotMergeFailure  = "SnapshotMergeFailure"
	SnapshotDeleteFailure = "SnapshotDeleteFailure"

	fourMiB int64 = 1 << 22
)

type SnapshotReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	// AntstorClientset r/w AntstorSnapshot
	AntstorClientset versioned.Interface
	// EventRecorder
	EventRecorder record.EventRecorder
}

// SetupWithManager sets up the controller with the Manager.
func (r *SnapshotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		For(&v1.AntstorSnapshot{}).
		Complete(r)
}

func (r *SnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var log = r.Log.WithValues("SnapshotReconciler", req.NamespacedName)
	var obj v1.AntstorSnapshot
	var originVol v1.AntstorVolume
	var err error

	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		// When user deleted a snapshot, a request will be recieved.
		// However the snapshot does not exists. Therefore the code goes to here
		log.Error(err, "unable to fetch Snapshot")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("start handling Snapshot", "status", obj.Status)

	// handle delete request
	if obj.DeletionTimestamp != nil {
		log.Info("start handling snapshot deletion")
		// if merged, delete it
		if obj.Status.Status == v1.SnapshotStatusMerged {
			log.Info("snapshot is merged, so remove all Finalizers")
			obj.Finalizers = []string{}
			err = r.Update(context.Background(), &obj)
			// TODO if Snapshot is created by CreateSnapshot method, we should also delete VolumeSnapshot create by k8s
			// because merge operation will delete AntstorSnapshot
			// var volumeSnapshot VolumeSnapshot
			// volumeSnapshotName := obj.Labels["volume-snapshot-name"]
			// err = r.Get(ctx, client.ObjectKey{})
			// err = r.Delete(ctx, volumeSnapshot)
			return ctrl.Result{}, err
		}

		// get storage pool
		nodeID := obj.Spec.OriginVolTargetNodeID
		var targetPool v1.StoragePool
		err = r.Client.Get(ctx, client.ObjectKey{
			Namespace: v1.DefaultNamespace,
			Name:      nodeID,
		}, &targetPool)
		// error is not NotFound, throw the error out
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "get StoragePool failed", "nodeid", nodeID)
			r.EventRecorder.Event(&obj, corev1.EventTypeWarning, SnapshotDeleteFailure, err.Error())
			return ctrl.Result{}, err
		}

		// targetPool is not found. It was deleted from cluster, so remove the volume right away.
		if targetPool.Name == "" {
			obj.Finalizers = []string{}
			err = r.Update(context.Background(), &obj)
			return ctrl.Result{}, err
		}
	}

	if obj.Status.Status == v1.SnapshotStatusMerged {
		log.Info("snapshot is already merged, so ignore it")
		return ctrl.Result{}, nil
	}

	if len(obj.Labels) == 0 {
		log.Info("add labels to snapshot")
		if obj.Labels == nil {
			obj.Labels = make(map[string]string)
		}
		obj.Labels[v1.OriginVolumeNameLabelKey] = obj.Spec.OriginVolName
		obj.Labels[v1.OriginVolumeNamespaceLabelKey] = obj.Spec.OriginVolNamespace
		err = r.Update(context.Background(), &obj)
		return ctrl.Result{}, err
	}

	// TODO: validate Snapshot
	// 1. size align to 4MiB
	if obj.Spec.Size < fourMiB {
		r.EventRecorder.Event(&obj, corev1.EventTypeWarning, SnapshotCreateFailure, "size too small, at least 4MiB")
		return ctrl.Result{}, nil
	}
	if remainder := obj.Spec.Size % fourMiB; remainder > 0 {
		obj.Spec.Size = (obj.Spec.Size / fourMiB) * fourMiB
		err = r.Update(context.Background(), &obj)
		return ctrl.Result{}, err
	}

	// 2. The origin volume should only have one snapshot
	snapList, err := r.AntstorClientset.VolumeV1().AntstorSnapshots(obj.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", v1.OriginVolumeNameLabelKey, obj.Spec.OriginVolName),
	})
	if err != nil {
		log.Error(err, "list by OriginVolName failed")
		return ctrl.Result{}, err
	}
	for _, item := range snapList.Items {
		// skip itself
		if item.Name == obj.Name && item.Namespace == obj.Namespace {
			continue
		}
		if item.Status.Status != v1.SnapshotStatusMerged {
			log.Info("origin volume can only have one snapshot", "originVolName", obj.Spec.OriginVolName)
			r.EventRecorder.Event(&obj, corev1.EventTypeWarning, SnapshotCreateFailure, "origin volume can only have one snapshot")
			return ctrl.Result{Requeue: true, RequeueAfter: 3 * time.Minute}, nil
		}
	}

	// get origin volume
	volFullName := types.NamespacedName{
		Namespace: obj.Spec.OriginVolNamespace,
		Name:      obj.Spec.OriginVolName,
	}
	log.Info("get origin volume", "volFullName", volFullName)
	err = r.Get(ctx, volFullName, &originVol)
	if err != nil {
		if errors.IsNotFound(err) {
			// TODO: submit Event, update Status to error
			log.Info("not found StoragePool", "name", volFullName)
			r.EventRecorder.Event(&obj, corev1.EventTypeWarning, SnapshotCreateFailure, fmt.Sprintf("cannot find origin volume %s", volFullName))
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// TODO: validate origin volume
	if originVol.Spec.TargetNodeId == "" {
		log.Info("origin vol is not scheduled. Snapshot creation will be retried in 1 min", "volName", volFullName)
		r.EventRecorder.Event(&obj, corev1.EventTypeWarning, SnapshotCreateFailure, fmt.Sprintf("origin volume %s is not scheduled", volFullName))
		return ctrl.Result{Requeue: true, RequeueAfter: time.Minute}, nil
	}

	if originVol.Annotations == nil {
		r.EventRecorder.Event(&obj, corev1.EventTypeWarning, SnapshotCreateFailure, "origin volume has no obnvmf/snapshot-reserved-bytes in annotations")
		return ctrl.Result{Requeue: true, RequeueAfter: 3 * time.Minute}, nil
	} else {
		snapReservedSpace := originVol.Annotations[v1.SnapshotReservedSpaceAnnotationKey]
		snapReservedBytes, err := strconv.Atoi(snapReservedSpace)
		if err != nil {
			r.EventRecorder.Event(&obj, corev1.EventTypeWarning, SnapshotCreateFailure, fmt.Sprintf("invalid reserved space %s, err %+v", snapReservedSpace, err))
			return ctrl.Result{Requeue: true, RequeueAfter: 3 * time.Minute}, nil
		}

		if obj.Spec.Size > int64(snapReservedBytes) {
			r.EventRecorder.Event(&obj, corev1.EventTypeWarning, SnapshotCreateFailure, fmt.Sprintf("snap size too large: %d, reserved size %d", obj.Spec.Size, snapReservedBytes))
			return ctrl.Result{Requeue: true, RequeueAfter: 3 * time.Minute}, nil
		}
	}

	// bind snapshot to Node. update snapshot target id
	if obj.Spec.OriginVolTargetNodeID == "" {
		log.Info("update origin vol node id", "nodeID", originVol.Spec.TargetNodeId)
		obj.Spec.OriginVolTargetNodeID = originVol.Spec.TargetNodeId
		obj.Labels[v1.TargetNodeIdLabelKey] = originVol.Spec.TargetNodeId
		err = r.Update(context.Background(), &obj)
		return ctrl.Result{}, err
	}

	// update status to creating
	if obj.Status.Status == "" && obj.Spec.OriginVolTargetNodeID != "" {
		log.Info("update status to creating")
		obj.Status.Status = v1.SnapshotStatusCreating
		err = r.Status().Update(context.Background(), &obj)
		return ctrl.Result{}, err
	}

	// update status to merging
	if val, has := obj.Labels[v1.MergeStartTimestampLabelKey]; has && val != "" && obj.Status.Status != v1.SnapshotStatusMerging {
		log.Info("update status to merging")
		obj.Status.Status = v1.SnapshotStatusMerging
		err = r.Status().Update(context.Background(), &obj)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
