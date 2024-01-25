package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	cfgStr := `
storage:
  pooling:
    mode: KernelLVM
    name: antstore-vg
  pvs:
  - devicePath: /dev/xxx
    size: 1234
    file: /tmp/xxx
  bdev:
    type: aioBdev
    name: aio-bdev-xxx
nodeInfoKeys:
  ipLabelKey: liteio.io/ip`

	cfg, err := Load([]byte(cfgStr))
	assert.NoError(t, err)
	t.Log(cfg, *cfg.Storage.Bdev)
}
