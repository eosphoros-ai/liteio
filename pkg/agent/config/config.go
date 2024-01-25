package config

import (
	"encoding/json"
	"io/ioutil"
	"os"

	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"lite.io/liteio/pkg/util/misc"
)

type Config struct {
	Storage  StorageStack `json:"storage" yaml:"storage"`
	NodeKeys NodeInfoKeys `json:"nodeInfoKeys" yaml:"nodeInfoKeys"`
	NodeInfo v1.NodeInfo  `json:"nodeInfo,omitempty"`
}

type NodeInfoKeys struct {
	IPLabelKey       string `json:"ipLabelKey" yaml:"ipLabelKey"`
	HostnameLabelKey string `json:"hostnameLabelKey" yaml:"hostnameLabelKey"`
	RackLabelKey     string `json:"rackLabelKey" yaml:"rackLabelKey"`
	RoomLabelKey     string `json:"roomLabelKey" yaml:"roomLabelKey"`
}

func Load(bs []byte) (c Config, err error) {
	jsonStr, err := misc.YamlToJSON(string(bs))
	if err != nil {
		return
	}

	err = json.Unmarshal([]byte(jsonStr), &c)
	return
}

func LoadFile(file string) (c Config, err error) {
	var (
		f *os.File
		b []byte
	)

	f, err = os.Open(file)
	if err != nil {
		return
	}
	b, err = ioutil.ReadAll(f)
	if err != nil {
		return
	}

	return Load(b)
}
