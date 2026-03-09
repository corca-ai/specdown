package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Entry          string          `json:"entry"`
	Adapters       []AdapterConfig `json:"adapters"`
	Models         ModelConfig     `json:"models"`
	Reporters      []Reporter      `json:"reporters"`
	IgnorePrefixes []string        `json:"ignorePrefixes,omitempty"`
}

type ModelConfig struct {
	Builtin string `json:"builtin"`
}

type AdapterConfig struct {
	Name        string   `json:"name"`
	Command     []string `json:"command"`
	Blocks      []string `json:"blocks"`
	Checks    []string `json:"checks,omitempty"`
	ChecksDir string   `json:"checksDir,omitempty"`
	BuiltinShell bool    `json:"-"` // set internally for the auto-registered shell adapter
}

type Reporter struct {
	Builtin string `json:"builtin"`
	OutFile string `json:"outFile"`
}

// Default returns sensible defaults that allow specdown to run without a config file.
func Default() Config {
	return Config{
		Entry:    "specs/index.spec.md",
		Adapters: nil,
		Models:   ModelConfig{Builtin: "alloy"},
		Reporters: []Reporter{
			{Builtin: "html", OutFile: "specs/report.html"},
			{Builtin: "json", OutFile: "specs/report.json"},
		},
	}
}

func applyDefaults(cfg *Config) {
	if cfg.Entry == "" {
		cfg.Entry = "specs/index.spec.md"
	}
	if cfg.Models.Builtin == "" {
		cfg.Models.Builtin = "alloy"
	}
	if len(cfg.Reporters) == 0 {
		cfg.Reporters = []Reporter{
			{Builtin: "html", OutFile: "specs/report.html"},
			{Builtin: "json", OutFile: "specs/report.json"},
		}
	}
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

	applyDefaults(&cfg)
	if err := validateAdapters(cfg.Adapters); err != nil {
		return Config{}, "", err
	}
	if cfg.Models.Builtin != "alloy" {
		return Config{}, "", fmt.Errorf("models builtin %q is not supported", cfg.Models.Builtin)
	}

	return cfg, filepath.Dir(absPath), nil
}

// LoadOrDefault loads config from path, or returns Default() if the file does not exist.
func LoadOrDefault(path string) (Config, string, error) {
	cfg, dir, err := Load(path)
	if err != nil {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) && os.IsNotExist(pathErr.Err) {
			cwd, wdErr := os.Getwd()
			if wdErr != nil {
				return Config{}, "", fmt.Errorf("get working directory: %w", wdErr)
			}
			return Default(), cwd, nil
		}
		return Config{}, "", err
	}
	return cfg, dir, nil
}

func validateAdapters(adapters []AdapterConfig) error {
	seen := make(map[string]struct{}, len(adapters))
	for _, adapter := range adapters {
		if adapter.BuiltinShell {
			continue
		}
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
		if len(adapter.Blocks) == 0 && len(adapter.Checks) == 0 {
			return fmt.Errorf("adapter %q must declare at least one block or check", adapter.Name)
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
