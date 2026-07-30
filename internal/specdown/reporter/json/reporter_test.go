package json

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

func TestWriteEncodesReportJSON(t *testing.T) {
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "report.json")

	report := core.Report{
		SchemaVersion: 3,
		GeneratedAt:   time.Date(2026, 3, 6, 1, 2, 3, 0, time.UTC),
		Summary: core.Summary{
			SpecsTotal:      1,
			SpecsFailed:     1,
			CasesTotal:      4,
			CasesPassed:     3,
			CasesSkipped:    1,
			LifecycleTotal:  2,
			LifecyclePassed: 1,
			LifecycleFailed: 1,
		},
		LifecycleEvents: []core.LifecycleEvent{{
			Scope:   core.LifecycleScopeGlobal,
			Phase:   core.HookTeardown,
			Status:  core.StatusFailed,
			Message: "exit status 7",
		}},
		Results: []core.DocumentResult{{
			Document: core.Document{RelativeTo: "specs/lifecycle.md"},
			Status:   core.StatusFailed,
			LifecycleEvents: []core.LifecycleEvent{{
				Scope:       core.LifecycleScopeSection,
				Phase:       core.HookSetup,
				Status:      core.StatusPassed,
				File:        "specs/lifecycle.md",
				HeadingPath: core.HeadingPath{"Lifecycle"},
			}},
		}},
	}

	if err := Write(report, outPath); err != nil {
		t.Fatalf("write report: %v", err)
	}

	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	text := string(body)
	if !strings.Contains(text, `"casesPassed": 3`) {
		t.Fatalf("expected cases summary, got %q", text)
	}
	for _, want := range []string{
		`"schemaVersion": 3`,
		`"lifecycleFailed": 1`,
		`"casesSkipped": 1`,
		`"scope": "global"`,
		`"phase": "teardown"`,
		`"message": "exit status 7"`,
		`"scope": "section"`,
		`"headingPath": [`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in lifecycle JSON, got %q", want, text)
		}
	}
}
