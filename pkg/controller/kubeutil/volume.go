package kubeutil

import (
	"context"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CacheListAnstorVolumeByNodeID
func CacheListAnstorVolumeByNodeID(cli client.Client, nodeID string) (list v1.AntstorVolumeList, err error) {
	// example code: https://github.com/kubernetes-sigs/kubebuilder/blob/master/docs/book/src/cronjob-tutorial/testdata/project/controllers/cronjob_controller.go#L122
	err = cli.List(context.Background(), &list, client.MatchingFields{v1.IndexKeyTargetNodeID: nodeID})
	return
}

// CacheGetAnstorVolumeByUUID is not used
func CacheGetAnstorVolumeByUUID(cli client.Client, uuid string) (vol *v1.AntstorVolume, err error) {
	// example code: https://github.com/kubernetes-sigs/kubebuilder/blob/master/docs/book/src/cronjob-tutorial/testdata/project/controllers/cronjob_controller.go#L122
	var vols v1.AntstorVolumeList
	err = cli.List(context.Background(), &vols, client.MatchingFields{v1.IndexKeyUUID: uuid})
	if len(vols.Items) > 0 {
		vol = vols.Items[0].DeepCopy()
	}
	return
}
