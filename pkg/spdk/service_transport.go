package spdk

import (
	"fmt"

	"lite.io/liteio/pkg/spdk/jsonrpc/client"
	"k8s.io/klog/v2"
)

func (svc *SpdkService) InitTransport() (err error) {
	cli, err := svc.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	list, err := cli.NVMFGetTransports()
	if err != nil {
		klog.Error(err)
		return
	}
	// TODO: check whether nvmf_tgt has the capability to create VFIO transport
	hasTCPTransport := false
	hasVFIOTransport := false
	for _, trans := range list {
		if trans.TransType == client.TransportTypeTCP {
			hasTCPTransport = true
		}
		if trans.TransType == client.TransportTypeVFIOUSER {
			hasVFIOTransport = true
		}
	}
	if !hasTCPTransport {
		result, err := cli.NVMFCreateTransport(client.NVMFCreateTransportReq{
			TrType: client.TransportTypeTCP,
			// MaxQPairsPerCtrlr is deprecated
			// MaxIOQPairsPerCtrlr is changed from 64 to 4
			MaxIOQPairsPerCtrlr: 4,
			InCapsuleDataSize:   1000,
			IOUnitSize:          16384,
		})
		if err != nil {
			klog.Error(err)
			return err
		}
		if !result {
			err = fmt.Errorf("SPDK init TCP transport result is false")
			return err
		}
	}
	if !hasVFIOTransport {
		// create vfio transport
		// TODO do not return error to fit current version of nvmf_tgt
		result, err := cli.NVMFCreateTransport(client.NVMFCreateTransportReq{
			TrType:    client.TransportTypeVFIOUSER,
			MaxIOSize: 131072,
		})
		if err != nil {
			klog.Error(err)
			// return err
		}
		if !result {
			err = fmt.Errorf("SPDK init VFIOUSER transport result is false")
			klog.Error(err)
			// return err
		}

	}
	return
}
