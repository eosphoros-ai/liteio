package manager

import (
	"context"
	"fmt"
	"time"

	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"lite.io/liteio/pkg/generated/clientset/versioned"
	coordv1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type HeartbeatService struct {
	Interval time.Duration
	kubeCli  kubernetes.Interface
	storeCli versioned.Interface
	nodeID   string
	// cache
	lease *coordv1.Lease
}

// StartHeartbeat is an endless loop to send heartbeat
func (hs *HeartbeatService) Start(ctx context.Context) (er error) {
	var err error
	tick := time.NewTicker(hs.Interval)
	for {
		select {
		case <-tick.C:
			err = hs.doHeartbeat()
			if err != nil {
				klog.Errorf("Heartbeat failed: %+v", err)
			} else {
				klog.Info("Heartbeat is ok")
			}
		case <-ctx.Done():
			klog.Info("Agent quit heartbeat loop")
			return
		}
	}
}

func (hs *HeartbeatService) doHeartbeat() (err error) {
	// create or update Lease Object
	if hs.kubeCli == nil {
		return fmt.Errorf("kube client is nil")
	}

	var renew = metav1.NowMicro()
	var holder = hs.nodeID
	var leaseDurSec int32 = 40

	// if hs.lease is an empty value, do Get or Create
	if hs.lease == nil || hs.lease.Name == "" {
		hs.lease, err = hs.kubeCli.CoordinationV1().Leases(LeaseNamespace).Get(context.Background(), holder, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				var pool *v1.StoragePool
				spCli := hs.storeCli.VolumeV1().StoragePools(AntstorDefaultNamespace)
				pool, err = spCli.Get(context.Background(), holder, metav1.GetOptions{})
				if err != nil {
					klog.Error(err)
					return
				}
				// create a new Lease
				hs.lease = &coordv1.Lease{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: LeaseNamespace,
						Name:      holder,
						OwnerReferences: []metav1.OwnerReference{
							{
								// TODO: pool's APIVersion, Kind are empty
								APIVersion: v1.SchemeGroupVersion.String(),
								Kind:       v1.StoragePoolKind,
								Name:       pool.Name,
								UID:        pool.UID,
							},
						},
					},
					Spec: coordv1.LeaseSpec{
						HolderIdentity:       &holder,
						RenewTime:            &renew,
						LeaseDurationSeconds: &leaseDurSec,
					},
				}
				hs.lease, err = hs.kubeCli.CoordinationV1().Leases(LeaseNamespace).Create(context.Background(), hs.lease, metav1.CreateOptions{})
				if err != nil {
					hs.lease = nil
					klog.Error(err)
				}
				return
			} else {
				klog.Error(err)
				return
			}
		}
	}

	// update renew time
	hs.lease.Spec.RenewTime = &renew
	hs.lease, err = hs.kubeCli.CoordinationV1().Leases(LeaseNamespace).Update(context.Background(), hs.lease, metav1.UpdateOptions{})
	if err != nil {
		klog.Error(err)
		// if updating Lease failed, an empty Lease will be returned. So hs.lease.Name may be empty.
		// hs.lease should be set back if error occurs.
		// hs.lease object may be not latest version, so it should be fetched from APIServer.
		hs.lease, err = hs.kubeCli.CoordinationV1().Leases(LeaseNamespace).Get(context.Background(), holder, metav1.GetOptions{})
		if err != nil {
			klog.Error(err)
		}
		return
	}

	return
}
