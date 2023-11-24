package plugin

import (
	"fmt"

	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/syncmeta"
	"github.com/go-logr/logr"
)

type MetaSyncPlugin struct {
	// to sync metadata
	Syncer syncmeta.MetaSyncer
	log    logr.Logger
}

func (r *MetaSyncPlugin) Name() string {
	return "MetaSync"
}

func (r *MetaSyncPlugin) HandleDeletion(ctx *Context) (err error) {
	result := r.Reconcile(ctx)
	return result.Error
}

func (r *MetaSyncPlugin) Reconcile(ctx *Context) (result Result) {
	if ctx == nil {
		return Result{Error: fmt.Errorf("plugin context is nil")}
	}

	var (
		obj = ctx.Object
		req = ctx.Request
	)
	r.log = ctx.Log

	if obj == nil {
		r.log.Error(nil, "object is nil", "req", req)
		return
	}

	if r.Syncer == nil {
		r.log.Info("skip syncing", "req", req)
		return
	}

	result.Error = r.Syncer.Sync(obj)
	return
}

/*
func (r *MetaSyncPlugin) syncSnapshot(snap *v1.AntstorSnapshot, req ctrl.Request) {
	var (
		err error
		log = r.log.WithValues("Snapshot", req.NamespacedName)
	)

	log.Info("found Snapshot to sync")

	if snap.DeletionTimestamp != nil {
		log.Info("mark Snapshot as deleted")
		err = r.Syncer.DeleteAntstorSnapshot(snap)
		if err != nil {
			log.Error(err, "DeleteSnapshot failed")
		}
		return
	} else {
		err = r.Syncer.SaveAntstorSnapshot(snap)
		if err != nil {
			log.Error(err, "SaveSnapshot failed")
			return
		}
	}
}

func (r *MetaSyncPlugin) syncVolume(volume *v1.AntstorVolume, req ctrl.Request) {
	var (
		err error
		log = r.log.WithValues("Volume", req.NamespacedName)
	)

	log.Info("found AntstorVolume to sync")
	if volume.DeletionTimestamp != nil {
		log.Info("mark AntstorVolume as deleted")
		err = r.Syncer.DeleteAntstorVolume(volume)
		if err != nil {
			log.Error(err, "DelelteAntstorVolume failed")
		}
		return
	} else {
		err = r.Syncer.SaveAntstorVolume(volume)
		if err != nil {
			log.Error(err, "SaveAntstorVolume failed")
			return
		}
	}
}

func (r *MetaSyncPlugin) syncPool(pool *v1.StoragePool, req ctrl.Request) {
	var (
		err error
		log = r.log.WithValues("StoragePool", req.NamespacedName)
	)
	log.Info("found StoragePool to sync")

	if pool.DeletionTimestamp != nil {
		log.Info("mark StoragePool as deleted")
		err = r.Syncer.DeleteStoragePool(pool)
		if err != nil {
			log.Error(err, "DeleteStoragePool failed")
		}
		return
	} else {
		err = r.Syncer.SaveStoragePool(pool)
		if err != nil {
			log.Error(err, "SaveStoragePool")
			return
		}
	}
}

*/
