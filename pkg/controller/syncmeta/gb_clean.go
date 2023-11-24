package syncmeta

import (
	"context"
	"time"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"github.com/go-xorm/xorm"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type GarbageCleaner struct {
	K8SCluster    string
	CleanInterval time.Duration
	Client        client.Client
	engine        *xorm.Engine
}

func NewGbCleaner(cluster string, interval time.Duration, cli client.Client, engine *xorm.Engine) *GarbageCleaner {
	return &GarbageCleaner{
		K8SCluster:    cluster,
		CleanInterval: interval,
		Client:        cli,
		engine:        engine,
	}
}

// Run start cleaning deleted resources periodically
func (gc *GarbageCleaner) Run() {
	var tick = time.Tick(gc.CleanInterval)
	var err error
	for range tick {
		err = gc.cleanStoragePool()
		if err != nil {
			klog.Error(err)
		}

		err = gc.cleanAntstorVolume()
		if err != nil {
			klog.Error(err)
		}
	}
}

func (gc *GarbageCleaner) cleanStoragePool() (err error) {
	klog.Info("GarbageCleaner is collecting deleted StoragePool")
	// list all storagepool where updated_at < (1 day ago) and status != "offline"
	var spList []*StoragePoolBriefMapping
	var sess = gc.engine.NewSession()
	defer sess.Close()
	var tsOneDayAgo = time.Now().Add(-24 * time.Hour).Unix()
	spList, err = ListStoragePoolBriefByCondition(sess, Condition{
		// "status <>":    string(v1.PoolStatusOffline),
		"cluster_name": gc.K8SCluster,
		"updated_at <": tsOneDayAgo,
		"deleted_at":   0,
	})
	if err != nil {
		return
	}

	for _, sp := range spList {
		var apiPool v1.StoragePool
		var apiErr error
		var needUpdate bool
		apiErr = gc.Client.Get(context.Background(), client.ObjectKey{
			Namespace: v1.DefaultNamespace,
			Name:      sp.Name,
		}, &apiPool)

		// check if it is deleted
		if apiErr == nil {
			if apiPool.DeletionTimestamp != nil {
				needUpdate = true
				sp.DeletedAt = int(apiPool.DeletionTimestamp.Unix())
			}
		} else {
			if errors.IsNotFound(apiErr) {
				needUpdate = true
				sp.DeletedAt = int(time.Now().Unix())
				sp.Status = string(v1.PoolStatusOffline)
			} else {
				klog.Error(err)
			}
		}

		// update DB
		if needUpdate {
			klog.Infof("found storagepool %+v is deleted", sp)
			// do update
			err = MarkDeleteStoragePool(sess, &StoragePoolMapping{
				ClusterName: sp.ClusterName,
				Name:        sp.Name,
				DeletedAt:   sp.DeletedAt,
				Status:      sp.Status,
			})
			if err != nil {
				klog.Error(err)
				// continue collecting next sotragepool
			}
		}
	}

	return
}

func (gc *GarbageCleaner) cleanAntstorVolume() (err error) {
	klog.Info("GarbageCleaner is collecting deleted AntstorVolume")
	// list all antstorvolume where updated_at < (1 day ago) and status != "deleted"
	var avList []*AntstorVolumeBriefMapping
	var sess = gc.engine.NewSession()
	defer sess.Close()
	var tsOneDayAgo = time.Now().Add(-24 * time.Hour).Unix()
	avList, err = ListAntstorVolumeBriefByCondition(sess, Condition{
		"cluster_name": gc.K8SCluster,
		"status <>":    string(v1.VolumeStatusDeleted),
		"updated_at <": tsOneDayAgo,
		"deleted_at":   0,
	})
	if err != nil {
		return
	}

	for _, av := range avList {
		var apiVol v1.AntstorVolume
		var apiErr error
		var needUpdate bool
		apiErr = gc.Client.Get(context.Background(), client.ObjectKey{
			Namespace: v1.DefaultNamespace,
			Name:      av.Name,
		}, &apiVol)

		// check if it is deleted
		if apiErr == nil {
			if apiVol.DeletionTimestamp != nil {
				needUpdate = true
				av.DeletedAt = int(apiVol.DeletionTimestamp.Unix())
			}
		} else {
			if errors.IsNotFound(apiErr) {
				needUpdate = true
				av.DeletedAt = int(time.Now().Unix())
				av.Status = string(v1.VolumeStatusDeleted)
			} else {
				klog.Error(err)
			}
		}

		// update DB
		if needUpdate {
			klog.Infof("found antstorvolume %+v is deleted", av)
			// do update
			err = MarkDeleteAntstorVolume(sess, &AntstorVolumeMapping{
				ClusterName: av.ClusterName,
				Name:        av.Name,
				DeletedAt:   av.DeletedAt,
				Status:      av.Status,
			})
			if err != nil {
				klog.Error(err)
				// continue collecting next antstorvolume
			}
		}
	}

	return
}
