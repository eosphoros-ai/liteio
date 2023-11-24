package syncmeta

import (
	"reflect"
	"time"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/mysql"
	"github.com/go-xorm/xorm"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MetaSyncer interface {
	Sync(obj runtime.Object) (err error)
}

// OBSink sync meta data to OB table
type OBSyncer struct {
	K8SCluster string
	connInfo   mysql.DSNProvidor
	engine     *xorm.Engine
}

func NewOBSyncer(cluster string, connInfo mysql.DSNProvidor, cli client.Client) (s *OBSyncer, err error) {
	s = &OBSyncer{
		K8SCluster: cluster,
		connInfo:   connInfo,
	}

	s.engine, err = mysql.NewMySQLEngine(connInfo, true, false)
	if err != nil {
		klog.Error(err)
		return
	}

	if cli != nil {
		s.startGbClean(cli)
	}
	return
}

func (s *OBSyncer) startGbClean(cli client.Client) {
	klog.Info("starting metadata garbage cleaner")
	gbClean := NewGbCleaner(s.K8SCluster, time.Hour, cli, s.engine)
	go gbClean.Run()
}

func (s *OBSyncer) Sync(obj runtime.Object) (err error) {
	switch val := obj.(type) {
	case *v1.StoragePool:
		err = s.syncStoragePool(val)
	case *v1.AntstorVolume:
		err = s.syncAntstorVolume(val)
	case *v1.AntstorSnapshot:
		err = s.syncAntstorSnapshot(val)
	case *v1.AntstorDataControl:
		err = s.syncAntstorDataControl(val)
	case *v1.AntstorVolumeGroup:
		err = s.syncAntstorVolumeGroup(val)
	}
	return
}

func (s *OBSyncer) syncStoragePool(sp *v1.StoragePool) (err error) {
	// convert to Mapping, read from DB, compare with DeepEqual()
	spMap := ToStoragePoolMapping(s.K8SCluster, sp)

	// handle deletion
	if sp.DeletionTimestamp != nil {
		spMap.DeletedAt = int(time.Now().Unix())
		sess := s.engine.NewSession()
		err = MarkDeleteStoragePool(sess, spMap)
		return
	}

	// insert or update
	sess := s.engine.NewSession()
	spFromDB, has, err := GetStoragePool(sess, s.K8SCluster, sp.Name)
	if err != nil {
		return
	}

	if !has {
		// Insert SP to DB
		err = CreateStoragePool(sess, spMap)
		return
	} else {
		// Compare and Update
		spMap.CreatedAt = spFromDB.CreatedAt
		spMap.UpdatedAt = spFromDB.UpdatedAt

		if !reflect.DeepEqual(spMap, spFromDB) {
			err = UpdateStoragePool(sess, spMap)
			return
		}
	}

	return
}

func (s *OBSyncer) syncAntstorVolume(vol *v1.AntstorVolume) (err error) {
	// convert to Mapping, read from DB, compare with DeepEqual()
	volMap := ToAntstorVolumeMapping(s.K8SCluster, vol)

	if vol.DeletionTimestamp != nil {
		volMap.DeletedAt = int(time.Now().Unix())
		volMap.Status = string(v1.VolumeStatusDeleted)
		sess := s.engine.NewSession()
		err = MarkDeleteAntstorVolume(sess, volMap)
		return
	}

	sess := s.engine.NewSession()
	volFromDB, has, err := GetAntstorVolume(sess, s.K8SCluster, vol.Namespace, vol.Name)
	if err != nil {
		return
	}

	if !has {
		// Insert Vol to DB
		err = CreateAntstorVolume(sess, volMap)
		if err != nil {
			return
		}
		// update vol ext
		// volExtMap := ToAntstorVolumeExtMapping(vol, volFromDB.ID)
		// err = CreateAntstorVolumeExt(sess, volExtMap)
		return
	} else {
		// Compare and Update
		volMap.CreatedAt = volFromDB.CreatedAt
		volMap.UpdatedAt = volFromDB.UpdatedAt

		if !reflect.DeepEqual(volMap, volFromDB) {
			err = UpdateAntstorVolume(sess, volMap)
			return
		}
	}

	return
}

func (s *OBSyncer) syncAntstorSnapshot(as *v1.AntstorSnapshot) (err error) {
	snapshotMap := ToAntstorSnapshotMapping(s.K8SCluster, as)
	if as.DeletionTimestamp != nil {
		snapshotMap.DeletedAt = int(time.Now().Unix())
		snapshotMap.Status = string(v1.SnapshotStatusDeleted)
		sess := s.engine.NewSession()
		err = MarkDeleteAntstorSnapshot(sess, snapshotMap)
		return
	}

	sess := s.engine.NewSession()
	snapshotFromDB, has, err := GetAntstorSnapshotByName(sess, s.K8SCluster, as.Name)
	if err != nil {
		return
	}

	if !has {
		// Insert Vol to DB
		err = CreateAntstorSnapshot(sess, snapshotMap)
		return
	} else {
		// Compare and Update
		snapshotMap.CreatedAt = snapshotFromDB.CreatedAt
		snapshotMap.UpdatedAt = snapshotFromDB.UpdatedAt

		if !reflect.DeepEqual(snapshotMap, snapshotFromDB) {
			err = UpdateAntstorSnapshot(sess, snapshotMap)
			return
		}
	}

	return
}

func (s *OBSyncer) syncAntstorDataControl(dc *v1.AntstorDataControl) (err error) {
	dcMap := ToAntstorDataControlMapping(s.K8SCluster, dc)
	if dc.DeletionTimestamp != nil {
		dcMap.DeletedAt = int(time.Now().Unix())
		dcMap.Status = string(v1.VolumeStatusDeleted)
		sess := s.engine.NewSession()
		err = MarkDeleteAntstorDataControl(sess, dcMap)
		return
	}

	sess := s.engine.NewSession()
	dcFromDB, has, err := GetAntstorDataControl(sess, s.K8SCluster, dc.Namespace, dc.Name)
	if err != nil {
		return
	}

	if !has {
		// Insert Vol to DB
		err = CreateAntstorDataControl(sess, dcMap)
		return
	} else {
		// Compare and Update
		dcMap.CreatedAt = dcFromDB.CreatedAt
		dcMap.UpdatedAt = dcFromDB.UpdatedAt

		if !reflect.DeepEqual(dcMap, dcFromDB) {
			err = UpdateAntstorDataControl(sess, dcMap)
			return
		}
	}

	return
}

func (s *OBSyncer) syncAntstorVolumeGroup(vg *v1.AntstorVolumeGroup) (err error) {
	vgMap := ToAntstorVolumeGroupMapping(s.K8SCluster, vg)
	if vg.DeletionTimestamp != nil {
		vgMap.DeletedAt = int(time.Now().Unix())
		vgMap.Status = string(v1.VolumeStatusDeleted)
		sess := s.engine.NewSession()
		err = MarkDeleteAntstorVolumeGroup(sess, vgMap)
		return
	}

	sess := s.engine.NewSession()
	vgFromDB, has, err := GetAntstorVolumeGroup(sess, s.K8SCluster, vg.Namespace, vg.Name)
	if err != nil {
		return
	}

	if !has {
		// Insert Vol to DB
		err = CreateAntstorVolumeGroup(sess, vgMap)
		return
	} else {
		// Compare and Update
		vgMap.CreatedAt = vgFromDB.CreatedAt
		vgMap.UpdatedAt = vgFromDB.UpdatedAt

		if !reflect.DeepEqual(vgMap, vgFromDB) {
			err = UpdateAntstorVolumeGroup(sess, vgMap)
			return
		}
	}

	return
}
