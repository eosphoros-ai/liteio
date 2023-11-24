package extender

import (
	"context"
	"time"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/kubeutil"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	LeaseNamespace = "obnvmf"
)

var (
	// check heartbeat every 1 min
	checkLoopInterval = 1 * time.Minute
	// if last heartbeat is 3 min ago, consider node lost
	nodeExpireDuration = 3 * time.Minute
	// if last heartbeat is 72h ago, consider node offline
	nodeOfflineExpireDuration = 72 * time.Hour
)

type HeartbeatManager struct {
	state state.StateIface
	// updater to update status of StoragePool
	updater kubeutil.StoragePoolUpdater
	// kube client
	kubeCli kubernetes.Interface
	// time when HeartbeatManager start
	startTime time.Time
}

func NewHeartbeatManager(state state.StateIface, updater kubeutil.StoragePoolUpdater, kubeCli kubernetes.Interface) *HeartbeatManager {
	hm := &HeartbeatManager{
		state:     state,
		updater:   updater,
		startTime: time.Now(),
		kubeCli:   kubeCli,
	}

	return hm
}

// Start implements Runnable
func (hm *HeartbeatManager) Start(ctx context.Context) (err error) {
	var tick = time.NewTicker(checkLoopInterval)
	for {
		select {
		case <-tick.C:
			err = hm.checkHeartbeats()
			if err != nil {
				klog.Error(err)
			}
		case <-ctx.Done():
			klog.Info("quit heartbeat loop")
			return nil
		}
	}
}

func (hm *HeartbeatManager) checkHeartbeats() (err error) {
	// if Agent registered a StoragePool, but not created a Lease. The StoragePool's status will be be updated here.
	// This situation could be avoided if the initialized status of StoragePool is Unknown.
	var leaseCli = hm.kubeCli.CoordinationV1().Leases(LeaseNamespace)
	allLeases, err := leaseCli.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		klog.Error(err)
		return
	}
	klog.Infof("found %d agent Leases", len(allLeases.Items))

	for _, lease := range allLeases.Items {
		nodeID := lease.Name
		durationSinceLastHB := time.Since(lease.Spec.RenewTime.Time)
		lostHB := durationSinceLastHB > nodeExpireDuration
		shouldOffline := durationSinceLastHB > nodeOfflineExpireDuration
		node, err := hm.state.GetNodeByNodeID(nodeID)
		if err != nil {
			klog.Error(err, " node id:", nodeID)
			_, err = hm.kubeCli.CoreV1().Nodes().Get(context.Background(), nodeID, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				klog.Infof("node not exists, so delete lease of node %s", nodeID)
				err = leaseCli.Delete(context.Background(), lease.Name, metav1.DeleteOptions{})
				if err != nil {
					klog.Error(err)
				}
			}
			continue
		}

		// heartbeat is recovered, update status to ready
		if !lostHB && (node.Pool.Status.Status == v1.PoolStatusUnknown || node.Pool.Status.Status == v1.PoolStatusOffline) {
			klog.Infof("Setting node %s status to Ready", nodeID)
			err = hm.updater.UpdateStoragePoolStatus(node.Pool, v1.PoolStatusReady)
			if err != nil {
				klog.Error(err)
			}
		}

		// lost heartbeat, update status to unknown
		if lostHB && node.Pool.Status.Status == v1.PoolStatusReady {
			klog.Infof("Setting node %s status to Unknown", nodeID)
			err = hm.updater.UpdateStoragePoolStatus(node.Pool, v1.PoolStatusUnknown)
			if err != nil {
				klog.Error(err)
			}
		}

		// lost HB for too long, set status to offline
		if shouldOffline {
			klog.Infof("Setting node %s status to Offline", nodeID)
			err = hm.updater.UpdateStoragePoolStatus(node.Pool, v1.PoolStatusOffline)
			if err != nil {
				klog.Error(err)
			}
		}
	}

	return
}
