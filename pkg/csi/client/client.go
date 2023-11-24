package client

import (
	"context"
	"fmt"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/generated/clientset/versioned"
	uuid "github.com/satori/go.uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rest "k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

var (
	fourMiB uint64             = 1 << 22
	_       AntstorClientIface = &KubeAPIClient{}

	ErrorNotFoundResource = fmt.Errorf("ResourceNotFound")
)

const (
	defaultNamespace = v1.DefaultNamespace
)

type SetNodePublishParamRequest struct {
	ID                string
	HostNodeID        string
	StagingTargetPath string
	TargetPath        string
	CSIVolumeContext  map[string]string
}

type KubeAPIClient struct {
	cli *versioned.Clientset
}

func NewKubeAPIClient(c *rest.Config) (mgr *KubeAPIClient, err error) {
	cli := versioned.NewForConfigOrDie(c)
	mgr = &KubeAPIClient{
		cli: cli,
	}
	return
}

func (cm *KubeAPIClient) CreateSnapshot(snap Snapshot) (snapID string, err error) {
	if snap.Name == "" || snap.Spec.Size == 0 {
		err = fmt.Errorf("invalid request %+v", snap)
		klog.Error(err)
		return
	}
	// requestSize MUST align to 4MiB
	requestSize := uint64(snap.Spec.Size)
	if ret := requestSize % fourMiB; ret > 0 {
		requestSize = requestSize / fourMiB * fourMiB
	}
	if requestSize < fourMiB {
		err = fmt.Errorf("snapshot size too small, should be bigger than 4MiB")
		klog.Error(err)
		return
	}

	// set uuid
	if snap.Spec.Uuid == "" {
		snap.Spec.Uuid = uuid.NewV4().String()
	}
	snap.Labels[v1.SnapUuidLabelKey] = snap.Spec.Uuid

	klog.Infof("Creating snapshot %s with size %d", snap.Name, requestSize)
	snapshot, err := cm.cli.VolumeV1().AntstorSnapshots(defaultNamespace).Create(context.Background(), &snap, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		return
	}

	snapID = snapshot.Spec.Uuid
	return
}

func (cm *KubeAPIClient) GetSnapshotByID(id string) (snapshot *Snapshot, err error) {
	labelSelector := fmt.Sprintf("%s=%s", v1.SnapUuidLabelKey, id)
	list, err := cm.cli.VolumeV1().AntstorSnapshots(defaultNamespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		klog.Error(err)
		return
	}

	if len(list.Items) == 0 {
		err = ErrorNotFoundResource
		return
	}

	snapshot = &list.Items[0]
	return
}

func (cm *KubeAPIClient) GetSnapshotByName(ns, name string) (snapshot *Snapshot, err error) {
	snapshot, err = cm.cli.VolumeV1().AntstorSnapshots(ns).Get(context.Background(), name, metav1.GetOptions{})
	return
}

func (cm *KubeAPIClient) DeleteSnapshot(snapID string) (err error) {
	if snapID == "" {
		err = fmt.Errorf("invalid empty snapID")
		return
	}

	snap, err := cm.GetSnapshotByID(snapID)
	if err != nil {
		if err == ErrorNotFoundResource {
			klog.Infof("snapshot %s may be already deleted", snapID)
			return nil
		}
		klog.Error(err)
		return
	}

	err = cm.cli.VolumeV1().AntstorSnapshots(defaultNamespace).Delete(context.Background(), snap.Name, metav1.DeleteOptions{})
	if err != nil {
		klog.Error(err)
	}
	return
}

func (cm *KubeAPIClient) GetStoragePoolByName(ns, name string) (sp *StoragePool, err error) {
	sp, err = cm.cli.VolumeV1().StoragePools(ns).Get(context.Background(), name, metav1.GetOptions{})
	return
}
