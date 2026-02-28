package state

import (
	"os"

	"gopkg.in/yaml.v3"
)

// LoadConfig reads workspace.yaml and returns a Config
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	// Apply defaults
	if cfg.Supervision.StuckTimeoutMinutes == 0 {
		cfg.Supervision.StuckTimeoutMinutes = 5
	}
	if cfg.Supervision.MaxSpawnRetries == 0 {
		cfg.Supervision.MaxSpawnRetries = 3
	}
	if cfg.Supervision.TokenBudgetPerTask == 0 {
		cfg.Supervision.TokenBudgetPerTask = 100000
	}
	if cfg.Supervision.ResourceSampleIntervalSec == 0 {
		cfg.Supervision.ResourceSampleIntervalSec = 60
	}
	if cfg.Alerts.GPUUtilizationPercent == 0 {
		cfg.Alerts.GPUUtilizationPercent = 95
	}
	if cfg.Alerts.GPUAlertDurationMinutes == 0 {
		cfg.Alerts.GPUAlertDurationMinutes = 5
	}
	if cfg.Alerts.RAMAvailablePercentMin == 0 {
		cfg.Alerts.RAMAvailablePercentMin = 10
	}
	if cfg.Alerts.DiskFreeGBMin == 0 {
		cfg.Alerts.DiskFreeGBMin = 5
	}
	return &cfg, nil
}
