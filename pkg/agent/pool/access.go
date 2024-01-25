package pool

import (
	"fmt"
	"io/fs"
	"os"

	"lite.io/liteio/pkg/spdk"
	"lite.io/liteio/pkg/spdk/jsonrpc/client"
	"lite.io/liteio/pkg/util/misc"
	"k8s.io/klog/v2"
)

type Access struct {
	AIO  *AioVolume
	LVol *SpdkLVolume
	// when remove access, NQN is required
	OpenAccess spdk.Target
	// allow host nqn
	AllowHostNQN []string
}

type AioVolume struct {
	// VolumeName string
	DevPath string
	// BdevName is bdev to be exposed publicly
	BdevName string
}

type SpdkLVolume struct {
	LvsName  string
	LvolName string
}

type AccessIface interface {
	ExposeAccess(a Access) (tgt spdk.Target, err error)
	RemoveAccces(a Access) (err error)
}

type SpdkAccess struct {
	spdk spdk.SpdkServiceIface
}

func NewSpdkAccess(spdk spdk.SpdkServiceIface) AccessIface {
	return &SpdkAccess{
		spdk: spdk,
	}
}

func (sa *SpdkAccess) ExposeAccess(a Access) (tgt spdk.Target, err error) {
	var bdevName string
	if a.AIO != nil {
		klog.Infof("creating aio bdev: %+v", *a.AIO)
		bdevName = a.AIO.BdevName
		// create aio bdev
		err = sa.spdk.CreateAioBdev(spdk.AioBdevCreateRequest{
			BdevName: a.AIO.BdevName,
			DevPath:  a.AIO.DevPath,
		})
		if err != nil {
			klog.Error(err)
			return
		}

	}

	if a.LVol != nil {
		// TODO: create target service over lvol
		bdevName = fmt.Sprintf("%s/%s", a.LVol.LvsName, a.LVol.LvolName)
	}

	// create the socket directory for VFIOUSER local volume,
	if a.OpenAccess.TransType == client.TransportTypeVFIOUSER {
		var exist bool
		exist, err = misc.FileExists(a.OpenAccess.TransAddr)
		if err != nil {
			klog.Error(err)
			return
		}
		if !exist {
			err = os.MkdirAll(a.OpenAccess.TransAddr, fs.ModeDir)
			if err != nil {
				klog.Error(err)
				return
			}
		}
	}

	// expose target service over bdev
	klog.Infof("creating target service %+v", a.OpenAccess)
	var resp spdk.Target
	resp, err = sa.spdk.CreateTarget(spdk.TargetCreateRequest{
		BdevName:   bdevName,
		TargetInfo: a.OpenAccess,
	})
	if err != nil {
		return
	}
	tgt.SvcID = resp.SvcID
	tgt.NQN = resp.NQN

	// set allow host
	// get subsystem by nqn
	var subsys spdk.Subsystem
	var allowHostSet = misc.NewEmptySet()
	subsys, err = sa.spdk.GetSubsystemByNQN(tgt.NQN)
	if err != nil {
		return
	}
	for _, host := range subsys.Hosts {
		allowHostSet.Add(host.NQN)
	}
	// add host if it is not added
	if len(a.AllowHostNQN) > 0 {
		for _, item := range a.AllowHostNQN {
			if !allowHostSet.Contains(item) {
				err = sa.spdk.SubsysAddHost(spdk.SubsystemAddHostRequest{
					NQN:     resp.NQN,
					HostNQN: item,
				})
				if err != nil {
					return
				}
			}
		}
	}

	return
}

func (sa *SpdkAccess) RemoveAccces(a Access) (err error) {
	if a.OpenAccess.NQN != "" {
		klog.Infof("deleting target %s", a.OpenAccess.NQN)
		err = sa.spdk.DeleteTarget(a.OpenAccess.NQN)
		if err != nil {
			klog.Error(err)
			return
		}
	}

	// clear socket path of VFIOUSER local volume after removing access
	if a.OpenAccess.TransType == client.TransportTypeVFIOUSER {
		if has, _ := misc.FileExists(a.OpenAccess.TransAddr); has {
			err = os.RemoveAll(a.OpenAccess.TransAddr)
			if err != nil {
				err = fmt.Errorf("delete directory %s failed, %w", a.OpenAccess.TransAddr, err)
				klog.Error(err)
				return
			}
		}
	}

	var bdevName string

	if a.AIO != nil {
		bdevName = a.AIO.BdevName
		klog.Infof("deleting aio bdev %s", bdevName)
		err = sa.spdk.DeleteAioBdev(spdk.AioBdevDeleteRequest{
			BdevName: bdevName,
		})
		if err != nil {
			klog.Error(err)
			return
		}
	}

	return
}
