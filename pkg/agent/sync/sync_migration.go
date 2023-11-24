package sync

import (
	"context"
	"fmt"
	"time"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/kubeutil"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/generated/clientset/versioned"
	antstorinformers "code.alipay.com/dbplatform/node-disk-controller/pkg/generated/informers/externalversions"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/spdk"
	spdkrpc "code.alipay.com/dbplatform/node-disk-controller/pkg/spdk/jsonrpc/client"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type MigrationReconciler struct {
	nodeID   string
	storeCli versioned.Interface
	spdkCli  spdk.SpdkServiceIface
}

func NewMigrationReconciler(nodeID string, storeCli versioned.Interface, spdkCli spdk.SpdkServiceIface) *MigrationReconciler {
	return &MigrationReconciler{
		nodeID:   nodeID,
		storeCli: storeCli,
		spdkCli:  spdkCli,
	}
}

func (r *MigrationReconciler) Start(ctx context.Context) (err error) {
	informerFactory := antstorinformers.NewFilteredSharedInformerFactory(r.storeCli, time.Hour, v1.DefaultNamespace, func(lo *metav1.ListOptions) {
		lo.LabelSelector = fmt.Sprintf("%s=%s", v1.MigrationLabelKeySourceNodeId, r.nodeID)
	})

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	informer := informerFactory.Volume().V1().VolumeMigrations().Informer()
	informer.AddEventHandler(kubeutil.CommonResourceEventHandlerFuncs(queue))

	go informer.Run(ctx.Done())
	// or informerFactory.Start(spm.quitChan)

	kubeutil.NewSimpleController("agent-migration-loop", queue, r).Start(ctx)
	return
}

func (r *MigrationReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var (
		ns           = req.Namespace
		name         = req.Name
		err          error
		migration    *v1.VolumeMigration
		migrationCli = r.storeCli.VolumeV1().VolumeMigrations(ns)
	)

	migration, err = migrationCli.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	if migration.DeletionTimestamp != nil {
		klog.Error("TODO: not support delete migration ", req)
		return reconcile.Result{}, nil
	}

	// Step-1: Destination Volume must be ready
	var (
		srcVolume  *v1.AntstorVolume
		destVolume *v1.AntstorVolume
		volCli     = r.storeCli.VolumeV1().AntstorVolumes(migration.Spec.DestVolume.Namespace)
	)

	if migration.Spec.DestVolume.Name == "" || migration.Spec.DestVolume.Namespace == "" {
		klog.Warningf("Migration %v, dest volume is empty. Retry in 1 min", req)
		return reconcile.Result{RequeueAfter: time.Minute}, nil
	}

	srcVolume, err = volCli.Get(ctx, migration.Spec.SourceVolume.Name, metav1.GetOptions{})
	if err != nil {
		return reconcile.Result{}, err
	}

	destVolume, err = volCli.Get(ctx, migration.Spec.DestVolume.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{RequeueAfter: time.Minute}, nil
		}
		return reconcile.Result{}, err
	}
	// must be ready
	if destVolume.Status.Status != v1.VolumeStatusReady {
		klog.Infof("Migration %v, destVolume type is not ready. Retry in 30s", req)
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}
	// type must be SpdkLvol
	if destVolume.Spec.Type != v1.VolumeTypeSpdkLVol {
		klog.Errorf("Migration %v, destVolume type is not SpdkLvol", req)
		return reconcile.Result{}, nil
	}

	// TODO: Setp-2: setup sync pipe
	if migration.Status.Phase == v1.MigrationPhaseSetupPipe &&
		migration.Spec.MigrationInfo.MigrationPipe.DestBdevName == "" {
		// do attach dest target
		var controllerName = destVolume.Name
		err = r.spdkCli.EnsureMigrationDestBdev(spdk.AttachDestBdevRequest{
			ControllerName: controllerName,
			Target: spdk.SpdkTargetInfo{
				NQN:       destVolume.Spec.SpdkTarget.SubsysNQN,
				AddrFam:   string(spdkrpc.AddrFamilyIPv4),
				IPAddr:    destVolume.Spec.SpdkTarget.Address,
				TransType: spdkrpc.TransportTypeTCP,
				SvcID:     destVolume.Spec.SpdkTarget.SvcID,
			},
		})
		if err != nil {
			klog.Error(err)
			return reconcile.Result{}, err
		}

		migration.Spec.MigrationInfo.MigrationPipe = v1.MigrationPipe{
			/*
				连接目标端
				./rpc.py bdev_nvme_attach_controller -b mig-a4fec53b-0220-4944-a4ac-fa6711c95e8d -t tcp -a 100.100.100.1 -f IPv4 -s 20002 -n nqn.2021-03.xuhai:test
				注意: 返回的 bdev 的名称是 mig-a4fec53b-0220-4944-a4ac-fa6711c95e8dn1, 多了一个n1后缀
			*/
			DestBdevName: controllerName + "n1",
			Status:       v1.ConnectStatusConnected,
		}
		migration.Finalizers = append(migration.Finalizers, v1.MigrationFinalizerPipeConnected)
		_, err = migrationCli.Update(ctx, migration, metav1.UpdateOptions{})
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	// Step-3: wait Host connected.
	if migration.Status.Phase == v1.MigrationPhaseSetupPipe &&
		migration.Spec.MigrationInfo.HostConnectStatus != v1.ConnectStatusConnected {
		klog.Infof("Migration %v, host not connected. Retry in 30s", req)
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Step-4: start migration
	// TODO: check migration status;
	if migration.Status.Phase == v1.MigrationPhaseSyncing {
		// Read/Update job status or Create job
		var (
			srcBdevName      = srcVolume.Spec.SpdkLvol.FullName()
			dstBdevName      = migration.Spec.MigrationInfo.MigrationPipe.DestBdevName
			tasks            []spdk.MigrateTask
			patch            = client.MergeFrom(migration.DeepCopy())
			patchData        []byte
			jobElapsedTimeMs = migration.Spec.MigrationInfo.JobProgress.ElapsedTimeMs
		)

		tasks, err = r.spdkCli.GetMigrationTask(spdk.GetMigrationTaskRequest{
			SrcBdev: srcBdevName,
		})
		if err != nil {
			klog.Error(err)
			return reconcile.Result{}, err
		}

		if len(tasks) == 0 {
			// start migration
			err = r.spdkCli.StartMigrationTask(spdk.MigrateStartRequest{
				SrcBdev: srcBdevName,
				DstBdev: dstBdevName,
			})
			if err != nil {
				klog.Error(err)
			}
			return reconcile.Result{RequeueAfter: 10 * time.Second}, err
		} else {
			// update migration task status
			task := tasks[0]
			migration.Spec.MigrationInfo.JobProgress = v1.JobProgress{
				SrcBdev:         srcBdevName,
				DstBdev:         dstBdevName,
				Status:          task.Status,
				IsLastRound:     task.WorkingRound.IsLastRound,
				TotalWritePages: task.TotalWritePages,
				TotalReadPages:  task.TotalReadPages,
				RoundPassed:     task.RoundPassed,
				ElapsedTimeMs:   task.TimeElapsedMS,
			}
			// if time since last reconcling is less than 10s, then wait for 30s.
			if task.Status != spdkrpc.MigrateTaskStatusCompleted && task.TimeElapsedMS-jobElapsedTimeMs < 10*1000 {
				klog.Infof("reconciling too often: %dms ; wait 30s for next reconciling", task.TimeElapsedMS-jobElapsedTimeMs)
				return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
			}

			// enable autoswitch
			if !migration.Spec.MigrationInfo.AutoSwitch.Enabled {
				// Read job status, if job is about to complete
				if migration.Spec.MigrationInfo.JobProgress.RoundPassed > 0 {
					// do enable autoswitch
					err = r.spdkCli.SetMigrationConfig(spdk.MigrateConfigRequest{
						SrcBdev:    srcBdevName,
						DstBdev:    dstBdevName,
						AutoSwitch: true,
					})
					if err != nil {
						klog.Error(err)
						return reconcile.Result{}, err
					}

					migration.Spec.MigrationInfo.AutoSwitch.Enabled = true
					migration.Spec.MigrationInfo.AutoSwitch.Status = v1.ResultStatusUnknown
				} else {
					klog.Infof("migration is not about to complete, wait 15s")
					return reconcile.Result{RequeueAfter: 15 * time.Second}, nil
				}
			}

			// update migration
			patchData, err = patch.Data(migration)
			if err != nil {
				klog.Error(err)
				return reconcile.Result{}, err
			}
			_, err = migrationCli.Patch(ctx, name, types.MergePatchType, patchData, metav1.PatchOptions{})
			if err != nil {
				klog.Error(err)
				return reconcile.Result{}, err
			}
			return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	// Step-5: clean job and pipe
	if migration.Status.Phase == v1.MigrationPhaseCleaning {
		if migration.Spec.MigrationInfo.MigrationPipe.Status == v1.ConnectStatusConnected {
			var (
				srcBdevName    = srcVolume.Spec.SpdkLvol.FullName()
				dstBdevName    = migration.Spec.MigrationInfo.MigrationPipe.DestBdevName
				controllerName = destVolume.Name
			)

			// delete job, should be idempotent
			err = r.spdkCli.CleanMigrationTask(spdk.MigrateStartRequest{
				SrcBdev: srcBdevName,
				DstBdev: dstBdevName,
			})
			if err != nil {
				klog.Error(err)
				return reconcile.Result{}, err
			}

			// detach dstBdev, should be idempotent
			err = r.spdkCli.DetachMigrationDestBdev(controllerName)
			if err != nil {
				klog.Error(err)
				return reconcile.Result{}, err
			}

			migration.Spec.MigrationInfo.MigrationPipe.Status = v1.ConnectStatusDisconnected
			// remove MigrationFinalizerPipeConnected
			var newFinalizers []string
			for _, item := range migration.Finalizers {
				if item != v1.MigrationFinalizerPipeConnected {
					newFinalizers = append(newFinalizers, item)
				}
			}
			migration.Finalizers = newFinalizers
			_, err = migrationCli.Update(ctx, migration, metav1.UpdateOptions{})
			if err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	return reconcile.Result{}, nil
}
