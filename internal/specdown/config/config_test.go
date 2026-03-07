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
  "include": ["specs/**/*.spec.md"],
  "adapters": [
    {
      "name": "project",
      "command": ["python3", "./tools/adapter.py"],
      "blocks": ["run:myapp", "verify:myapp"],
      "fixtures": ["user-exists"]
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
	if len(cfg.Include) != 1 || cfg.Include[0] != "specs/**/*.spec.md" {
		t.Fatalf("unexpected include %#v", cfg.Include)
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
  "include": ["specs/**/*.spec.md"],
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
  "include": ["specs/**/*.spec.md"],
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
	body := `{"include": ["specs/**/*.spec.md"], "reporters": [{"builtin": "html", "outFile": "r.html"}]}`
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

func TestLoadConfigRejectsUnknownModelBuiltin(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "specdown.json")
	body := `{
  "include": ["specs/**/*.spec.md"],
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
