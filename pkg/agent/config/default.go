package config

const (
	SigmaLabelKeyNodeIP   = "lite.io/ip"
	SigmaLabelKeyHostname = "lite.io/hostname"
	SigmaLabelKeyRack     = "lite.io/rack"
	SigmaLabelKeyRoom     = "lite.io/room"
)

func SetDefaults(cfg *Config) {
	// set label key
	SetNodeInfoDefaults(&cfg.NodeKeys)
}

func SetNodeInfoDefaults(cfg *NodeInfoKeys) {
	if cfg.HostnameLabelKey == "" {
		cfg.HostnameLabelKey = SigmaLabelKeyHostname
	}

	if cfg.IPLabelKey == "" {
		cfg.IPLabelKey = SigmaLabelKeyNodeIP
	}

	if cfg.RackLabelKey == "" {
		cfg.RackLabelKey = SigmaLabelKeyRack
	}

	if cfg.RoomLabelKey == "" {
		cfg.RoomLabelKey = SigmaLabelKeyRoom
	}
}
