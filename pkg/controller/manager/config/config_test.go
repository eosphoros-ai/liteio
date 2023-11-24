package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	cfg = `scheduler:
  maxRemoteVolumeCount: 3
pluginConfigs:
  test:
    aaa: bbb
  test2:
    ccc: ddd`
)

func TestConfig(t *testing.T) {
	c, err := fromYamlBytes([]byte(cfg))
	assert.NoError(t, err)
	assert.Equal(t, 3, c.Scheduler.MaxRemoteVolumeCount)

	type TestPluginConfigs struct {
		Test  map[string]string `json:"test"`
		Test2 map[string]string `json:"test2"`
	}

	var testCfg TestPluginConfigs
	err = json.Unmarshal(c.PluginConfigs, &testCfg)
	assert.NoError(t, err)
	t.Log(testCfg)

	assert.Equal(t, "bbb", testCfg.Test["aaa"])
	assert.Equal(t, "ddd", testCfg.Test2["ccc"])
}
