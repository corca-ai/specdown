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
		GeneratedAt: time.Date(2026, 3, 6, 1, 2, 3, 0, time.UTC),
		Summary: core.Summary{
			SpecsTotal:  1,
			SpecsPassed: 1,
			CasesTotal:  3,
			CasesPassed: 3,
		},
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
	if !strings.Contains(text, `"specsPassed": 1`) {
		t.Fatalf("expected spec summary, got %q", text)
	}
}
