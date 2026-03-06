package config

import (
	"os"
	"path/filepath"
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
      "protocol": "specdown-adapter/v1"
    }
  ],
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
	if got := cfg.HTMLReportOutFile(); got != ".artifacts/specdown/report.html" {
		t.Fatalf("unexpected report output %q", got)
	}
}
