package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Title     string          `json:"title"`
	Include   []string        `json:"include"`
	Adapters  []AdapterConfig `json:"adapters"`
	Models    ModelConfig     `json:"models"`
	Reporters []Reporter      `json:"reporters"`
}

type ModelConfig struct {
	Builtin string `json:"builtin"`
}

type AdapterConfig struct {
	Name     string   `json:"name"`
	Command  []string `json:"command"`
	Blocks   []string `json:"blocks"`
	Fixtures []string `json:"fixtures,omitempty"`
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
	if err := validateAdapters(cfg.Adapters); err != nil {
		return Config{}, "", err
	}
	if cfg.Models.Builtin != "" && cfg.Models.Builtin != "alloy" {
		return Config{}, "", fmt.Errorf("models builtin %q is not supported", cfg.Models.Builtin)
	}

	return cfg, filepath.Dir(absPath), nil
}

func validateAdapters(adapters []AdapterConfig) error {
	seen := make(map[string]struct{}, len(adapters))
	for _, adapter := range adapters {
		if adapter.Name == "" {
			return fmt.Errorf("adapter name must not be empty")
		}
		if _, ok := seen[adapter.Name]; ok {
			return fmt.Errorf("adapter %q is defined more than once", adapter.Name)
		}
		seen[adapter.Name] = struct{}{}
		if len(adapter.Command) == 0 {
			return fmt.Errorf("adapter %q must define a command", adapter.Name)
		}
		if len(adapter.Blocks) == 0 && len(adapter.Fixtures) == 0 {
			return fmt.Errorf("adapter %q must declare at least one block or fixture", adapter.Name)
		}
	}
	return nil
}

func (c Config) HTMLReportOutFile() string {
	for _, reporter := range c.Reporters {
		if reporter.Builtin == "html" && reporter.OutFile != "" {
			return reporter.OutFile
		}
	}
	return ""
}

func (c Config) JSONReportOutFile() string {
	for _, reporter := range c.Reporters {
		if reporter.Builtin == "json" && reporter.OutFile != "" {
			return reporter.OutFile
		}
	}
	return ""
}
