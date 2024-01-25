package syncmeta

import (
	"testing"

	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"lite.io/liteio/pkg/util/mysql"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInsertStoragePool(t *testing.T) {
	sink, err := NewOBSyncer("test-sigma", mysql.ConnectInfo{
		Host:   "localhost",
		Port:   3306,
		User:   "root",
		Passwd: "",
		DB:     "obnvmf",
	}, nil)

	assert.NoError(t, err)
	sp := &v1.StoragePool{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "testNS",
			Name:      "TestName",
		},
		Spec: v1.StoragePoolSpec{
			KernelLVM: v1.KernelLVM{
				Name: "testvg",
				ReservedLVol: []v1.KernelLVol{
					{
						Name:     "reserved-lv",
						SizeByte: 12345,
					},
				},
				Bytes:  12345,
				VgUUID: "xxx-xxx",
			},
		},
	}
	err = sink.Sync(sp)
	assert.NoError(t, err)

	now := metav1.Now()
	sp.DeletionTimestamp = &now
	err = sink.Sync(sp)
	assert.NoError(t, err)

}

func TestInsertAntstorVolume(t *testing.T) {
	sink, err := NewOBSyncer("test-sigma", mysql.ConnectInfo{
		Host:   "localhost",
		Port:   3306,
		User:   "root",
		Passwd: "",
		DB:     "obnvmf",
	}, nil)

	assert.NoError(t, err)
	vol := &v1.AntstorVolume{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "testNS",
			Name:      "TestName",
			Labels: map[string]string{
				"test": "test",
			},
		},
		Spec: v1.AntstorVolumeSpec{
			Uuid:     "xxx-xxx",
			Type:     v1.VolumeTypeKernelLVol,
			SizeByte: 1234,
		},
	}
	err = sink.syncAntstorVolume(vol)
	assert.NoError(t, err)

	now := metav1.Now()
	vol.DeletionTimestamp = &now
	err = sink.syncAntstorVolume(vol)
	assert.NoError(t, err)
}
