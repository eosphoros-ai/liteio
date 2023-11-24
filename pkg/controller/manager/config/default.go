package config

func SetDefaults(cfg *Config) {
	// set max remote volume
	if cfg.Scheduler.MaxRemoteVolumeCount <= 0 {
		cfg.Scheduler.MaxRemoteVolumeCount = 3
	}

	if len(cfg.Scheduler.Filters) == 0 {
		cfg.Scheduler.Filters = []string{
			"Basic",
			"Affinity",
		}
	}

	if len(cfg.Scheduler.Priorities) == 0 {
		cfg.Scheduler.Priorities = []string{
			"LeastResource",
			"PositionAdvice",
		}
	}
}
