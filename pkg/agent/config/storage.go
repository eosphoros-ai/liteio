package config

import v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"

const (
	AioBdevType  BdevType = "aioBdev"
	MemBdevType  BdevType = "memBdev"
	RaidBdevType BdevType = "raidBdev"

	DefaultLVMName   = "antstore-vg"
	DefaultLVSName   = "antstor_lvstore"
	DefaultRaid0Name = "antstor_raid0"

	DefaultMallocBdevName = "antstor_malloc"
	DefaultAioBdevName    = "antstor_aio"
)

var (
	DefaultLVM = StorageStack{
		Pooling: Pooling{
			Mode: v1.PoolModeKernelLVM,
			Name: DefaultLVMName,
		},
	}

	DefaultLVS = StorageStack{
		Pooling: Pooling{
			Mode: v1.PoolModeSpdkLVStore,
			Name: DefaultLVSName,
		},
		Bdev: &SpdkBdev{
			Type: RaidBdevType,
			Name: DefaultRaid0Name,
		},
	}
)

type BdevType string

type StorageStack struct {
	Pooling Pooling   `json:"pooling" yaml:"pooling"`
	PVs     []LvmPV   `json:"pvs,omitempty" yaml:"pvs"`
	Bdev    *SpdkBdev `json:"bdev,omitempty" yaml:"bdev"`
}

type Pooling struct {
	Mode v1.PoolMode `json:"mode" yaml:"mode"`
	Name string      `json:"name" yaml:"name"`
}

type LvmPV struct {
	// DevicePath is device path of PV. if it is empty, create a loop device from a file
	DevicePath string `json:"devicePath" yaml:"devicePath"`
	// if DevicePath is empty, use Size to create a file
	Size uint64 `json:"size" yaml:"size"`
	// if not empty, create loop device from file
	FilePath         string `json:"filePath,omitempty" yaml:"filePath"`
	CreateIfNotExist bool   `json:"createIfNotExist,omitempty" yaml:"createIfNotExist"`
}

type SpdkBdev struct {
	Type BdevType `json:"type" yaml:"type"`
	Name string   `json:"name" yaml:"name"`
	// size in byte
	Size uint64 `json:"size" yaml:"size"`
	// for aioBdev
	FilePath         string `json:"filePath,omitempty" yaml:"filePath"`
	CreateIfNotExist bool   `json:"createIfNotExist,omitempty" yaml:"createIfNotExist"`
	// for vfio raidBdev
	VfioPCIeKeyword string `json:"vfioPCIeKeyword,omitempty" yaml:"vfioPCIeKeyword"`
}
