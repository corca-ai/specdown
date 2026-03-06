package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Include   []string        `json:"include"`
	Adapters  []AdapterConfig `json:"adapters"`
	Reporters []Reporter      `json:"reporters"`
}

type AdapterConfig struct {
	Name     string   `json:"name"`
	Command  []string `json:"command"`
	Protocol string   `json:"protocol"`
}

type Reporter struct {
	Builtin string `json:"builtin"`
	OutFile string `json:"outFile"`
}

func Load(path string) (Config, string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return Config{}, "", fmt.Errorf("resolve config path: %w", err)
	}

	body, err := os.ReadFile(absPath)
	if err != nil {
		return Config{}, "", fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(body, &cfg); err != nil {
		return Config{}, "", fmt.Errorf("parse config: %w", err)
	}

	if len(cfg.Adapters) == 0 {
		return Config{}, "", fmt.Errorf("config must define at least one adapter")
	}
	for _, adapter := range cfg.Adapters {
		if adapter.Name == "" {
			return Config{}, "", fmt.Errorf("adapter name must not be empty")
		}
		if len(adapter.Command) == 0 {
			return Config{}, "", fmt.Errorf("adapter %q must define a command", adapter.Name)
		}
	}

	return cfg, filepath.Dir(absPath), nil
}

func (c Config) HTMLReportOutFile() string {
	for _, reporter := range c.Reporters {
		if reporter.Builtin == "html" && reporter.OutFile != "" {
			return reporter.OutFile
		}
	}
	return ""
}
