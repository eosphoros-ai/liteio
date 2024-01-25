package spdk

import (
	"errors"
	"fmt"
	"strings"

	"lite.io/liteio/pkg/spdk/jsonrpc/client"
	"lite.io/liteio/pkg/util/misc"
	"k8s.io/klog/v2"
)

type LVStoreInfo = client.LVStoreInfo

type LVolServiceIface interface {
	// TODO: remove
	CreateLVStoreFromNVMeIDs(req AttachNVMeReq) (lvs LVStoreInfo, err error)

	// LVS
	GetLVStore(name string) (lvs LVStoreInfo, err error)
	CreateLVStore(req CreateLVStoreReq) (lvs LVStoreInfo, err error)

	// LVol
	CreateLvol(req CreateLvolReq) (uuid string, err error)
	DeleteLvol(req DeleteLvolReq) (err error)
	ResizeLvol(req ResizeLvolReq) (err error)

	CreateLvolSnapshot(req CreateLvolSnapReq) (uuid string, err error)
	CreateLvolClone(req CreateLvolCloneReq) (uuid string, err error)
	InflateLvol(req InflateLvolReq) (err error)
}

type ResizeLvolReq struct {
	LvolFullName string
	TargetSize   uint64 // uint: MB
}

type AttachNVMeReq struct {
	NVMeIDs []string
}

type CreateBdevRaidReq struct {
	RaidName    string
	BdevNames   []string
	RaidLevel   string
	StripSizeKB int
}

type CreateLVStoreReq struct {
	BdevName    string
	LVStoreName string
}

type CreateLvolReq struct {
	LVStore  string
	LvolName string
	SizeByte int
}

type CreateLvolSnapReq struct {
	// full_name = lvs_name/lvol_name
	LvolFullName string
	SnapName     string
}

type CreateLvolCloneReq struct {
	LVStore   string
	SnapName  string
	CloneName string
}

type InflateLvolReq struct {
	LVStore  string
	LvolName string
}

func (req CreateLvolReq) BdevName() string {
	return fmt.Sprintf("%s/%s", req.LVStore, req.LvolName)
}

type DeleteLvolReq struct {
	LVStore  string
	LvolName string
}

func (req DeleteLvolReq) BdevName() string {
	return fmt.Sprintf("%s/%s", req.LVStore, req.LvolName)
}

func (ss *SpdkService) CreateLVStoreFromNVMeIDs(req AttachNVMeReq) (lvs LVStoreInfo, err error) {
	if len(req.NVMeIDs) == 0 {
		err = fmt.Errorf("cannot create lvs with empty NVMe list")
		return
	}

	ss.cli, err = ss.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	// attach controller
	var bdevNames []string
	bdevNames, err = ss.AttachNVMe(req)
	if err != nil {
		return
	}

	var (
		needRaid        bool   = len(bdevNames) > 1
		lvsBaseBdevName string = BdevRaidName
	)
	if needRaid {
		// create raid0
		err = ss.CreateBdevRaid(CreateBdevRaidReq{
			RaidName:    BdevRaidName,
			BdevNames:   bdevNames,
			RaidLevel:   "0",
			StripSizeKB: 128,
		})
		if err != nil {
			return
		}
	} else {
		lvsBaseBdevName = bdevNames[0]
	}

	// create lvs
	lvs, err = ss.CreateLVStore(CreateLVStoreReq{
		BdevName:    lvsBaseBdevName,
		LVStoreName: LVStoreName,
	})

	return
}

func (ss *SpdkService) CreateLvol(req CreateLvolReq) (uuid string, err error) {
	ss.cli, err = ss.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	var list []client.Bdev
	list, err = ss.cli.BdevGetBdevs(client.BdevGetBdevsReq{BdevName: req.BdevName()})
	if err != nil {
		if !IsNotFoundDeviceError(err) {
			klog.Error(err)
			return
		}
	}

	// do create
	if len(list) == 0 {
		uuid, err = ss.cli.BdevLVolCreate(client.BdevLVolCreateReq{
			LVolName: req.LvolName,
			Size:     req.SizeByte,
			LvsName:  req.LVStore,
		})
		if err != nil {
			return
		}
	}

	return
}

func (ss *SpdkService) DeleteLvol(req DeleteLvolReq) (err error) {
	ss.cli, err = ss.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	var list []client.Bdev
	list, err = ss.cli.BdevGetBdevs(client.BdevGetBdevsReq{BdevName: req.BdevName()})
	if err != nil {
		if !IsNotFoundDeviceError(err) {
			return
		}
	}

	if len(list) == 0 {
		klog.Infof("lvol %+s is already deleted", req.BdevName())
		return nil
	}

	var ok bool
	ok, err = ss.cli.BdevLVolDelete(client.BdevLVolDeleteReq{
		Name: req.BdevName(),
	})
	if err != nil {
		return
	}

	klog.Infof("DeleteLvol %+v, %t", req, ok)
	return
}

func (ss *SpdkService) ResizeLvol(req ResizeLvolReq) (err error) {
	ss.cli, err = ss.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	var list []client.Bdev
	list, err = ss.cli.BdevGetBdevs(client.BdevGetBdevsReq{BdevName: req.LvolFullName})
	if err != nil {
		if !IsNotFoundDeviceError(err) {
			klog.Error(err)
			return
		}
	}

	// do expand
	var ok bool
	if len(list) == 1 {
		ok, err = ss.cli.BdevLVolResize(client.BdevLVolResizeReq{
			Name: req.LvolFullName,
			Size: req.TargetSize,
		})
		if err != nil {
			return
		}
	}

	klog.Infof("ResizeLvol %+v, %t", req, ok)
	return
}

func (ss *SpdkService) GetLVStore(name string) (lvs LVStoreInfo, err error) {
	ss.cli, err = ss.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	var list []client.LVStoreInfo
	list, err = ss.cli.BdevLVolGetLVStores(client.BdevLVolGetLVStoresReq{
		LvsName: name,
	})
	if err != nil {
		return
	}

	lvs = list[0]
	return
}

func (ss *SpdkService) CreateLvolSnapshot(req CreateLvolSnapReq) (uuid string, err error) {
	ss.cli, err = ss.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	var (
		list         []client.Bdev
		lvs, _       = parseLvolFullName(req.LvolFullName)
		snapFullName = fmt.Sprintf("%s/%s", lvs, req.SnapName)
	)

	// check if snapshot exists
	list, err = ss.cli.BdevGetBdevs(client.BdevGetBdevsReq{BdevName: snapFullName})
	if err != nil && !IsNotFoundDeviceError(err) {
		klog.Error(err)
		return
	}
	if len(list) == 1 {
		klog.Infof("snapshot %s already exists", snapFullName)
		return
	}

	// check if original volume exists
	list, err = ss.cli.BdevGetBdevs(client.BdevGetBdevsReq{BdevName: req.LvolFullName})
	if err != nil {
		klog.Error(err)
		return
	}
	if len(list) != 1 {
		err = fmt.Errorf("the count of origin volume is invalid, %d", len(list))
		klog.Error(err)
		return "", err
	}
	uuid, err = ss.cli.BdevLVolSnapshot(client.BdevLVolSnapshotReq{
		LVolName:     req.LvolFullName,
		SnapshotName: req.SnapName,
	})
	if err != nil {
		klog.Error(err)
	}

	klog.Infof("CreateLvolSnapshot is done. req %+v, snap uuid %s", req, uuid)
	return
}

func (ss *SpdkService) CreateLvolClone(req CreateLvolCloneReq) (uuid string, err error) {
	ss.cli, err = ss.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	var list []client.Bdev
	var cloneFullName = fmt.Sprintf("%s/%s", req.LVStore, req.CloneName)
	// check if clone volume exists
	list, err = ss.cli.BdevGetBdevs(client.BdevGetBdevsReq{BdevName: cloneFullName})
	if err != nil && !IsNotFoundDeviceError(err) {
		klog.Error(err)
		return
	}
	if len(list) == 1 {
		klog.Infof("clone volume %s already exists", cloneFullName)
		return
	}

	// check if original volume exists
	list, err = ss.cli.BdevGetBdevs(client.BdevGetBdevsReq{
		BdevName: req.LVStore + "/" + req.SnapName,
	})
	if err != nil {
		klog.Error(err)
		return
	}
	if len(list) != 1 {
		err = fmt.Errorf("the count of snapshot volume is invalid, %d", len(list))
		klog.Error(err)
		return "", err
	}
	uuid, err = ss.cli.BdevLVolClone(client.BdevLVolCloneReq{
		SnapshotName: req.LVStore + "/" + req.SnapName,
		CloneName:    req.CloneName,
	})
	if err != nil {
		klog.Error(err)
	}

	klog.Infof("CreateLvolClone %+v, %s", req, uuid)
	return
}

func (ss *SpdkService) InflateLvol(req InflateLvolReq) (err error) {
	ss.cli, err = ss.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	// TODO: prevent from re-inflating any lvol
	var ok bool
	ok, err = ss.cli.BdevLVolInflate(client.BdevLVolInflateReq{
		Name: req.LVStore + "/" + req.LvolName,
	})
	if err != nil {
		klog.Error(err)
	}

	klog.Infof("CreateLvolClone %+v, %t", req, ok)
	return
}

func (svc *SpdkService) AttachNVMe(req AttachNVMeReq) (bdevNames []string, err error) {
	svc.cli, err = svc.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket", err)
		return
	}

	var (
		nvmeIDs    = misc.FromSlice(req.NVMeIDs)
		nvmeNames  = misc.NewEmptySet()
		existIDs   = misc.NewEmptySet()
		existNames = misc.NewEmptySet()
		list       []client.ControllerInfo
	)

	for idx := range req.NVMeIDs {
		nvmeName := fmt.Sprintf("Antstor_NVME%d", idx)
		nvmeNames.Add(nvmeName)
		bdevNames = append(bdevNames, nvmeName+"n1")
	}

	list, err = svc.cli.ListControllers()
	if err != nil {
		return
	}

	for _, item := range list {
		if strings.HasPrefix(item.Name, "Antstor_") && item.TrID.TrType == client.TrTypePCIe {
			existNames.Add(item.Name)
			existIDs.Add(item.TrID.TrAddr)
		}
	}

	toCreateNames := nvmeNames.Difference(existNames)
	toAttachIDs := nvmeIDs.Difference(existIDs)

	if toCreateNames.Size() != toAttachIDs.Size() {
		err = fmt.Errorf("size of toCreateNames not equals to toAttachIDs, %+v, %+v", toCreateNames.Values(), toAttachIDs.Values())
		return
	}

	var (
		toAttachIDsSlice   = toAttachIDs.Values()
		toCreateNamesSlice = toCreateNames.Values()
	)

	for idx, val := range toCreateNamesSlice {
		name := val
		id := toAttachIDsSlice[idx]

		_, err = svc.cli.AttachController(client.AttachControllerRequest{
			Name:   name,
			TrAddr: id,
			TrType: client.TrTypePCIe,
		})
		if err != nil {
			return
		}
	}

	return
}

func (ss *SpdkService) CreateBdevRaid(req CreateBdevRaidReq) (err error) {
	var (
		bdevName      = req.RaidName
		names         []string
		foundRaidBdev bool
		createOk      bool
	)
	names, err = ss.cli.ListBdevRaid(client.ListBdevRaidRequest{
		Category: client.RaidBdevCategoryAll,
	})
	if err != nil {
		return
	}
	foundRaidBdev = misc.InSliceString(bdevName, names)

	if !foundRaidBdev {
		createOk, err = ss.cli.CreateBdevRaid(client.CreateBdevRaidRequest{
			Name:        req.RaidName,
			BaseBdevs:   req.BdevNames,
			StripSizeKB: req.StripSizeKB,
			RaidLevel:   req.RaidLevel,
		})
		if !createOk || err != nil {
			err = fmt.Errorf("CreateBdevRaid failed, err %+v, createOk %t", err, createOk)
			return
		}
	}
	return
}

func (ss *SpdkService) CreateLVStore(req CreateLVStoreReq) (lvs client.LVStoreInfo, err error) {
	var (
		lvstoreName = req.LVStoreName
		list        []client.LVStoreInfo
		uuid        string
	)

	list, err = ss.cli.BdevLVolGetLVStores(client.BdevLVolGetLVStoresReq{
		LvsName: lvstoreName,
	})
	if err != nil {
		// only return error when error msg is "No such device"
		if !IsNotFoundDeviceError(err) {
			klog.Error(err)
			return
		}
	}

	// do create
	if len(list) == 0 {
		uuid, err = ss.cli.BdevLVolCreateLVStore(client.BdevLVolCreateLVStoreReq{
			BdevName:    req.BdevName,
			LvsName:     req.LVStoreName,
			ClusterSize: 2 * misc.MiB,
		})
		if err != nil {
			return
		}
		klog.Info("new lvs uuid = ", uuid)
		list, err = ss.cli.BdevLVolGetLVStores(client.BdevLVolGetLVStoresReq{
			LvsName: lvstoreName,
		})
		if err != nil || len(list) == 0 {
			err = errors.New(err.Error() + " or not found any lvs")
			return
		}
		lvs = list[0]

	} else {
		lvs = list[0]
	}

	return
}
