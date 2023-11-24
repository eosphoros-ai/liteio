package kubeutil

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
)

type PVFetcherUpdaterIface interface {
	GetByName(name string) (pv *corev1.PersistentVolume, err error)
	SetTargetNodeName(pvName, targetNode string) (err error)
}

type pvUpdater struct {
	// kube client
	kubeCli kubernetes.Interface
}

func NewPVUppdater(kubeCli kubernetes.Interface) *pvUpdater {
	return &pvUpdater{
		kubeCli: kubeCli,
	}
}

func (pu *pvUpdater) GetByName(name string) (pv *corev1.PersistentVolume, err error) {
	return pu.kubeCli.CoreV1().PersistentVolumes().Get(context.Background(), name, metav1.GetOptions{})
}

func (pu *pvUpdater) SetTargetNodeName(pvName, targetNode string) (err error) {
	pv, err := pu.GetByName(pvName)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.Warningf("cannot find PV by name %s, so not add targetNode to PV labels", pvName)
			return nil
		}
		return
	}

	if pv.Labels == nil {
		pv.Labels = make(map[string]string)
	}

	if val := pv.Labels[v1.PVTargetNodeNameLabelKey]; val != targetNode {
		pv.Labels[v1.PVTargetNodeNameLabelKey] = targetNode
		_, err = pu.kubeCli.CoreV1().PersistentVolumes().Update(context.Background(), pv, metav1.UpdateOptions{})
	}

	return
}
