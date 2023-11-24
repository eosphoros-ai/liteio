package manager

import (
	"context"
	"testing"

	fakev1 "code.alipay.com/dbplatform/node-disk-controller/pkg/generated/clientset/versioned/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetStoragePool(t *testing.T) {
	storeCli := fakev1.NewSimpleClientset()
	cli := storeCli.VolumeV1().StoragePools(AntstorDefaultNamespace)
	pool, err := cli.Get(context.Background(), "test-storagepool", metav1.GetOptions{})
	t.Log(err, pool)
}
