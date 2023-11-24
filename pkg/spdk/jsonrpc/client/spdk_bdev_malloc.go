package client

import "encoding/json"

type SpdkMallocIface interface {
	// bdev_malloc_create
	CreateBdevMalloc(req CreateBdevMallocReq) (name string, err error)
	// bdev_malloc_delete
	DeleteBdevMalloc(req DeleteBdevMallocReq) (ok bool, err error)
}

type CreateBdevMallocReq struct {
	// required
	BlockSize int `json:"block_size"`
	NumBlocks int `json:"num_blocks"`
	// optional
	Name              string `json:"name,omitempty"`
	UUID              string `json:"uuid,omitempty"`
	OptimalIoBoundary int    `json:"optimal_io_boundary,omitempty"`
	MdSize            int    `json:"md_size,omitempty"`
	MdInterleave      bool   `json:"md_interleave,omitempty"`
	DifType           int    `json:"dif_type,omitempty"`
	DifIsHeadOfMd     bool   `json:"dif_is_head_of_md,omitempty"`
}

type DeleteBdevMallocReq struct {
	// required
	Name string `json:"name"`
}

func (s *SPDK) CreateBdevMalloc(req CreateBdevMallocReq) (name string, err error) {
	bs, err := s.rawCli.Call("bdev_malloc_create", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(bs, &name)
	return
}

func (s *SPDK) DeleteBdevMalloc(req DeleteBdevMallocReq) (ok bool, err error) {
	bs, err := s.rawCli.Call("bdev_malloc_delete", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(bs, &ok)
	return
}
