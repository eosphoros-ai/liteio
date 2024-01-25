package config

import (
	"encoding/json"
	"io"
	"os"

	"lite.io/liteio/pkg/util/misc"
	corev1 "k8s.io/api/core/v1"
)

type Config struct {
	Scheduler     SchedulerConfig `json:"scheduler" yaml:"scheduler"`
	PluginConfigs json.RawMessage `json:"pluginConfigs" yaml:"pluginConfigs"`
}

type SchedulerConfig struct {
	// MaxRemoteVolumeCount defines the max count of remote volumes on a single node
	MaxRemoteVolumeCount int `json:"maxRemoteVolumeCount" yaml:"maxRemoteVolumeCount"`
	// RemoteIgnoreAnnoSelector defines volumes to be ignored when RemoteVolumesCount is called
	RemoteIgnoreAnnoSelector map[string]string `json:"remoteIgnoreAnnoSelector" yaml:"remoteIgnoreAnnoSelector"`
	// filter names
	Filters []string `json:"filters" yaml:"filters"`
	// priority names
	Priorities []string `json:"priorities" yaml:"priorities"`
	// LockSchedCfg
	LockSchedCfg NoScheduleConfig `json:"lockSchedConfig" yaml:"lockSchedConfig"`
	// NodeCacheSelector specify which nodes are cached to Node Informer.
	// Empty selector means all nodes are allowd to be cached.
	NodeCacheSelector map[string]string `json:"nodeCacheSelector" yaml:"nodeCacheSelector"`
	// MinLocalStoragePct defines the minimun percentage of local storage to be reserved on one node.
	MinLocalStoragePct int `json:"minLocalStoragePct" yaml:"minLocalStoragePct"`
	// NodeReservations defines the reservations on each node
	NodeReservations []NodeReservation `json:"nodeReservations" yaml:"nodeReservations"`
}

type NodeReservation struct {
	ID   string `json:"id" yaml:"id"`
	Size int64  `json:"size" yaml:"size"`
}

type NoScheduleConfig struct {
	NodeSelector []corev1.NodeSelectorRequirement `json:"nodeSelector" yaml:"nodeSelector"`
	NodeTaints   []corev1.Toleration              `json:"nodeTaints" yaml:"nodeTaints"`
}

func Load(file string) (c Config, err error) {
	var (
		f *os.File
		b []byte
	)

	f, err = os.Open(file)
	if err != nil {
		return
	}
	b, err = io.ReadAll(f)
	if err != nil {
		return
	}

	return fromYamlBytes(b)
}

func fromYamlBytes(bs []byte) (c Config, err error) {
	jsonStr, err := misc.YamlToJSON(string(bs))
	if err != nil {
		return
	}

	err = json.Unmarshal([]byte(jsonStr), &c)
	return
}
