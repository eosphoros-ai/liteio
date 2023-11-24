package kata

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteConfigFile(t *testing.T) {
	err := WriteKataVolumeConfigFile("/tmp/config.json", "dev", "xfs", false)
	assert.NoError(t, err)

	cfg, err := LoadKataVolumeConfigFile("/tmp/config.json")
	assert.NoError(t, err)
	assert.Equal(t, "dev", cfg.Device)
}
