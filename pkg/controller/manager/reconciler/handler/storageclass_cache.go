package handler

import (
	"context"
	"sync"

	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type CachedStorageClassIface interface {
	GetByName(name string) (sc *storagev1.StorageClass, err error)
}

type CachedStorageClass struct {
	kubeCli kubernetes.Interface
	scMap   map[string]*storagev1.StorageClass
	lock    sync.Mutex
}

func NewCachedStorageClass(kubeCli kubernetes.Interface) *CachedStorageClass {
	return &CachedStorageClass{
		kubeCli: kubeCli,
		scMap:   make(map[string]*storagev1.StorageClass),
	}
}

func (c *CachedStorageClass) GetByName(name string) (sc *storagev1.StorageClass, err error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if val, has := c.scMap[name]; has {
		return val, nil
	}

	sc, err = c.kubeCli.StorageV1().StorageClasses().Get(context.Background(), name, metav1.GetOptions{})
	if err == nil {
		c.scMap[name] = sc
	}
	return
}
