package kata

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/misc"
)

const (
	FsTypeExt4 = util.FileSystemExt4
	FsTypeXfs  = util.FileSystemXfs

	VolumeTypeRawfile   = "rawfile"
	VolumeTypeGuestNvmf = "guest_nvmf"

	VolumeModeBlock      = "Block"
	VolumeModeFilesystem = "Filesystem"
)

type KataVolumeConfig struct {
	// Device path for rawfile type volume, e.g. /dev/vg0/lvol0
	Device string `json:"device"`
	// xfs or ext4
	FsType string `json:"fs_type"`
	// guest_nvmf or rawfile
	VolumeType string `json:"volume_type"`
	// rund guest_nvmf type volume need this info to directly connect spdk
	SpdkInfo *SpdkInfo `json:"spdk_info,omitempty"`
	// Block or Filesystem
	VolumeMode string `json:"volume_mode,omitempty"`
}

/*
example config.json

	{
	  "device": "",
	  "fs_type": "ext2",
	  "volume_type": "guest_nvmf",
	  "spdk_info": {
	    "address": "100.100.100.1",
	    "bdev_name": "d3e56bb62253411285a9",
	    "ns_uuid": "1282805a-fc06-4051-9742-dbd9f1915f50",
	    "sn": "1282805afc0640519742",
	    "subsys_nqn": "nqn.2021-03.com.alipay.ob:uuid:1282805a-fc06-4051-9742-dbd9f1915f50",
	    "svc_id": "4510",
	    "trans_type": "TCP"
	  }
	}
*/
type SpdkInfo struct {
	Address   string `json:"address"`
	BdevName  string `json:"bdev_name"`
	NsUuid    string `json:"ns_uuid"`
	SN        string `json:"sn"`
	SubsysNQN string `json:"subsys_nqn"`
	SvcID     string `json:"svc_id"`
	TransType string `json:"trans_type"`
}

func WriteConfigFileForKataSpdkDirectConnect(file, fsType string, spdkInfo *v1.SpdkTarget) (err error) {
	if spdkInfo == nil {
		err = fmt.Errorf("spdkInfo is nil")
		return
	}

	config := KataVolumeConfig{
		FsType:     fsType,
		VolumeType: VolumeTypeGuestNvmf,
		SpdkInfo: &SpdkInfo{
			Address:   spdkInfo.Address,
			BdevName:  spdkInfo.BdevName,
			NsUuid:    spdkInfo.NSUUID,
			SN:        spdkInfo.SerialNum,
			SubsysNQN: spdkInfo.SubsysNQN,
			SvcID:     spdkInfo.SvcID,
			TransType: spdkInfo.TransType,
		},
	}
	jsonBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return writeFile(jsonBytes, file)
}

func WriteKataVolumeConfigFile(file, devicePath, fsType string, isBlockMode bool) error {
	// validate fsType
	if fsType != FsTypeExt4 && fsType != FsTypeXfs {
		return fmt.Errorf("not supported fstype=%s", fsType)
	}
	var volMode = VolumeModeFilesystem
	if isBlockMode {
		volMode = VolumeModeBlock
	}

	configContext := &KataVolumeConfig{
		Device:     devicePath,
		FsType:     fsType,
		VolumeType: VolumeTypeRawfile,
		VolumeMode: volMode,
	}
	data, err := json.MarshalIndent(configContext, "", "  ")
	if err != nil {
		return err
	}

	return writeFile(data, file)
}

func writeFile(data []byte, file string) (err error) {
	// vlidate parent dir exist
	pDir := path.Dir(file)
	has, err := misc.FileExists(pDir)
	if err != nil {
		return err
	}
	if !has {
		return fmt.Errorf("Dir %s not exists", pDir)
	}

	return ioutil.WriteFile(file, data, 0644)
}

func LoadKataVolumeConfigFile(file string) (volumeConfig *KataVolumeConfig, err error) {
	has, err := misc.FileExists(file)
	if !has {
		return
	}
	volumeConfig = &KataVolumeConfig{}
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	err = json.Unmarshal(data, &volumeConfig)
	return
}

func GetConfigFilePath(dir string) (file string) {
	return filepath.Join(dir, "config.json")
}
