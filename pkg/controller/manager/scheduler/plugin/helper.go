package plugin

import (
	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
)

const (
	annSelectedNode = "volume.kubernetes.io/selected-node"
)

func isPVCFullyBound(pvc *corev1.PersistentVolumeClaim) (nodeName string, bound bool) {
	if pvc == nil {
		return
	}
	var has bool
	nodeName, has = pvc.Annotations[annSelectedNode]
	return nodeName, has && pvc.Status.Phase == corev1.ClaimBound
}

func isMustLocalStorageClass(sc *storagev1.StorageClass) bool {
	if sc == nil {
		return false
	}
	return sc.Provisioner == v1.StorageClassProvisioner && sc.Parameters[v1.StorageClassParamPositionAdvice] == string(v1.MustLocal)
}

func isAntstorStorageClass(sc *storagev1.StorageClass) bool {
	if sc == nil {
		return false
	}
	return sc.Provisioner == v1.StorageClassProvisioner
}
