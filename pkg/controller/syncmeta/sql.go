package syncmeta

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/didi/gendry/builder"
	"github.com/go-xorm/xorm"
)

// SQL for AntstorVolumeBrief

func ListAntstorVolumeBriefByCondition(sess *xorm.Session, where Condition) (list []*AntstorVolumeBriefMapping, err error) {
	var sql string
	var table = "antstor_volume"
	selectFields := []string{"id", "cluster_name", "name", "status", "created_at", "updated_at", "deleted_at"}
	sql, values, err := builder.BuildSelect(table, where, selectFields)
	if err != nil {
		return
	}
	sql = replaceArgs(sql, values)
	quoteStringArg(values)
	sql = fmt.Sprintf(sql, values...)

	err = sess.SQL(sql).Find(&list)
	return
}

// SQL for StoragePoolBrief

func ListStoragePoolBriefByCondition(sess *xorm.Session, where Condition) (list []*StoragePoolBriefMapping, err error) {
	var sql string
	var table = "storage_pool"
	selectFields := []string{"id", "cluster_name", "name", "status", "created_at", "updated_at", "deleted_at"}
	sql, values, err := builder.BuildSelect(table, where, selectFields)
	if err != nil {
		return
	}
	sql = replaceArgs(sql, values)
	quoteStringArg(values)
	sql = fmt.Sprintf(sql, values...)

	err = sess.SQL(sql).Find(&list)
	return
}

// SQL for StoragePool

func GetStoragePool(sess *xorm.Session, cluster, name string) (spm *StoragePoolMapping, has bool, err error) {
	spm = &StoragePoolMapping{}
	has, err = sess.Where("cluster_name=? and name=?", cluster, name).Get(spm)
	return
}

func GetStoragePoolByName(sess *xorm.Session, name string) (spm *StoragePoolMapping, has bool, err error) {
	spm = &StoragePoolMapping{}
	has, err = sess.Where("name=?", name).Get(spm)
	return
}

func CreateStoragePool(sess *xorm.Session, spm *StoragePoolMapping) (err error) {
	ts := time.Now().Unix()
	spm.CreatedAt = int(ts)
	spm.UpdatedAt = int(ts)
	_, err = sess.InsertOne(spm)
	return
}

func UpdateStoragePool(sess *xorm.Session, spm *StoragePoolMapping) (err error) {
	ts := time.Now().Unix()
	spm.UpdatedAt = int(ts)
	_, err = sess.Cols("vg_type", "vg_name", "reserved_vol", "total_size",
		"node_ip", "node_hostname", "free_size", "status", "updated_at").
		Update(spm, &StoragePoolMapping{ClusterName: spm.ClusterName, Name: spm.Name})
	return
}

func MarkDeleteStoragePool(sess *xorm.Session, spm *StoragePoolMapping) (err error) {
	var (
		delAt  int
		status string
	)

	if spm == nil {
		err = fmt.Errorf("storagepool is nil")
		return
	}

	delAt = spm.DeletedAt
	status = spm.Status
	if delAt == 0 {
		delAt = int(time.Now().Unix())
	}

	_, err = sess.Cols("status", "deleted_at").
		Update(&StoragePoolMapping{Status: status, DeletedAt: delAt},
			&StoragePoolMapping{ClusterName: spm.ClusterName, Name: spm.Name})
	return
}

// SQL for AntstorVolume

func MarkDeleteAntstorVolume(sess *xorm.Session, avm *AntstorVolumeMapping) (err error) {
	var (
		delAt  int
		status string
	)

	if avm == nil {
		err = fmt.Errorf("antstorvolume is nil")
		return
	}

	delAt = avm.DeletedAt
	status = avm.Status
	if delAt == 0 {
		delAt = int(time.Now().Unix())
	}

	_, err = sess.Cols("status", "deleted_at").
		Update(&AntstorVolumeMapping{Status: status, DeletedAt: delAt},
			&AntstorVolumeMapping{ClusterName: avm.ClusterName, Name: avm.Name})
	return
}

func GetAntstorVolume(sess *xorm.Session, cluster, ns, name string) (avm *AntstorVolumeMapping, has bool, err error) {
	avm = &AntstorVolumeMapping{}
	has, err = sess.Where("cluster_name=? and name=?", cluster, name).Get(avm)
	return
}

func GetAntstorVolumeByUUID(sess *xorm.Session, uuid string) (avm *AntstorVolumeMapping, has bool, err error) {
	avm = &AntstorVolumeMapping{}
	has, err = sess.Where("uuid=?", uuid).Get(avm)
	return
}

func CreateAntstorVolume(sess *xorm.Session, avm *AntstorVolumeMapping) (err error) {
	ts := time.Now().Unix()
	avm.CreatedAt = int(ts)
	avm.UpdatedAt = int(ts)
	_, err = sess.InsertOne(avm)
	return
}

func UpdateAntstorVolume(sess *xorm.Session, avm *AntstorVolumeMapping) (err error) {
	ts := time.Now().Unix()
	avm.UpdatedAt = int(ts)
	_, err = sess.Cols("uid", "labels", "pvc_ns", "pvc_name",
		"lvol_dev_path", "lvol_name", "is_thin", "position_advice", "size",
		"t_node_id", "h_node_id", "h_node_ip", "h_node_hostname",
		"spdk_subsys_nqn", "spdk_svc_id", "spdk_sn", "spdk_trans_type", "spdk_bdev_name", "spdk_ns_uuid", "spdk_address",
		"csi_staging_path", "csi_publish_path", "pod_ns", "pod_name",
		"status", "updated_at").
		Update(avm, &AntstorVolumeMapping{ClusterName: avm.ClusterName, Name: avm.Name})
	return
}

func GetAntstorVolumeExt(sess *xorm.Session, volID int) (avem *AntstorVolumeExtMapping, has bool, err error) {
	avem = &AntstorVolumeExtMapping{}
	has, err = sess.Where("vol_id=?", volID).Get(avem)
	return
}

func CreateAntstorVolumeExt(sess *xorm.Session, avm *AntstorVolumeExtMapping) (err error) {
	if avm == nil {
		return
	}
	ts := time.Now().Unix()
	avm.CreatedAt = int(ts)
	avm.UpdatedAt = int(ts)
	_, err = sess.InsertOne(avm)
	return
}

// 根据name获取Snapshot
func GetAntstorSnapshotByName(sess *xorm.Session, cluster, name string) (asm *AntstorSnapshotMapping, has bool, err error) {
	asm = &AntstorSnapshotMapping{}
	has, err = sess.Where("cluster_name=? and name=?", cluster, name).Get(asm)
	return
}

func GetAntstorSnapshotByUUID(sess *xorm.Session, uuid string) (asm *AntstorSnapshotMapping, has bool, err error) {
	asm = &AntstorSnapshotMapping{}
	has, err = sess.Where("uuid=?", uuid).Get(asm)
	return
}

func CreateAntstorSnapshot(sess *xorm.Session, asm *AntstorSnapshotMapping) (err error) {
	ts := time.Now().Unix()
	asm.CreatedAt = int(ts)
	asm.UpdatedAt = int(ts)
	_, err = sess.InsertOne(asm)
	return
}

func UpdateAntstorSnapshot(sess *xorm.Session, asm *AntstorSnapshotMapping) (err error) {
	ts := time.Now().Unix()
	asm.UpdatedAt = int(ts)
	_, err = sess.Cols("vol_type", "lvol_dev_path", "lvol_name", "size", "status", "updated_at").
		Update(asm, &AntstorSnapshotMapping{
			ClusterName:   asm.ClusterName,
			OriginVolName: asm.OriginVolName,
			OriginVolNs:   asm.OriginVolNs,
		})
	return
}

func MarkDeleteAntstorSnapshot(sess *xorm.Session, asm *AntstorSnapshotMapping) (err error) {
	var (
		delAt  int
		status string
	)

	if asm == nil {
		err = fmt.Errorf("antstor_snapshot is nil")
		return
	}

	delAt = asm.DeletedAt
	status = asm.Status
	if delAt == 0 {
		delAt = int(time.Now().Unix())
	}

	_, err = sess.Cols("status", "deleted_at").
		Update(&AntstorSnapshotMapping{Status: status, DeletedAt: delAt},
			&AntstorSnapshotMapping{
				ClusterName:   asm.ClusterName,
				OriginVolName: asm.OriginVolName,
				OriginVolNs:   asm.OriginVolNs,
			})
	return
}

type RawSQL string
type Condition map[string]interface{}

// TODO: raw SQL is about to be deprecated
const (
	SQL_CreateStoragePool_Tpl = `
insert into storage_pool (cluster_name, name, vg_type, vg_name, reserved_vol, total_size, free_size, 
node_id, node_ip, node_hostname, status, created_at, updated_at) VALUES (%s, %s, %s, %s, %s, %d, %d,
%s, %s, %s, %s, UNIX_TIMESTAMP(), UNIX_TIMESTAMP())`

	SQL_UpdateStoragePool_Tpl = `
update storage_pool set vg_type=%s, vg_name=%s, reserved_vol=%s, total_size=%d, free_size=%d,
status=%s, updated_at=UNIX_TIMESTAMP()  where cluster_name=%s and name=%s`

	SQL_CreateAntstorVolume_Tpl = `
insert into antstor_volume (cluster_name, name, uid, labels, pvc_ns, pvc_name, 
lvol_dev_path, lvol_name, is_thin, position_advice, size,
t_node_id, h_node_id, h_node_ip, h_node_hostname, 
spdk_subsys_nqn, spdk_svc_id, spdk_sn, spdk_trans_type, spdk_bdev_name, spdk_ns_uuid, spdk_address,
csi_staging_path, csi_publish_path, pod_ns, pod_name, 
status, created_at, updated_at) VALUES (%s, %s, %s, %s, %s, %s,
%s, %s, %d, %s, %d,
%s, %s, %s, %s,
%s, %s, %s, %s, %s, %s, %s,
%s, %s, %s, %s,
%s, UNIX_TIMESTAMP(), UNIX_TIMESTAMP())`

	SQL_UpdateAntstorVolume_Tpl = `
update antstor_volume set uid=%s, labels=%s, pvc_ns=%s, pvc_name=%s,
lvol_dev_path=%s, lvol_name=%s, is_thin=%d, position_advice=%s, size=%d,
t_node_id=%s, h_node_id=%s, h_node_ip=%s, h_node_hostname=%s,
spdk_subsys_nqn=%s, spdk_svc_id=%s, spdk_sn=%s, spdk_trans_type=%s, spdk_bdev_name=%s, spdk_ns_uuid=%s, spdk_address=%s,
csi_staging_path=%s, csi_publish_path=%s, pod_ns=%s, pod_name=%s, 
status=%s, updated_at=UNIX_TIMESTAMP() where cluster_name=%s and name=%s`
)

func quoteStringArg(args []interface{}) {
	for i := 0; i < len(args); i++ {
		switch s := args[i].(type) {
		case string:
			args[i] = strconv.Quote(s)
		case RawSQL:
			args[i] = string(s)
		}
	}
}

func replaceArgs(sql string, args []interface{}) string {
	for _, item := range args {
		switch item.(type) {
		case string, RawSQL:
			sql = strings.Replace(sql, "?", "%s", 1)
		case float64, float32:
			sql = strings.Replace(sql, "?", "%f", 1)
		default:
			sql = strings.Replace(sql, "?", "%d", 1)
		}
	}
	return sql
}
