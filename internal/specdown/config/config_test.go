package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigParsesAdaptersAndReporters(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "specdown.json")
	body := `{
  "entry": "specs/index.spec.md",
  "adapters": [
    {
      "name": "project",
      "command": ["python3", "./tools/adapter.py"],
      "blocks": ["run:myapp"],
      "checks": ["user-exists"]
    }
  ],
  "models": {
    "builtin": "alloy"
  },
  "reporters": [
    {
      "builtin": "html",
      "outFile": ".artifacts/specdown/report.html"
    }
  ]
}`
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, dir, err := Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if dir != root {
		t.Fatalf("unexpected config dir %q", dir)
	}
	if len(cfg.Adapters) != 1 || cfg.Adapters[0].Name != "project" {
		t.Fatalf("unexpected adapters %#v", cfg.Adapters)
	}
	if cfg.Entry != "specs/index.spec.md" {
		t.Fatalf("unexpected entry %q", cfg.Entry)
	}
	if cfg.Models.Builtin != "alloy" {
		t.Fatalf("unexpected models %#v", cfg.Models)
	}
	if got := cfg.HTMLReportOutFile(); got != ".artifacts/specdown/report.html" {
		t.Fatalf("unexpected report output %q", got)
	}
}

func TestLoadConfigAllowsAlloyOnlyProjectWithoutAdapters(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "specdown.json")
	body := `{
  "entry": "specs/index.spec.md",
  "reporters": [
    {
      "builtin": "html",
      "outFile": ".artifacts/specdown/report.html"
    }
  ]
}`
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, _, err := Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.Adapters) != 0 {
		t.Fatalf("unexpected adapters %#v", cfg.Adapters)
	}
}

func TestJSONReportOutFile(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "specdown.json")
	body := `{
  "entry": "specs/index.spec.md",
  "reporters": [
    {"builtin": "html", "outFile": "report.html"},
    {"builtin": "json", "outFile": "result.json"}
  ]
}`
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, _, err := Load(configPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.JSONReportOutFile(); got != "result.json" {
		t.Fatalf("expected 'result.json', got %q", got)
	}
}

func TestJSONReportOutFileReturnsEmptyWhenNotConfigured(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "specdown.json")
	body := `{"entry": "specs/index.spec.md", "reporters": [{"builtin": "html", "outFile": "r.html"}]}`
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, _, err := Load(configPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.JSONReportOutFile(); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Entry != "specs/index.spec.md" {
		t.Fatalf("unexpected entry %q", cfg.Entry)
	}
	if cfg.Models.Builtin != "alloy" {
		t.Fatalf("unexpected models %#v", cfg.Models)
	}
	if len(cfg.Reporters) != 2 {
		t.Fatalf("expected 2 reporters, got %d", len(cfg.Reporters))
	}
	if cfg.HTMLReportOutFile() != "specs/report" {
		t.Fatalf("unexpected html report %q", cfg.HTMLReportOutFile())
	}
	if cfg.JSONReportOutFile() != "specs/report.json" {
		t.Fatalf("unexpected json report %q", cfg.JSONReportOutFile())
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "specdown.json")
	if err := os.WriteFile(configPath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, _, err := Load(configPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Entry != "specs/index.spec.md" {
		t.Fatalf("expected default entry, got %q", cfg.Entry)
	}
	if cfg.Models.Builtin != "alloy" {
		t.Fatalf("expected default models builtin, got %q", cfg.Models.Builtin)
	}
	if len(cfg.Reporters) != 2 {
		t.Fatalf("expected 2 default reporters, got %d", len(cfg.Reporters))
	}
}

func TestLoadOrDefaultWhenMissing(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "nonexistent.json")
	cfg, dir, err := LoadOrDefault(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Entry != "specs/index.spec.md" {
		t.Fatalf("expected default entry, got %q", cfg.Entry)
	}
	if cfg.Models.Builtin != "alloy" {
		t.Fatalf("expected default models builtin, got %q", cfg.Models.Builtin)
	}
	if dir == "" {
		t.Fatal("expected non-empty dir")
	}
}

func TestLoadConfigRejectsUnknownModelBuiltin(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "specdown.json")
	body := `{
  "entry": "specs/index.spec.md",
  "models": {
    "builtin": "unknown"
  }
}`
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got == "" || !strings.Contains(got, `models builtin "unknown" is not supported`) {
		t.Fatalf("unexpected error %v", err)
	}
}
