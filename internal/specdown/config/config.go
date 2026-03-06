package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"specdown/internal/specdown/adapterprotocol"
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

	if len(cfg.Include) == 0 {
		return Config{}, "", fmt.Errorf("config must define at least one include pattern")
	}
	if len(cfg.Adapters) == 0 {
		return Config{}, "", fmt.Errorf("config must define at least one adapter")
	}
	seen := make(map[string]struct{}, len(cfg.Adapters))
	for _, adapter := range cfg.Adapters {
		if adapter.Name == "" {
			return Config{}, "", fmt.Errorf("adapter name must not be empty")
		}
		if _, ok := seen[adapter.Name]; ok {
			return Config{}, "", fmt.Errorf("adapter %q is defined more than once", adapter.Name)
		}
		seen[adapter.Name] = struct{}{}
		if len(adapter.Command) == 0 {
			return Config{}, "", fmt.Errorf("adapter %q must define a command", adapter.Name)
		}
		if adapter.Protocol != adapterprotocol.Version {
			return Config{}, "", fmt.Errorf("adapter %q must use protocol %q", adapter.Name, adapterprotocol.Version)
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
