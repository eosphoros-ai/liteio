package kubeutil

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	PatchOpRemove = "remove"
)

type PatchOP string

type JSONPathItem struct {
	Op   string `json:"op"`
	Path string `json:"path"`
}

type StoragePoolUpdater interface {
	MergePatchStoragePool(old, new *v1.StoragePool) (err error)
	UpdateStoragePoolStatus(original *v1.StoragePool, newStatus v1.PoolStatus) (err error)
	// TriggerLabelEvent patches pool's Labels with evKey=true and merge labels
	TriggerLabelEvent(pool *v1.StoragePool, eventKey string, mergeLabels map[string]string) (err error)
	RemoveTriggerLabel(pool *v1.StoragePool, eventKey string) (err error)
	SavePoolLocalStorageMark(pool *v1.StoragePool, size uint64) (err error)
	// RemovePoolLocalStorageMark(pool *v1.StoragePool) (err error)
}

type StoragePoolUtil struct {
	cli client.Client
}

func NewStoragePoolUtil(cli client.Client) *StoragePoolUtil {
	return &StoragePoolUtil{
		cli: cli,
	}
}

func (su *StoragePoolUtil) SavePoolLocalStorageMark(pool *v1.StoragePool, size uint64) (err error) {
	sizeKey := strings.ReplaceAll(v1.PoolLocalStorageBytesKey, "/", "~1")

	patchStr := fmt.Sprintf(`[{"op":"add", "path":"/metadata/labels/%s", "value":"%d"}]`, sizeKey, size)
	if _, has := pool.Labels[v1.PoolLocalStorageBytesKey]; has {
		patchStr = fmt.Sprintf(`[{"op":"replace", "path":"/metadata/labels/%s", "value":"%d"}]`, sizeKey, size)
	}

	data := []byte(patchStr)

	patch := client.RawPatch(types.JSONPatchType, data)
	err = su.cli.Patch(context.Background(), pool, patch)

	return
}

/*
// RemovePoolLocalStorageMark remove storage pool's labels
func (su *StoragePoolUtil) RemovePoolLocalStorageMark(pool *v1.StoragePool) (err error) {
	var jsonPatch = make([]JSONPathItem, 0, 2)

	evkey := strings.ReplaceAll(v1.PoolEventSyncNodeLocalStorageKey, "/", "~1")
	sizeKey := strings.ReplaceAll(v1.PoolLocalStorageBytesKey, "/", "~1")

	jsonPatch = append(jsonPatch, JSONPathItem{
		Op:   PatchOpRemove,
		Path: fmt.Sprintf("/metadata/labels/%s", evkey),
	})

	if _, has := pool.Labels[v1.PoolLocalStorageBytesKey]; has {
		jsonPatch = append(jsonPatch, JSONPathItem{
			Op:   PatchOpRemove,
			Path: fmt.Sprintf("/metadata/labels/%s", sizeKey),
		})
	}

	data, _ := json.Marshal(jsonPatch)


		// if key not exists, patching will fail
		// patchStr := fmt.Sprintf(`[{"op": "remove", "path": "/metadata/labels/%s"},
		// 	{"op":"remove", "path":"/metadata/labels/%s"}]`, evkey, sizeKey)
		// data := []byte(patchStr)


	patch := client.RawPatch(types.JSONPatchType, data)
	err = su.cli.Patch(context.Background(), pool, patch)

	return
}
*/

// TriggerLabelEvent patches pool's Labels with evKey=true and merge labels
func (su *StoragePoolUtil) TriggerLabelEvent(pool *v1.StoragePool, evKey string, mergeLabels map[string]string) (err error) {
	// Escape JSON Patch Path for Key with slash
	// The answer you're looking for is documented in RFC6901, section 3. "~"(tilde) is encoded as "~0" and "/"(forward slash) is encoded as "~1".
	// https://github.com/json-patch/json-patch-tests/issues/42
	/*
		key := strings.ReplaceAll(evKey, "/", "~1")
		data := []byte(fmt.Sprintf(`[{"op": "add", "path": "/metadata/labels/%s", "value": "%s"}]`, key, "true"))
		patch := client.RawPatch(types.JSONPatchType, data)
		err = su.cli.Patch(context.Background(), pool, patch)
	*/

	mergePatch := client.MergeFrom(pool.DeepCopy())
	if pool.Labels == nil {
		pool.Labels = make(map[string]string)
	}
	pool.Labels[evKey] = "true"
	for key, val := range mergeLabels {
		pool.Labels[key] = val
	}
	err = su.cli.Patch(context.Background(), pool, mergePatch)

	return
}

func (su *StoragePoolUtil) RemoveTriggerLabel(pool *v1.StoragePool, evKey string) (err error) {
	key := strings.ReplaceAll(evKey, "/", "~1")
	data := []byte(fmt.Sprintf(`[{"op": "remove", "path": "/metadata/labels/%s"}]`, key))
	patch := client.RawPatch(types.JSONPatchType, data)

	err = su.cli.Patch(context.Background(), pool, patch)
	return
}

// UpdateStoragePoolStatus may fail, if StoragePool was updated before calling this function. because the ResourceVersion field is changed.
func (su *StoragePoolUtil) UpdateStoragePoolStatus(original *v1.StoragePool, newStatus v1.PoolStatus) (err error) {
	patch := client.MergeFrom(original.DeepCopy())
	original.Status.Status = newStatus
	return su.cli.Status().Patch(context.Background(), original, patch)
}

// MergePatchStoragePool uses MergeFrom to patch StoragePool
func (su *StoragePoolUtil) MergePatchStoragePool(old, new *v1.StoragePool) (err error) {
	patch := client.MergeFrom(old)
	return su.cli.Patch(context.Background(), new, patch)
}

func IsNodeInfoDifferent(a, b v1.NodeInfo) (diff bool) {
	diff = a.ID != b.ID || a.IP != b.IP || a.Hostname != b.Hostname
	if diff {
		return
	}

	diff = !reflect.DeepEqual(a.Labels, b.Labels)
	return
}

// CRD not support strategic merge patch
func GenerateMergePatchOfStoragePool(old, new *v1.StoragePool) (data []byte, err error) {
	oldData, err := json.Marshal(old)
	if err != nil {
		klog.Errorf("marshal StoragePool %s failed, %v", old.Name, err)
		return nil, err
	}
	newData, err := json.Marshal(new)
	if err != nil {
		klog.Errorf("marshal StoragePool %s failed, %v", new.Name, err)
		return nil, err
	}
	return strategicpatch.CreateTwoWayMergePatch(oldData, newData, &v1.StoragePool{})
}
