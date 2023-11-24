package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"code.alipay.com/dbplatform/node-disk-controller/pkg/agent/config"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/agent/pool"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/agent/pool/engine"
	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/kubeutil"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/generated/clientset/versioned"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/spdk/hostnqn"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/lvm"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

type PoolSyncer struct {
	poolService pool.StoragePoolServiceIface
	// storeCli is used to read/write StoragePool, AntstorVolumes from APIServer
	storeCli versioned.Interface
	// read node info from APIServer
	nodeGetter kubeutil.NodeInfoGetterIface
	cfg        config.Config
}

func NewPoolSyncer(poolService pool.StoragePoolServiceIface, storeCli versioned.Interface, nodeGetter kubeutil.NodeInfoGetterIface, cfg config.Config) *PoolSyncer {
	return &PoolSyncer{
		poolService: poolService,
		storeCli:    storeCli,
		nodeGetter:  nodeGetter,
		cfg:         cfg,
	}
}

func (ps *PoolSyncer) Start(ctx context.Context) (err error) {
	var evChan = make(chan pool.ChangedStatusPayload)
	ps.poolService.SpdkWatcher().Notify(evChan)

	poolTicker := time.NewTicker(10 * time.Minute)
	statusTicker := time.NewTicker(2 * time.Minute)

	// run sync pool once, to create pool immediately if not exist
	err = ps.syncPool()
	if err != nil {
		klog.Error(err)
	}

	for {
		klog.Info("syncPoolIteration start")
		startTime := time.Now()
		quit := ps.syncPoolIteration(poolTicker.C, statusTicker.C, evChan, ctx.Done())
		klog.Info("syncPoolIteration end, cost time", time.Since(startTime))
		if quit {
			klog.Info("quit PoolSyncer")
			return nil
		}
	}

	return
}

func (ps *PoolSyncer) syncPoolIteration(poolInterval, statusInterval <-chan time.Time, evCh <-chan pool.ChangedStatusPayload, quitCh <-chan struct{}) (quit bool) {
	var err error
	select {
	case <-poolInterval:
		// sync pool metadata and spec
		err = ps.syncPool()
		if err != nil {
			klog.Error(err)
		}
	case <-statusInterval:
		// sync pool status
		err = ps.updatePoolStatus()
		if err != nil {
			klog.Error(err)
		}
	// sync status if there is an Event
	case ev := <-evCh:
		if ev.Current.Error != ev.Last.Error && ev.Current.Error == nil {
			// recover spdk service
			// When PoolService is initialized, SdpkWatcher keeps watching status change of spdk tgt.
			klog.Info("found Spdk status changed, try to recover spdk service. event %+v", ev)
			// if spdk tgt service came back alive, try to recover targets
			if ev.SpdkBackAlive() {
				klog.Info("TODO recover spdk subsys")
				// err := spm.recoverSpdkTargets()
				// if err != nil {
				// 	klog.Error(err)
				// }
			}

			// update status
			err = ps.updatePoolStatus()
			if err != nil {
				klog.Error(err)
			}
		}
	case <-quitCh:
		return true
	}

	return false
}

func (ps *PoolSyncer) syncPool() (err error) {
	var pool = ps.poolService.GetStoragePool()
	// KernelLVM.Bytes may be 0, if SPDK takes control of all PCIe devices
	if pool == nil || pool.Name == "" || pool.Namespace == "" {
		err = fmt.Errorf("invalid StoragePool, empty name or ns")
		return
	}

	klog.Infof("syncing StoragePoool to APIServer, %s", pool.Name)

	if ps.storeCli == nil {
		err = fmt.Errorf("empty storeCli")
		return
	}

	// read from APIServer
	var apiPool *v1.StoragePool
	var spCli = ps.storeCli.VolumeV1().StoragePools(v1.DefaultNamespace)
	var spdkVer = ps.poolService.SpdkWatcher().ReadStatus()

	apiPool, err = spCli.Get(context.Background(), pool.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// create if not exist
			setPoolAttributes(pool, spdkVer)
			pool.Spec, err = ps.getPoolSpec()
			if err != nil {
				klog.Error(err)
				return
			}
			_, err = spCli.Create(context.Background(), pool, metav1.CreateOptions{})
			if err != nil {
				klog.Error(err)
			}
			return
		}
		klog.Error(err)
	} else {
		// 1. copy Annotations and Labels from API to local
		if pool.Annotations == nil {
			pool.Annotations = make(map[string]string)
		}
		for k, v := range apiPool.Annotations {
			pool.Annotations[k] = v
		}

		pool.Labels = apiPool.Labels
		// set hostnqn annotation
		if hostnqn.HostNQNValue != "" {
			pool.Annotations[v1.AnnotationHostNQN] = hostnqn.HostNQNValue
		}
		// override tgt-version annotation
		// if spdk service is not healthy, version could be empty
		if spdkVer.SpdkVersion != "" {
			pool.Annotations[v1.AnnotationTgtSpdkVersion] = spdkVer.SpdkVersion
		}
		// 2. copy Status from API to local. Sycner will update true status later.
		pool.Status = apiPool.Status
		// 3. assemble Spec(address, node info, lvm/spdk info ) in local
		pool.Spec, err = ps.getPoolSpec()
		if err != nil {
			// if vgs with error, update pool status
			if errors.Is(err, lvm.PvLostErr) {
				klog.Error("get PoolSpec error: ", err)
				setLvmCondition(apiPool, v1.StatusError, err.Error())
			} else {
				setLvmCondition(apiPool, v1.StatusOK, "")
			}
		}

		if !reflect.DeepEqual(pool.Annotations, apiPool.Annotations) || !reflect.DeepEqual(pool.Spec, apiPool.Spec) {
			apiPool.Annotations = pool.Annotations
			apiPool.Spec = pool.Spec
			bs, _ := json.Marshal(apiPool)
			klog.Infof("updating StoragePool: %s", string(bs))
			_, err = spCli.Update(context.Background(), apiPool, metav1.UpdateOptions{})
			if err != nil {
				klog.Error(err)
				return
			}
		}
	}

	return
}

func (ps *PoolSyncer) getPoolSpec() (spec v1.StoragePoolSpec, err error) {
	// poolService is initialized,
	var pool = ps.poolService.GetStoragePool()
	var poolInfo engine.StaticInfo
	poolInfo, err = ps.poolService.PoolEngine().PoolInfo(ps.cfg.Storage.Pooling.Name)
	if err != nil {
		klog.Error(err)
		return
	}
	spec.NodeInfo, err = ps.nodeGetter.GetByNodeID(pool.Name, kubeutil.NodeInfoOption(ps.cfg.NodeKeys))
	if err != nil {
		klog.Error(err)
		return
	}
	spec.Addresses = []corev1.NodeAddress{
		{
			Type:    corev1.NodeInternalIP,
			Address: spec.NodeInfo.IP,
		},
	}

	if poolInfo.LVM != nil {
		spec.KernelLVM = *poolInfo.LVM
	}
	if poolInfo.LVS != nil {
		spec.SpdkLVStore = *poolInfo.LVS
	}

	return
}

func setPoolAttributes(pool *v1.StoragePool, spdkVer pool.SpdkStatus) {
	if pool.Annotations == nil {
		pool.Annotations = make(map[string]string)
	}
	if pool.Labels == nil {
		pool.Labels = make(map[string]string)
	}
	pool.Labels[v1.PoolLabelsNodeSnKey] = pool.Name
	if spdkVer.Error == nil {
		pool.Annotations[v1.AnnotationTgtSpdkVersion] = spdkVer.SpdkVersion
	}

	if pool.Status.Capacity == nil {
		pool.Status.Capacity = make(corev1.ResourceList)
	}
	quant := resource.NewQuantity(pool.GetVgTotalBytes(), resource.BinarySI)
	pool.Status.Capacity[v1.ResourceDiskPoolByte] = *quant
	pool.Status.VGFreeSize = *quant
}

func (ps *PoolSyncer) updatePoolStatus() (err error) {
	pool := ps.poolService.GetStoragePool()
	// update pool's status to truth
	setStatusConditions(pool, ps.poolService)
	errVG := setStatusVgFree(pool, ps.poolService)

	realStatus := pool.Status.DeepCopy()

	cli := ps.storeCli.VolumeV1().StoragePools(v1.DefaultNamespace)
	apiPool, err := cli.Get(context.Background(), pool.Name, metav1.GetOptions{})
	if err != nil {
		klog.Error(err, "cannt get pool from APIServer, not updateStoragePoolStatus")
		return err
	}

	if errors.Is(errVG, lvm.PvLostErr) {
		klog.Error(errVG)
		// update condition of lvm
		setLvmCondition(apiPool, v1.StatusError, errVG.Error())
	} else {
		klog.Info("lvm is ok", errVG)
		setLvmCondition(apiPool, v1.StatusOK, "")
	}

	if apiPool.Status.Capacity == nil {
		apiPool.Status.Capacity = make(corev1.ResourceList)
	}

	var condEqual = reflect.DeepEqual(realStatus.Conditions, apiPool.Status.Conditions)
	var freeByteEqual = realStatus.VGFreeSize.Equal(apiPool.Status.VGFreeSize)
	var totalByteEqual = realStatus.Capacity[v1.ResourceDiskPoolByte].Equal(apiPool.Status.Capacity[v1.ResourceDiskPoolByte])

	if !condEqual || !freeByteEqual || !totalByteEqual {
		// to update status
		klog.Infof("update StoragePool condition and cap, %+v, server-side status is %+v", *realStatus, apiPool.Status)
		apiPool.Status.Conditions = realStatus.Conditions
		apiPool.Status.VGFreeSize = realStatus.VGFreeSize.DeepCopy()
		apiPool.Status.Capacity[v1.ResourceDiskPoolByte] = realStatus.Capacity[v1.ResourceDiskPoolByte]
		// APIServer is supposed to check resourceVersion before updating the data.
		// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency
		// https://stackoverflow.com/questions/52910322/kubernetes-resource-versioning
		latestPool, err := cli.UpdateStatus(context.Background(), apiPool, metav1.UpdateOptions{})
		if err != nil {
			klog.Error(err)
			return err
		} else {
			pool.Status = latestPool.Status
		}
	} else {
		klog.Info("StoragePool status not changed")
	}

	return
}

func setStatusConditions(pool *v1.StoragePool, poolSvc pool.StoragePoolServiceIface) {
	// get spdk condition
	var status v1.ConditionStatus = v1.StatusError
	var foundSpkdCondition bool
	for _, item := range pool.Status.Conditions {
		if item.Type == v1.PoolConditionSpkdHealth {
			foundSpkdCondition = true
		}
	}

	if currState := poolSvc.SpdkWatcher().Current(); currState.Error == nil {
		status = v1.StatusOK
	} else {
		klog.Errorf("spdk RPC is not healthy, err %+v", currState)
	}

	// never set spdk condition
	if len(pool.Status.Conditions) == 0 || !foundSpkdCondition {
		pool.Status.Conditions = append(pool.Status.Conditions, v1.PoolCondition{
			Type:   v1.PoolConditionSpkdHealth,
			Status: status,
		})
		return
	} else {
		for i := 0; i < len(pool.Status.Conditions); i++ {
			if pool.Status.Conditions[i].Type == v1.PoolConditionSpkdHealth {
				pool.Status.Conditions[i].Status = status
			}
		}
	}
}

func setStatusVgFree(pool *v1.StoragePool, poolSvc pool.StoragePoolServiceIface) (err error) {
	totalByte, freeByte, err := poolSvc.PoolEngine().TotalAndFreeSize()
	if err != nil {
		klog.Error(err)
		return err
	}

	if pool.Status.Capacity == nil {
		pool.Status.Capacity = make(corev1.ResourceList)
	}

	quant := resource.NewQuantity(int64(freeByte), resource.BinarySI)
	total := resource.NewQuantity(int64(totalByte), resource.BinarySI)
	// quant.String() is important, it will set q.s field. This field will affect reflect.DeepEqual()
	klog.Infof("set local pool ResourceLvmFreeByte %s, ResourceDiskPoolByte %s", quant.String(), total.String())
	// Key storage/vg-free indicates the real free bytes left in the VG
	pool.Status.VGFreeSize = *quant
	pool.Status.Capacity[v1.ResourceDiskPoolByte] = *total

	return
}

func setLvmCondition(sp *v1.StoragePool, status v1.ConditionStatus, msg string) {
	for idx, item := range sp.Status.Conditions {
		if item.Type == v1.PoolConditionLvmHealth {
			sp.Status.Conditions[idx].Status = status
			sp.Status.Conditions[idx].Message = msg
		}
	}
}
