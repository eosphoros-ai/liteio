package config

const (
	SigmaLabelKeyNodeIP   = "sigma.ali/node-ip"
	SigmaLabelKeyHostname = "sigma.ali/armory-hostname"
	SigmaLabelKeyRack     = "sigma.ali/rack"
	SigmaLabelKeyRoom     = "sigma.ali/room"
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
