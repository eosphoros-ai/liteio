package syncmeta

import (
	"fmt"
	"time"

	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"github.com/go-xorm/xorm"
)

type AntstorDataControlMapping struct {
	// id
	ID          int    `xorm:"pk autoincr 'id'"`
	Name        string `xorm:"name"`
	ClusterName string `xorm:"cluster_name"`

	UUID         string `xorm:"uid"`
	RaidLevel    string `xorm:"raid_level"`
	TargetNodeId string `xorm:"t_node_id"`
	EngineType   string `xorm:"engine_type"`
	LvmVG        string `xorm:"lvm_vg"`
	LvmLV        string `xorm:"lvm_lv"`

	// pvc
	PvcNS   string `xorm:"pvc_ns"`
	PvcName string `xorm:"pvc_name"`

	// host node
	HostNodeID       string `xorm:"h_node_id"`
	HostNodeIP       string `xorm:"h_node_ip"`
	HostNodeHostname string `xorm:"h_node_hostname"`
	// pod_name
	PodNS   string `xorm:"pod_ns"`
	PodName string `xorm:"pod_name"`
	// status
	Status    string `xorm:"status"`
	CreatedAt int    `xorm:"created_at"`
	UpdatedAt int    `xorm:"updated_at"`
	DeletedAt int    `xorm:"deleted_at"`
}

func (spm *AntstorDataControlMapping) TableName() string {
	return "antstor_datacontrol"
}

func ToAntstorDataControlMapping(clusterName string, ad *v1.AntstorDataControl) (adm *AntstorDataControlMapping) {
	var (
		podNS, podName string
		vg, lv         string
		pvcNS, pvcName string
	)
	if ad.Status.CSINodePubParams != nil && ad.Status.CSINodePubParams.CSIVolumeContext != nil {
		podNS = ad.Status.CSINodePubParams.CSIVolumeContext[v1.VolumeContextKeyPodNS]
		podName = ad.Status.CSINodePubParams.CSIVolumeContext[v1.VolumeContextKeyPodName]
	}
	if ad.Spec.LVM != nil {
		vg = ad.Spec.LVM.VG
		lv = ad.Spec.LVM.LVol
	}
	if ad.Labels != nil {
		pvcName = ad.Labels[v1.VolumeContextKeyPvcName]
		pvcNS = ad.Labels[v1.VolumeContextKeyPvcNS]
	}

	adm = &AntstorDataControlMapping{
		ClusterName: clusterName,
		Name:        ad.Name,

		UUID:         ad.Spec.UUID,
		RaidLevel:    string(ad.Spec.Raid.Level),
		TargetNodeId: ad.Spec.TargetNodeId,
		EngineType:   string(ad.Spec.EngineType),
		LvmVG:        vg,
		LvmLV:        lv,

		PvcNS:   pvcNS,
		PvcName: pvcName,

		HostNodeID:       ad.Spec.HostNode.ID,
		HostNodeIP:       ad.Spec.HostNode.IP,
		HostNodeHostname: ad.Spec.HostNode.Hostname,

		PodNS:   podNS,
		PodName: podName,

		Status: string(ad.Status.Status),
	}
	return
}

// SQL for datacontrol

func MarkDeleteAntstorDataControl(sess *xorm.Session, adm *AntstorDataControlMapping) (err error) {
	var (
		delAt  int
		status string
	)

	if adm == nil {
		err = fmt.Errorf("AntstorDataControlMapping is nil")
		return
	}

	delAt = adm.DeletedAt
	status = adm.Status
	if delAt == 0 {
		delAt = int(time.Now().Unix())
	}

	_, err = sess.Cols("status", "deleted_at").
		Update(&AntstorDataControlMapping{Status: status, DeletedAt: delAt},
			&AntstorDataControlMapping{ClusterName: adm.ClusterName, Name: adm.Name})
	return
}

func GetAntstorDataControl(sess *xorm.Session, cluster, ns, name string) (adm *AntstorDataControlMapping, has bool, err error) {
	adm = &AntstorDataControlMapping{}
	has, err = sess.Where("cluster_name=? and name=?", cluster, name).Get(adm)
	return
}

func CreateAntstorDataControl(sess *xorm.Session, adm *AntstorDataControlMapping) (err error) {
	ts := time.Now().Unix()
	adm.CreatedAt = int(ts)
	adm.UpdatedAt = int(ts)
	_, err = sess.InsertOne(adm)
	return
}

func UpdateAntstorDataControl(sess *xorm.Session, adm *AntstorDataControlMapping) (err error) {
	ts := time.Now().Unix()
	adm.UpdatedAt = int(ts)

	_, err = sess.Cols("uid", "pvc_ns", "pvc_name",
		"t_node_id", "h_node_id", "h_node_ip", "h_node_hostname",
		"raid_level", "engine_type", "lvm_vg", "lvm_lv",
		"pod_ns", "pod_name",
		"status", "updated_at").
		Update(adm, &AntstorDataControlMapping{ClusterName: adm.ClusterName, Name: adm.Name})
	return
}
