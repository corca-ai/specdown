package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/corca-ai/specdown/internal/specdown/core"
	"github.com/corca-ai/specdown/internal/specdown/trace"
)

func TestCaseTag(t *testing.T) {
	tests := []struct {
		name       string
		status     core.Status
		expectFail bool
		want       string
	}{
		{"failed with expectFail", core.StatusFailed, true, "XFAIL"},
		{"failed without expectFail", core.StatusFailed, false, "FAIL"},
		{"passed", core.StatusPassed, false, "PASS"},
		{"passed with expectFail", core.StatusPassed, true, "PASS"},
		{"skipped", core.StatusSkipped, false, "SKIP"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := caseTag(tt.status, tt.expectFail)
			if got != tt.want {
				t.Errorf("caseTag(%q, %v) = %q, want %q", tt.status, tt.expectFail, got, tt.want)
			}
		})
	}
}

func TestHasHelpFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"empty", nil, false},
		{"no help", []string{"run", "-config", "x.json"}, false},
		{"-help", []string{"-help"}, true},
		{"--help", []string{"--help"}, true},
		{"-h", []string{"-h"}, true},
		{"help in middle", []string{"run", "-help", "extra"}, true},
		{"not a flag", []string{"help"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasHelpFlag(tt.args)
			if got != tt.want {
				t.Errorf("hasHelpFlag(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestResolvePath(t *testing.T) {
	tests := []struct {
		name    string
		baseDir string
		value   string
		want    string
	}{
		{"absolute path unchanged", "/base", "/abs/path", "/abs/path"},
		{"relative joined", "/base", "rel/path", "/base/rel/path"},
		{"relative cleaned", "/base", "rel/../other", "/base/other"},
		{"dot path", "/base", ".", "/base"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePath(tt.baseDir, tt.value)
			if got != tt.want {
				t.Errorf("resolvePath(%q, %q) = %q, want %q", tt.baseDir, tt.value, got, tt.want)
			}
		})
	}
}

func TestJsonReportPath(t *testing.T) {
	got := jsonReportPath("/out/report")
	want := "/out/report/report.json"
	if got != want {
		t.Errorf("jsonReportPath(%q) = %q, want %q", "/out/report", got, want)
	}
}

func TestReportFailedIncludesLifecycleFailures(t *testing.T) {
	report := core.Report{
		Summary: core.Summary{
			SpecsFailed:     0,
			CasesFailed:     0,
			LifecycleFailed: 1,
		},
	}
	if !reportFailed(report) {
		t.Fatal("reportFailed = false, want true for a lifecycle failure")
	}
}

func TestFailureSummaryPreservesCaseOnlyFormat(t *testing.T) {
	report := core.Report{Summary: core.Summary{
		SpecsFailed: 1,
		CasesFailed: 2,
	}}
	got := failureSummary(report, 12*time.Millisecond)
	want := "FAIL 1 spec(s), 2 case(s) in 12ms"
	if got != want {
		t.Fatalf("failure summary = %q, want %q", got, want)
	}
}

func TestFailureSummaryIncludesLifecycleFailuresWhenPresent(t *testing.T) {
	report := core.Report{Summary: core.Summary{
		SpecsFailed:     1,
		SpecsSkipped:    3,
		CasesFailed:     0,
		CasesSkipped:    2,
		LifecycleFailed: 1,
	}}
	got := failureSummary(report, 12*time.Millisecond)
	want := "FAIL 1 spec(s), 0 case(s), 1 lifecycle failure(s), 3 spec(s) skipped, 2 case(s) skipped in 12ms"
	if got != want {
		t.Fatalf("failure summary = %q, want %q", got, want)
	}
}

func TestRunSetupOnlyFailureWritesFirstClassReport(t *testing.T) {
	root := t.TempDir()
	helperCommand := strconv.Quote(os.Args[0]) + " -test.run=^TestCLILifecycleFailureProcess$ -- cli-lifecycle-fail"
	configPath := filepath.Join(root, "specdown.json")
	configBody := `{"entry":"index.md","setup":` + strconv.Quote(helperCommand) +
		`,"reporters":[{"builtin":"json","outFile":"report/report.json"}]}`
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	reportDir := filepath.Join(root, "report")

	cmd := command{stdout: io.Discard, stderr: io.Discard, workingDir: root}
	err := cmd.run(context.Background(), []string{
		"-config", configPath,
		"-setup",
		"-quiet",
		"-out", reportDir,
	})
	if err == nil || !strings.Contains(err.Error(), "spec run failed") {
		t.Fatalf("run error = %v, want spec run failure", err)
	}
	body, readErr := os.ReadFile(filepath.Join(reportDir, "report.json"))
	if readErr != nil {
		t.Fatalf("read JSON report after %v: %v", err, readErr)
	}
	var report core.Report
	if unmarshalErr := json.Unmarshal(body, &report); unmarshalErr != nil {
		t.Fatalf("unmarshal JSON report: %v", unmarshalErr)
	}
	if report.Summary.LifecycleFailed != 1 {
		t.Fatalf("summary = %+v, want one lifecycle failure", report.Summary)
	}
	if _, statErr := os.Stat(filepath.Join(reportDir, "index.html")); statErr != nil {
		t.Fatalf("HTML lifecycle landing page: %v", statErr)
	}
}

func TestRunWritesLifecycleReportAlongsideDiscoveryError(t *testing.T) {
	root := t.TempDir()
	helperCommand := strconv.Quote(os.Args[0]) + " -test.run=^TestCLILifecycleFailureProcess$ -- cli-lifecycle-fail"
	configPath := filepath.Join(root, "specdown.json")
	configBody := `{"entry":"missing.md","setup":` + strconv.Quote(helperCommand) +
		`,"teardown":` + strconv.Quote(helperCommand) +
		`,"reporters":[{"builtin":"json","outFile":"report/report.json"}]}`
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	reportDir := filepath.Join(root, "report")

	cmd := command{stdout: io.Discard, stderr: io.Discard, workingDir: root}
	err := cmd.run(context.Background(), []string{
		"-config", configPath,
		"-quiet",
		"-out", reportDir,
	})
	if err == nil || !strings.Contains(err.Error(), "setup command failed") {
		t.Fatalf("run error = %v, want setup/discovery error", err)
	}
	body, readErr := os.ReadFile(filepath.Join(reportDir, "report.json"))
	if readErr != nil {
		t.Fatalf("read JSON report: %v", readErr)
	}
	var report core.Report
	if unmarshalErr := json.Unmarshal(body, &report); unmarshalErr != nil {
		t.Fatalf("unmarshal JSON report: %v", unmarshalErr)
	}
	if report.Summary.LifecycleFailed != 2 || len(report.LifecycleEvents) != 2 {
		t.Fatalf("report = %+v, want setup and teardown failures", report)
	}
	if _, statErr := os.Stat(filepath.Join(reportDir, "index.html")); statErr != nil {
		t.Fatalf("HTML lifecycle landing page: %v", statErr)
	}
}

func TestCLILifecycleFailureProcess(_ *testing.T) {
	if len(os.Args) > 0 && os.Args[len(os.Args)-1] == "cli-lifecycle-fail" {
		os.Exit(7)
	}
}

func TestBuildEdgeLookup(t *testing.T) {
	graph := trace.Graph{
		DirectEdges: []trace.Edge{
			{Source: "a.md", Target: "b.md", EdgeName: "depends"},
		},
		TransitiveEdges: []trace.Edge{
			{Source: "a.md", Target: "c.md", EdgeName: "depends"},
		},
	}
	lookup := buildEdgeLookup(graph)

	tests := []struct {
		name     string
		src, tgt string
		want     string
	}{
		{"direct edge", "a.md", "b.md", "depends"},
		{"transitive edge", "a.md", "c.md", "(depends)"},
		{"no edge", "b.md", "c.md", "."},
		{"self", "a.md", "a.md", "."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lookup(tt.src, tt.tgt)
			if got != tt.want {
				t.Errorf("lookup(%q, %q) = %q, want %q", tt.src, tt.tgt, got, tt.want)
			}
		})
	}
}

func TestBuildEdgeLookupEmpty(t *testing.T) {
	lookup := buildEdgeLookup(trace.Graph{})
	if got := lookup("x", "y"); got != "." {
		t.Errorf("empty graph lookup = %q, want %q", got, ".")
	}
}

func TestBuildEdgeLookupDirectTakesPrecedence(t *testing.T) {
	graph := trace.Graph{
		DirectEdges: []trace.Edge{
			{Source: "a.md", Target: "b.md", EdgeName: "direct"},
		},
		TransitiveEdges: []trace.Edge{
			{Source: "a.md", Target: "b.md", EdgeName: "transitive"},
		},
	}
	lookup := buildEdgeLookup(graph)
	got := lookup("a.md", "b.md")
	if got != "direct" {
		t.Errorf("expected direct edge to take precedence, got %q", got)
	}
}
