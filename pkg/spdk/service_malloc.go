package spdk

import (
	spdkrpc "lite.io/liteio/pkg/spdk/jsonrpc/client"
	"k8s.io/klog/v2"
)

type CreateBdevMallocReq = spdkrpc.CreateBdevMallocReq
type DeleteBdevMallocReq = spdkrpc.DeleteBdevMallocReq

type MallocServiceIface interface {
	CreateMemBdev(req CreateBdevMallocReq) (err error)
	DeleteMemBdev(req DeleteBdevMallocReq) (ok bool, err error)
}

func (ss *SpdkService) CreateMemBdev(req CreateBdevMallocReq) (err error) {
	ss.cli, err = ss.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	klog.Infof("creating mem_bdev")
	_, err = ss.cli.CreateBdevMalloc(req)
	return
}

func (ss *SpdkService) DeleteMemBdev(req DeleteBdevMallocReq) (ok bool, err error) {
	ss.cli, err = ss.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	klog.Infof("deleting mem_bdev")
	ok, err = ss.cli.DeleteBdevMalloc(req)
	return
}
