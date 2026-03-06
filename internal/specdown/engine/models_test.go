package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"specdown/internal/specdown/config"
	"specdown/internal/specdown/core"
)

func TestCheckModelsRunsOnlyAlloyChecks(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "pocket-board.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := strings.Join([]string{
		"# Pocket Board",
		"",
		"## 형식 규칙",
		"",
		"```alloy:model(board)",
		"module board",
		"",
		"sig Card {}",
		"",
		"assert cardShape { some Card }",
		"```",
		"",
		"<!-- alloy:ref(board#cardShape, scope=5) -->",
		"",
	}, "\n")
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	report, err := checkModelsWithRunner(root, config.Config{
		Include: []string{"specs/**/*.spec.md"},
	}, fakeAlloyRunner{
		results: map[string]core.AlloyCheckResult{
			"specs/pocket-board.spec.md|Pocket Board|형식 규칙|1": {
				ID: core.SpecID{
					File:        "specs/pocket-board.spec.md",
					HeadingPath: []string{"Pocket Board", "형식 규칙"},
					Ordinal:     1,
				},
				Model:     "board",
				Assertion: "cardShape",
				Scope:     "5",
				Label:     "alloy:ref(board#cardShape, scope=5) @ 형식 규칙",
				Status:    core.StatusPassed,
			},
		},
	})
	if err != nil {
		t.Fatalf("check models: %v", err)
	}

	if report.Summary.CasesTotal != 0 || report.Summary.AlloyChecksPassed != 1 {
		t.Fatalf("unexpected summary %+v", report.Summary)
	}
	if got := len(report.Results[0].Cases); got != 0 {
		t.Fatalf("unexpected cases %d", got)
	}
}
