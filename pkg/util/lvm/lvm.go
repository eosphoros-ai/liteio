package lvm

import "lite.io/liteio/pkg/util/osutil"

var (
	// default is cgo API; call EnableLvm2Cmd() to replace it with lvm2cmd implementation
	LvmUtil LvmIface = &cmd{
		jsonFormat: true,
		exec:       osutil.NewCommandExec(),
	}
)

type VG struct {
	Name      string
	UUID      string
	TotalByte uint64
	FreeByte  uint64
	PVCount   int
	// extends
	ExtendCount int
	ExtendSize  uint64
}

type LV struct {
	Name     string
	VGName   string
	DevPath  string
	SizeByte uint64
	// striped or linear or thin,pool
	LvLayout string
	// attributes
	LvAttr string
	// lv device status: "open" or ""
	LvDeviceOpen string
	// origin vol of snapshot
	Origin     string
	OriginUUID string
	OriginSize string
}

type LvOption struct {
	Size      uint64
	LogicSize string
}

type LvmIface interface {
	CreateVG(name string, pvs []string) (VG, error)
	CreatePV(pvs []string) error
	ListVG() ([]VG, error)
	ListLVInVG(vgName string) ([]LV, error)
	ListPV() ([]PV, error)
	CreateLinearLV(vgName, lvName string, opt LvOption) (vol LV, err error)
	CreateStripeLV(vgName, lvName string, sizeByte uint64) (vol LV, err error)
	RemoveLV(vgName, lvName string) (err error)
	RemoveVG(vgName string) (err error)
	RemovePVs(pvs []string) (err error)
	ExpandVolume(deltaBytes int64, targetVol string) (err error)

	CreateSnapshotLinear(vgName, snapName, originVol string, sizeByte uint64) (err error)
	CreateSnapshotStripe(vgName, snapName, originVol string, sizeByte uint64) (err error)
	MergeSnapshot(vgName, snapName string) (err error)
}
