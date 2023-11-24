package syncmeta

import (
	"encoding/json"
	"fmt"
	"time"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"github.com/go-xorm/xorm"
)

type AntstorVolumeGroupMapping struct {
	// id
	ID          int    `xorm:"pk autoincr 'id'"`
	Name        string `xorm:"name"`
	ClusterName string `xorm:"cluster_name"`

	UUID string `xorm:"uid"`
	Size int64  `xorm:"size"`

	DataControlName string `xorm:"data_control_name"`

	Volumes string `xorm:"volumes"`

	// status
	Status    string `xorm:"status"`
	CreatedAt int    `xorm:"created_at"`
	UpdatedAt int    `xorm:"updated_at"`
	DeletedAt int    `xorm:"deleted_at"`
}

func (spm *AntstorVolumeGroupMapping) TableName() string {
	return "antstor_volgroup"
}

func ToAntstorVolumeGroupMapping(clusterName string, avg *v1.AntstorVolumeGroup) (avm *AntstorVolumeGroupMapping) {
	var (
		dataControlName string
		volumesJSON     []byte
	)

	dataControlName = avg.Labels[v1.DataControlNameKey]
	volumesJSON, _ = json.Marshal(avg.Spec.Volumes)

	avm = &AntstorVolumeGroupMapping{
		ClusterName: clusterName,
		Name:        avg.Name,

		UUID:            avg.Spec.Uuid,
		Size:            avg.Spec.TotalSize,
		DataControlName: dataControlName,
		Volumes:         string(volumesJSON),

		Status: string(avg.Status.Status),
	}
	return
}

// SQL for datacontrol

func MarkDeleteAntstorVolumeGroup(sess *xorm.Session, avm *AntstorVolumeGroupMapping) (err error) {
	var (
		delAt  int
		status string
	)

	if avm == nil {
		err = fmt.Errorf("AntstorVolumeGroupMapping is nil")
		return
	}

	delAt = avm.DeletedAt
	status = avm.Status
	if delAt == 0 {
		delAt = int(time.Now().Unix())
	}

	_, err = sess.Cols("status", "deleted_at").
		Update(&AntstorVolumeGroupMapping{Status: status, DeletedAt: delAt},
			&AntstorVolumeGroupMapping{ClusterName: avm.ClusterName, Name: avm.Name})
	return
}

func GetAntstorVolumeGroup(sess *xorm.Session, cluster, ns, name string) (adm *AntstorVolumeGroupMapping, has bool, err error) {
	adm = &AntstorVolumeGroupMapping{}
	has, err = sess.Where("cluster_name=? and name=?", cluster, name).Get(adm)
	return
}

func CreateAntstorVolumeGroup(sess *xorm.Session, adm *AntstorVolumeGroupMapping) (err error) {
	ts := time.Now().Unix()
	adm.CreatedAt = int(ts)
	adm.UpdatedAt = int(ts)
	_, err = sess.InsertOne(adm)
	return
}

func UpdateAntstorVolumeGroup(sess *xorm.Session, adm *AntstorVolumeGroupMapping) (err error) {
	ts := time.Now().Unix()
	adm.UpdatedAt = int(ts)

	_, err = sess.Cols("uid", "size",
		"data_control_name", "volumes",
		"status", "updated_at").
		Update(adm, &AntstorVolumeGroupMapping{ClusterName: adm.ClusterName, Name: adm.Name})
	return
}
