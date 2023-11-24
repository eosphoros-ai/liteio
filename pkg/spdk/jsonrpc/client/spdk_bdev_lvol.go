package client

import "encoding/json"

type SpdkLvolIface interface {
	// bdev_lvol_get_lvstores
	BdevLVolGetLVStores(req BdevLVolGetLVStoresReq) (list []LVStoreInfo, err error)
	// bdev_lvol_create_lvstore
	BdevLVolCreateLVStore(req BdevLVolCreateLVStoreReq) (uuid string, err error)

	// bdev_lvol_create
	BdevLVolCreate(req BdevLVolCreateReq) (uuid string, err error)
	// bdev_lvol_delete
	BdevLVolDelete(req BdevLVolDeleteReq) (ok bool, err error)
	// bdev_lvol_resize
	BdevLVolResize(req BdevLVolResizeReq) (ok bool, err error)

	// bdev_lvol_snapshot
	BdevLVolSnapshot(req BdevLVolSnapshotReq) (uuid string, err error)
	// bdev_lvol_clone
	BdevLVolClone(req BdevLVolCloneReq) (uuid string, err error)
	// bdev_lvol_inflate
	BdevLVolInflate(req BdevLVolInflateReq) (ok bool, err error)
}

type BdevLVolGetLVStoresReq struct {
	// optional
	UUID    string `json:"uuid,omitempty"`
	LvsName string `json:"lvs_name,omitempty"`
}

type LVStoreInfo struct {
	UUID              string `json:"uuid"`
	Name              string `json:"name"`
	BaseBdev          string `json:"base_bdev"`
	FreeClusters      int    `json:"free_clusters"`
	ClusterSize       int    `json:"cluster_size"`
	TotalDataClusters int    `json:"total_data_clusters"`
	BlockSize         int    `json:"block_size"`
}

type BdevLVolCreateLVStoreReq struct {
	// required
	BdevName string `json:"bdev_name"`
	LvsName  string `json:"lvs_name"`
	// optional
	// Cluster size of the logical volume store in bytes
	ClusterSize int `json:"cluster_sz,omitempty"`
	// Change clear method for data region. Available: none, unmap (default), write_zeroes
	ClearMethod ClearMethod `json:"clear_method,omitempty"`
}

type BdevLVolCreateReq struct {
	// required
	LVolName string `json:"lvol_name"`
	// Desired size of logical volume in bytes; Size will be rounded up to a multiple of cluster size. Either uuid or lvs_name must be specified, but not both. lvol_name will be used in the alias of the created logical volume.
	Size int `json:"size"`

	// optional
	ThinProvision bool        `json:"thin_provision,omitempty"`
	UUID          string      `json:"uuid,omitempty"`
	LvsName       string      `json:"lvs_name,omitempty"`
	ClearMethod   ClearMethod `json:"clear_method,omitempty"`
}

type BdevLVolDeleteReq struct {
	Name string `json:"name"`
}

type BdevLVolResizeReq struct {
	Name string `json:"name"`
	Size uint64 `json:"size"`
}

type BdevLVolSnapshotReq struct {
	LVolName     string `json:"lvol_name"`
	SnapshotName string `json:"snapshot_name"`
}

type BdevLVolCloneReq struct {
	SnapshotName string `json:"snapshot_name"`
	CloneName    string `json:"clone_name"`
}

type BdevLVolInflateReq struct {
	Name string `json:"name"`
}

func (s *SPDK) BdevLVolCreateLVStore(req BdevLVolCreateLVStoreReq) (uuid string, err error) {
	result, err := s.rawCli.Call("bdev_lvol_create_lvstore", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(result, &uuid)
	return
}

func (s *SPDK) BdevLVolGetLVStores(req BdevLVolGetLVStoresReq) (list []LVStoreInfo, err error) {
	result, err := s.rawCli.Call("bdev_lvol_get_lvstores", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(result, &list)
	return
}

// bdev_lvol_create
func (s *SPDK) BdevLVolCreate(req BdevLVolCreateReq) (uuid string, err error) {
	result, err := s.rawCli.Call("bdev_lvol_create", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(result, &uuid)
	return
}

func (s *SPDK) BdevLVolDelete(req BdevLVolDeleteReq) (ok bool, err error) {
	result, err := s.rawCli.Call("bdev_lvol_delete", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(result, &ok)
	return
}

func (s *SPDK) BdevLVolResize(req BdevLVolResizeReq) (ok bool, err error) {
	result, err := s.rawCli.Call("bdev_lvol_resize", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(result, &ok)
	return
}

func (s *SPDK) BdevLVolSnapshot(req BdevLVolSnapshotReq) (uuid string, err error) {
	result, err := s.rawCli.Call("bdev_lvol_snapshot", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(result, &uuid)
	return
}

func (s *SPDK) BdevLVolClone(req BdevLVolCloneReq) (uuid string, err error) {
	result, err := s.rawCli.Call("bdev_lvol_clone", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(result, &uuid)
	return
}

func (s *SPDK) BdevLVolInflate(req BdevLVolInflateReq) (ok bool, err error) {
	result, err := s.rawCli.Call("bdev_lvol_inflate", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(result, &ok)
	return
}
