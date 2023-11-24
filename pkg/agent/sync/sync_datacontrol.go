package sync

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/kubeutil"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/generated/clientset/versioned"
	antstorinformers "code.alipay.com/dbplatform/node-disk-controller/pkg/generated/informers/externalversions"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/spdk/jsonrpc/nvme"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/lvm"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/misc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	nvmeClientFilePath = "/home/admin/nvmeof/bin/nvme"
	twentySec          = 20 * time.Second
)

type DataControlReconciler struct {
	nodeID   string
	storeCli versioned.Interface
	// spdkCli  spdk.SpdkServiceIface
}

func NewDataControlReconciler(nodeID string, storeCli versioned.Interface) *DataControlReconciler {
	return &DataControlReconciler{
		nodeID:   nodeID,
		storeCli: storeCli,
	}
}

func (r *DataControlReconciler) Start(ctx context.Context) (err error) {
	informerFactory := antstorinformers.NewFilteredSharedInformerFactory(r.storeCli, time.Hour, v1.DefaultNamespace, func(lo *metav1.ListOptions) {
		lo.LabelSelector = fmt.Sprintf("%s=%s", v1.TargetNodeIdLabelKey, r.nodeID)
	})

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	informer := informerFactory.Volume().V1().AntstorDataControls().Informer()
	informer.AddEventHandler(kubeutil.CommonResourceEventHandlerFuncs(queue))

	go informer.Run(ctx.Done())
	// or informerFactory.Start(spm.quitChan)

	kubeutil.NewSimpleController("agent-datacontrol", queue, r).Start(ctx)
	return
}

func (r *DataControlReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var (
		ns          = req.Namespace
		name        = req.Name
		err         error
		dataControl *v1.AntstorDataControl
		cli         = r.storeCli.VolumeV1().AntstorDataControls(ns)
	)

	dataControl, err = cli.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	if dataControl.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, dataControl)
	}

	// skip ready datacontrol
	if dataControl.Status.Status == v1.VolumeStatusReady {
		return reconcile.Result{}, nil
	}

	// Step-1: VolumeGroup must be ready
	var (
		vgCli      = r.storeCli.VolumeV1().AntstorVolumeGroups(dataControl.Namespace)
		volGroups  []*v1.AntstorVolumeGroup
		lvmControl = &v1.LVMControl{}
	)

	if len(dataControl.Spec.VolumeGroups) == 0 {
		klog.Warningf("VolumeGroups is empty, cannot setup data control")
		return reconcile.Result{RequeueAfter: time.Minute}, nil
	}

	if dataControl.Spec.LVM != nil {
		lvmControl = dataControl.Spec.LVM.DeepCopy()
	}

	for _, vg := range dataControl.Spec.VolumeGroups {
		var volGroup *v1.AntstorVolumeGroup
		volGroup, err = vgCli.Get(ctx, vg.Name, metav1.GetOptions{})
		if err != nil {
			klog.Error(err, "getting VolumeGroup failed")
		}

		if volGroup == nil || volGroup.Status.Status != v1.VolumeStatusReady {
			klog.Infof("VolumeGroup %s is not ready, %s, wait 20 sec", vg.Name, volGroup.Status.Status)
			return reconcile.Result{RequeueAfter: twentySec}, nil
		} else {
			volGroups = append(volGroups, volGroup)
			for idx, item := range volGroup.Status.VolumeStatus {
				// TODO: may miss PV
				if item.SpdkTarget != nil {
					lvmControl.PVs = append(lvmControl.PVs, v1.LVMControlPV{
						// Volumes length should be bigger than VolumeStatus length
						VolId:      volGroup.Spec.Volumes[idx].VolId,
						TargetInfo: *item.SpdkTarget,
					})
				}
			}
		}
	}

	// Setp-2:  sync LVM control
	if dataControl.Spec.EngineType == v1.PoolModeKernelLVM {
		if !misc.InSliceString(string(v1.KernelLVolFinalizer), dataControl.Finalizers) {
			// 1. list subsys and do connect targets
			var (
				connectedNQN []string
				out          []byte
				subsysList   nvme.SubsystemList
				nvmeList     []nvme.NvmeDevice
				nvmeCli      = nvme.NewClientWithCmdPath(nvmeClientFilePath)
				devs         []string
				vgName       = fmt.Sprintf("%s-%s", dataControl.Namespace, dataControl.Name)
				lvName       = dataControl.Name
				vgs          []lvm.VG
				lvs          []lvm.LV
				pvs          []lvm.PV
			)

			subsysList, err = nvmeCli.ListSubsystems()
			if err != nil {
				klog.Error(err)
				return reconcile.Result{RequeueAfter: twentySec}, nil
			}
			for _, subsys := range subsysList.Subsystems {
				connectedNQN = append(connectedNQN, subsys.NQN)
			}

			for _, item := range lvmControl.PVs {
				target := item.TargetInfo
				if !misc.InSliceString(target.SubsysNQN, connectedNQN) {
					var transType = strings.ToLower(target.TransType)
					out, err = nvmeCli.ConnectTarget(transType, target.Address, target.SvcID, target.SubsysNQN, nvme.ConnectTargetOpts{
						ReconnectDelaySec: 2,
						CtrlLossTMO:       10,
					})
					if err != nil {
						klog.Error(err)
						return reconcile.Result{RequeueAfter: twentySec}, nil
					}
					klog.Infof("connect spdk target %+v, output %s", target, string(out))
				}
			}

			// 2. list subsys, sync devPath
			nvmeList, err = nvmeCli.ListNvmeDisk()
			if err != nil {
				klog.Error(err)
				return reconcile.Result{RequeueAfter: twentySec}, nil
			}
			for idx, pv := range lvmControl.PVs {
				// find and set device path
				var found bool = false
				for _, item := range nvmeList {
					if item.SerialNumber == pv.TargetInfo.SerialNum {
						found = true
						lvmControl.PVs[idx].DevPath = item.DevicePath
						devs = append(devs, item.DevicePath)
					}
				}

				if !found {
					klog.Errorf("not found device of target %+v", pv)
					return reconcile.Result{RequeueAfter: twentySec}, nil
				}
			}

			// 3. list PVs and setup PVs
			pvs, err = lvm.LvmUtil.ListPV()
			if err != nil {
				klog.Error(err)
				return reconcile.Result{RequeueAfter: twentySec}, nil
			}
			pvNameSet := misc.NewEmptySet()
			devSet := misc.FromSlice(devs)
			for _, pv := range pvs {
				pvNameSet.Add(pv.PvName)
			}
			toCreatePVs := devSet.Difference(pvNameSet)

			if toCreatePVs.Size() > 0 {
				klog.Infof("create pvs %+v", toCreatePVs.Values())
				err = lvm.LvmUtil.CreatePV(toCreatePVs.Values())
				if err != nil {
					klog.Error(err)
					return reconcile.Result{RequeueAfter: twentySec}, nil
				}
			}

			// 4. list and setup VG
			vgs, err = lvm.LvmUtil.ListVG()
			if err != nil {
				klog.Error(err)
				return reconcile.Result{RequeueAfter: twentySec}, nil
			}
			var foundVg bool
			for _, item := range vgs {
				if item.Name == vgName {
					foundVg = true
					lvmControl.VG = vgName
				}
			}
			if !foundVg {
				_, err = lvm.LvmUtil.CreateVG(vgName, devs)
				if err != nil {
					klog.Error(err)
					return reconcile.Result{RequeueAfter: twentySec}, nil
				}
				lvmControl.VG = vgName
			}

			// 5. create linear lvol
			lvs, err = lvm.LvmUtil.ListLVInVG(vgName)
			if err != nil {
				klog.Error(err)
				return reconcile.Result{RequeueAfter: twentySec}, nil
			}
			var foundLV bool
			for _, item := range lvs {
				if item.Name == lvName {
					foundLV = true
					lvmControl.LVol = lvName
				}
			}
			if !foundLV {
				_, err = lvm.LvmUtil.CreateLinearLV(vgName, lvName, lvm.LvOption{
					LogicSize: "100%FREE",
				})
				if err != nil {
					klog.Error(err)
					return reconcile.Result{RequeueAfter: twentySec}, nil
				}
				lvmControl.LVol = lvName
			}

			dataControl.Spec.LVM = lvmControl
			dataControl.Finalizers = append(dataControl.Finalizers, v1.KernelLVolFinalizer)

			// update LVM info
			_, err = cli.Update(ctx, dataControl, metav1.UpdateOptions{})
			if err != nil {
				klog.Error(err)
				return reconcile.Result{RequeueAfter: twentySec}, nil
			}
		}

		if dataControl.Spec.LVM != nil && dataControl.Spec.LVM.LVol != "" {
			// update datacontrol status
			dataControl.Status.Status = v1.VolumeStatusReady
			_, err = cli.UpdateStatus(ctx, dataControl, metav1.UpdateOptions{})
			if err != nil {
				klog.Error(err)
				return reconcile.Result{RequeueAfter: twentySec}, nil
			}
		}
	}

	return reconcile.Result{}, nil
}

func (r *DataControlReconciler) handleDeletion(ctx context.Context, dataControl *v1.AntstorDataControl) (reconcile.Result, error) {
	var (
		err     error
		cli     = r.storeCli.VolumeV1().AntstorDataControls(dataControl.Namespace)
		nvmeCli = nvme.NewClientWithCmdPath(nvmeClientFilePath)
		vgName  string
		devs    []string
		vgs     []lvm.VG
		pvs     []lvm.PV
	)

	if dataControl.Spec.EngineType == v1.PoolModeKernelLVM && dataControl.Spec.LVM != nil {
		vgName = dataControl.Spec.LVM.VG
		for _, item := range dataControl.Spec.LVM.PVs {
			devs = append(devs, item.DevPath)
		}

		// 1. delete vg
		if vgName != "" {
			vgs, err = lvm.LvmUtil.ListVG()
			if err != nil {
				klog.Error(err, "get vg failed")
			}
			var foundVg bool
			for _, item := range vgs {
				if item.Name == vgName {
					foundVg = true
				}
			}
			if foundVg {
				err = lvm.LvmUtil.RemoveVG(vgName)
				if err != nil {
					klog.Error(err, "rm vg failed, retry in 20 sec")
					return reconcile.Result{RequeueAfter: twentySec}, nil
				}
			}
		}

		// 2. delete pv
		pvs, err = lvm.LvmUtil.ListPV()
		if err != nil {
			klog.Error(err, "list pv failed")
		}
		pvNameSet := misc.NewEmptySet()
		devSet := misc.FromSlice(devs)
		for _, pv := range pvs {
			pvNameSet.Add(pv.PvName)
		}
		toDelDevs := devSet.Intersect(pvNameSet)
		if toDelDevs.Size() > 0 {
			klog.Infof("remove pvs %+v", toDelDevs.Values())
			err = lvm.LvmUtil.RemovePVs(toDelDevs.Values())
			if err != nil {
				klog.Error(err, "rm pv failed, retry in 20 sec.")
				return reconcile.Result{RequeueAfter: twentySec}, nil
			}
		}

		// 3. disconnect target by nqn
		for _, item := range dataControl.Spec.LVM.PVs {
			if item.TargetInfo.SubsysNQN != "" {
				// "nvme disconnect" command is reentrant. if NQN device is not connected, the command return 0 exit-code.
				var out []byte
				out, err = nvmeCli.DisconnectTarget(nvme.DisconnectTargetRequest{
					NQN: item.TargetInfo.SubsysNQN,
				})
				if err != nil {
					klog.Error(err, string(out))
					// TODO: not ignore?
				}
			}
		}

		// remove finalizer
		var idxToDel int
		var found bool
		for idx, item := range dataControl.Finalizers {
			if item == v1.KernelLVolFinalizer {
				idxToDel = idx
				found = true
				break
			}
		}
		dataControl.Finalizers = append(dataControl.Finalizers[:idxToDel], dataControl.Finalizers[idxToDel+1:]...)
		if found {
			_, err = cli.Update(ctx, dataControl, metav1.UpdateOptions{})
			if err != nil {
				klog.Error(err, "update datacontrol failed, retry in 20 sec")
				return reconcile.Result{RequeueAfter: twentySec}, nil
			}
		}

	}

	return reconcile.Result{}, nil
}
