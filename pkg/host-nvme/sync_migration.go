package hostnvme

import (
	"context"
	"fmt"
	"time"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/kubeutil"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/generated/clientset/versioned"
	antstorinformers "code.alipay.com/dbplatform/node-disk-controller/pkg/generated/informers/externalversions"
	spdkclient "code.alipay.com/dbplatform/node-disk-controller/pkg/spdk/jsonrpc/client"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/spdk/jsonrpc/nvme"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	rt "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	stupaNvmePath = "/home/admin/nvmeof/bin/nvme"
)

type HostNvmeManager struct {
	nodeID   string
	storeCli versioned.Interface
}

func (spm *HostNvmeManager) SyncMigrationLoop(ctx context.Context) {
	informerFactory := antstorinformers.NewFilteredSharedInformerFactory(spm.storeCli, time.Hour, v1.DefaultNamespace, func(lo *metav1.ListOptions) {
		lo.LabelSelector = fmt.Sprintf("%s=%s", v1.MigrationLabelKeyHostNodeId, spm.nodeID)
	})

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	informer := informerFactory.Volume().V1().VolumeMigrations().Informer()
	informer.AddEventHandler(kubeutil.CommonResourceEventHandlerFuncs(queue))

	go informer.Run(ctx.Done())
	// or informerFactory.Start(spm.quitChan)

	kubeutil.NewSimpleController("hostnvme-migration-loop", queue, &MigrationReconciler{
		storeCli: spm.storeCli,
	}).Start(ctx)
}

type MigrationReconciler struct {
	storeCli versioned.Interface
}

func (r *MigrationReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var (
		ns           = req.Namespace
		name         = req.Name
		err          error
		migration    *v1.VolumeMigration
		migrationCli = r.storeCli.VolumeV1().VolumeMigrations(ns)
		nvmeCli      = nvme.NewClientWithCmdPath(stupaNvmePath)
		log          = rt.Log.WithName("hostnvme").WithValues("migration", req)
	)

	log.Info("handle migration")

	migration, err = migrationCli.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	if migration.DeletionTimestamp != nil {
		log.Info("TODO: not support delete migration ")
		return reconcile.Result{}, nil
	}

	// Step-1: Destination Volume must be ready
	var (
		destVolume *v1.AntstorVolume
		srcVolume  *v1.AntstorVolume
		volCli     = r.storeCli.VolumeV1().AntstorVolumes(migration.Spec.DestVolume.Namespace)
	)
	if migration.Spec.DestVolume.Name == "" || migration.Spec.DestVolume.Namespace == "" {
		klog.Warningf("Migration %v, dest volume is empty. Retry in 1 min", req)
		return reconcile.Result{RequeueAfter: time.Minute}, nil
	}

	srcVolume, err = volCli.Get(ctx, migration.Spec.DestVolume.Name, metav1.GetOptions{})
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
		log.Info("destVolume type is not ready. Retry in 30s")
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}
	// type must be SpdkLvol
	if destVolume.Spec.Type != v1.VolumeTypeSpdkLVol {
		log.Info("destVolume type is not SpdkLvol")
		return reconcile.Result{}, nil
	}

	// Setp-2: Host connect Target
	if migration.Status.Phase == v1.MigrationPhaseSetupPipe &&
		migration.Spec.MigrationInfo.HostConnectStatus != v1.ConnectStatusConnected {
		var (
			connOutput      []byte
			isHostConnected bool
			subsysList      nvme.SubsystemList
		)
		// ensure Host connected to Target: read and connect
		// read connection status.
		subsysList, err = nvmeCli.ListSubsystems()
		if err != nil {
			return reconcile.Result{}, err
		}
		log.Info("check host connection status", "list-subsys", subsysList)
		for _, item := range subsysList.Subsystems {
			if item.NQN == destVolume.Spec.SpdkTarget.SubsysNQN {
				for _, path := range item.Paths {
					addr, svcId := nvme.ParseNvmePathAddress(path.Address)
					isHostConnected = addr == destVolume.Spec.SpdkTarget.Address && svcId == destVolume.Spec.SpdkTarget.SvcID
					if isHostConnected {
						break
					}
				}
			}
		}

		// if Destination Target is not connected, try connecting the target.
		if !isHostConnected {
			log.Info("start connecting dest target", "spdk", destVolume.Spec.SpdkTarget)
			var transType string

			var opts = nvme.ConnectTargetOpts{
				ReconnectDelaySec: 2,
				CtrlLossTMO:       10,
			}
			switch destVolume.Spec.SpdkTarget.TransType {
			case spdkclient.TransportTypeVFIOUSER:
				transType = "vfio-user"
				opts.HostTransAddr = destVolume.Spec.SpdkTarget.AddrFam
			case spdkclient.TransportTypeTCP:
				transType = "tcp"
			}
			connOutput, err = nvmeCli.ConnectTarget(transType, destVolume.Spec.SpdkTarget.Address, destVolume.Spec.SpdkTarget.SvcID, destVolume.Spec.SpdkTarget.SubsysNQN, opts)
			if err != nil {
				log.Error(err, string(connOutput))
				return reconcile.Result{}, err
			}
		}

		migration.Spec.MigrationInfo.HostConnectStatus = v1.ConnectStatusConnected
		migration.Finalizers = append(migration.Finalizers, v1.MigrationFinalizerHostConnected)
		_, err = migrationCli.Update(ctx, migration, metav1.UpdateOptions{})
		if err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	// Setp-3: wait sync pipe being ready

	// Step-4: start migration
	// check migration status;
	if migration.Status.Phase == v1.MigrationPhaseSyncing {
		// check autoswitch result
		if migration.Spec.MigrationInfo.AutoSwitch.Status != v1.ResultStatusSuccess &&
			migration.Spec.MigrationInfo.AutoSwitch.Enabled {
			log.Info("start checking autoswitch result")
			// check if host is writing to dest path
			var subsysList nvme.SubsystemList
			var alreadySwitched bool
			subsysList, err = nvmeCli.ListSubsystems()
			if err != nil {
				return reconcile.Result{}, err
			}
			log.Info("check autowitch status", "list-subsys", subsysList)
			for _, item := range subsysList.Subsystems {
				if item.NQN == migration.Spec.DestVolume.Spdk.SubsysNQN {
					for _, path := range item.Paths {
						addr, svcId := nvme.ParseNvmePathAddress(path.Address)
						alreadySwitched = path.PathState == "working" && addr == migration.Spec.DestVolume.Spdk.Address && svcId == migration.Spec.DestVolume.Spdk.SvcID
						if alreadySwitched {
							break
						}
					}
				}
			}

			if alreadySwitched {
				log.Info("nvme path is switched to dest volume, update AutoSwitch.Status to Success")
				migration.Spec.MigrationInfo.AutoSwitch.Status = v1.ResultStatusSuccess
				_, err = migrationCli.Update(ctx, migration, metav1.UpdateOptions{})
				if err != nil {
					return reconcile.Result{}, err
				}
			} else {
				log.Info("nvme path is not switched to dest volume, retry after 30s")
				return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
			}
		}

		return reconcile.Result{RequeueAfter: time.Minute}, nil
	}

	// Step-5: clean job and pipe
	if migration.Status.Phase == v1.MigrationPhaseCleaning {
		if migration.Spec.MigrationInfo.HostConnectStatus == v1.ConnectStatusConnected {
			// disconnect source volume; check host multi-path
			var subsysList nvme.SubsystemList
			var srcVolumePath nvme.Path
			var foundSrcVolumePath bool
			var needDisconnectSrcVolume bool
			subsysList, err = nvmeCli.ListSubsystems()
			if err != nil {
				return reconcile.Result{}, err
			}
			for _, item := range subsysList.Subsystems {
				if item.NQN == migration.Spec.DestVolume.Spdk.SubsysNQN {
					if len(item.Paths) > 1 {
						for _, path := range item.Paths {
							addr, svcId := nvme.ParseNvmePathAddress(path.Address)
							if addr == migration.Spec.SourceVolume.Spdk.Address &&
								svcId == migration.Spec.SourceVolume.Spdk.SvcID {
								srcVolumePath = path
								foundSrcVolumePath = true
							}
						}
					}
				}
			}

			// wait for PathState to be degraded
			if foundSrcVolumePath && srcVolumePath.PathState != "degraded" {
				log.Info("WARNING: found sourceVolume path, but PahtState is not degrated", "Path", srcVolumePath)
				return reconcile.Result{RequeueAfter: 20 * time.Second}, nil
			}

			needDisconnectSrcVolume = foundSrcVolumePath && srcVolumePath.PathState == "degraded"

			if needDisconnectSrcVolume {
				log.Info("disconnect src target", "spdk", srcVolume.Spec.SpdkTarget)

				var out []byte
				out, err = nvmeCli.DisconnectTarget(nvme.DisconnectTargetRequest{
					NQN:    migration.Spec.SourceVolume.Spdk.SubsysNQN,
					TrAddr: migration.Spec.SourceVolume.Spdk.Address,
					SvcID:  migration.Spec.SourceVolume.Spdk.SvcID,
				})
				if err != nil {
					log.Error(err, "DisconnectTarget failed", "output", string(out))
					return reconcile.Result{}, err
				}
			}

			log.Info("source volume connection is closed")

			migration.Spec.MigrationInfo.HostConnectStatus = v1.ConnectStatusDisconnected
			// remove MigrationFinalizerHostConnected
			var newFinalizers []string
			for _, item := range migration.Finalizers {
				if item != v1.MigrationFinalizerHostConnected {
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
