package sync

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"code.alipay.com/dbplatform/node-disk-controller/pkg/agent/pool"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/agent/pool/engine"
	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/generated/clientset/versioned"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/misc"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type SnapshotSyncer struct {
	poolService pool.StoragePoolServiceIface
	// storeCli is used to read/write StoragePool, AntstorVolumes from APIServer
	storeCli versioned.Interface
}

func NewSnapshotSyncer(storeCli versioned.Interface, poolSvc pool.StoragePoolServiceIface) *SnapshotSyncer {
	return &SnapshotSyncer{
		poolService: poolSvc,
		storeCli:    storeCli,
	}
}

func (ss *SnapshotSyncer) Start(ctx context.Context) (err error) {
	poolName := ss.poolService.GetStoragePool().GetName()
	snapListWatcher := cache.NewFilteredListWatchFromClient(ss.storeCli.VolumeV1().RESTClient(), "antstorsnapshots",
		v1.DefaultNamespace, func(options *metav1.ListOptions) {
			options.LabelSelector = fmt.Sprintf("%s=%s", v1.TargetNodeIdLabelKey, poolName)
		})

	snapSyncLoop := NewSyncLoop("SnapshotLoop", snapListWatcher, &v1.AntstorSnapshot{}, func(name string) (err error) {
		return ss.syncOneSnapshot(name)
	})
	snapSyncLoop.RunLoop(ctx.Done())
	return
}

func (ss *SnapshotSyncer) syncOneSnapshot(nsName string) (err error) {
	ns, name, err := cache.SplitMetaNamespaceKey(nsName)
	if err != nil {
		return
	}
	snapCli := ss.storeCli.VolumeV1().AntstorSnapshots(ns)
	snapshot, err := snapCli.Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		// snapshot is already deleted, ignore not-found error
		if errors.IsNotFound(err) {
			return nil
		}
		return
	}

	// to delete snapshot
	if snapshot.DeletionTimestamp != nil {
		klog.Infof("deleting snapshot %s", name)

		// TODO: recondier snapshot deletion constraint

		if misc.InSliceString(v1.SnapshotFinalizer, snapshot.Finalizers) {
			klog.Infof("deleting snapshot %s lvol, vol type %s", name, string(snapshot.Spec.VolType))
			var volName string
			switch snapshot.Spec.VolType {
			case v1.VolumeTypeKernelLVol:
				volName = snapshot.Spec.KernelLvol.Name
			case v1.VolumeTypeSpdkLVol:
				volName = snapshot.Spec.SpdkLvol.Name
			}

			err = ss.poolService.PoolEngine().DeleteVolume(volName)
			if err != nil {
				klog.Error(err)
				return
			}
			// remove v1.SnapshotFinalizer
			var newFinalizers = make([]string, 0, len(snapshot.Finalizers))
			for _, item := range snapshot.Finalizers {
				if item != v1.SnapshotFinalizer {
					newFinalizers = append(newFinalizers, item)
				}
			}
			snapshot.Finalizers = newFinalizers
			_, err = snapCli.Update(context.Background(), snapshot, metav1.UpdateOptions{})
			if err != nil {
				klog.Error(err)
			}
			return
		}

		return
	}

	klog.Infof("start syncing snapshot %s lvol", name)
	if snapshot.Status.Status == v1.SnapshotStatusReady || snapshot.Status.Status == v1.SnapshotStatusMerged {
		klog.Infof("snapshot %s is already Ready, stop syncing", name)
		return
	}

	// create snapshot lvol
	if snapshot.Status.Status == v1.SnapshotStatusCreating {
		klog.Infof("create snapshot %s lvol, vol type %s", name, snapshot.Spec.VolType)

		// if Finalizers is added, but status is not updated
		if misc.InSliceString(v1.SnapshotFinalizer, snapshot.Finalizers) {
			klog.Infof("update snapshot %s to ready", name)
			snapshot.Status.Status = v1.SnapshotStatusReady
			_, err = snapCli.UpdateStatus(context.Background(), snapshot, metav1.UpdateOptions{})
			if err != nil {
				klog.Error(err)
			}
			return
		}

		// do create
		var originName, snapName string
		var sp = ss.poolService.GetStoragePool()
		if ss.poolService.Mode() == v1.PoolModeKernelLVM {
			snapName = fmt.Sprintf("%s_snap", snapshot.Spec.OriginVolName)
			vgName := sp.Spec.KernelLVM.Name
			originName = snapshot.Spec.OriginVolName

			snapshot.Spec.KernelLvol.Name = snapName
			snapshot.Spec.KernelLvol.DevPath = fmt.Sprintf("/dev/%s/%s", vgName, snapName)
			snapshot.Spec.VolType = v1.VolumeTypeKernelLVol

		} else if ss.poolService.Mode() == v1.PoolModeSpdkLVStore {
			lvsName := sp.Spec.SpdkLVStore.Name
			snapName = fmt.Sprintf("%s_snap", snapshot.Spec.OriginVolName)
			originName = fmt.Sprintf("%s/%s", lvsName, snapshot.Spec.OriginVolName)

			snapshot.Spec.SpdkLvol.Name = snapName
			snapshot.Spec.SpdkLvol.LvsName = lvsName
			snapshot.Spec.VolType = v1.VolumeTypeSpdkLVol
		}

		klog.Infof("create snapshot, originName %s, snapshotName %s, size %d", originName, snapName, snapshot.Spec.Size)
		err = ss.poolService.PoolEngine().CreateSnapshot(engine.CreateSnapshotRequest{
			SnapshotName: snapName,
			OriginName:   originName,
			SizeByte:     uint64(snapshot.Spec.Size),
		})
		if err != nil {
			klog.Error(err)
			return
		}

		// update Finalizer and Spec.KernelLVM
		snapshot.Finalizers = append(snapshot.Finalizers, v1.SnapshotFinalizer)
		_, err = snapCli.Update(context.Background(), snapshot, metav1.UpdateOptions{})
		if err != nil {
			klog.Error(err)
		}
		return
	}

	// merge snapshot lvol
	if snapshot.Spec.VolType == v1.VolumeTypeKernelLVol && snapshot.Status.Status == v1.SnapshotStatusMerging {
		klog.Info("start merging snapshot")

		// merge is finished
		if _, has := snapshot.Labels[v1.MergeFinishTimestampLabelKey]; has {
			snapshot.Status.Status = v1.SnapshotStatusMerged
			_, err = snapCli.UpdateStatus(context.Background(), snapshot, metav1.UpdateOptions{})
			if err != nil {
				klog.Error(err)
			}
			return
		}

		err = ss.poolService.PoolEngine().RestoreSnapshot(snapshot.Spec.KernelLvol.Name)
		if err != nil {
			klog.Error(err)
			return
		}

		if _, has := snapshot.Labels[v1.MergeFinishTimestampLabelKey]; !has {
			snapshot.Labels[v1.MergeFinishTimestampLabelKey] = strconv.Itoa(int(time.Now().Unix()))
			snapshot.Status.Status = v1.SnapshotStatusMerged

			_, err = snapCli.Update(context.Background(), snapshot, metav1.UpdateOptions{})
			if err != nil {
				klog.Error(err)
			}
			return
		}

		return
	}

	return
}
