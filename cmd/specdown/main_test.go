package main

import (
	"testing"

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
