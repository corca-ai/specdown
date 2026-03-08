package adapterhost

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/corca-ai/specdown/internal/specdown/adapterprotocol"
	"github.com/corca-ai/specdown/internal/specdown/core"
)

func TestResolveCommandPreservesAbsolutePaths(t *testing.T) {
	baseDir := "/workspace/project"
	command := []string{"/usr/bin/python3", "/tmp/adapter.py"}

	resolved := resolveCommand(baseDir, command)
	if !reflect.DeepEqual(resolved, command) {
		t.Fatalf("unexpected resolved command %#v", resolved)
	}
}

func TestApplyResponsePropagatesExpectedActualLabel(t *testing.T) {
	result := &core.CaseResult{
		ID:    core.SpecID{HeadingPath: []string{"A"}},
		Label: "default-label",
	}
	response := adapterprotocol.Response{
		ID:       1,
		Type:     "failed",
		Message:  "mismatch",
		Expected: "foo",
		Actual:   "bar",
		Label:    "custom-label",
	}
	if err := applyResponse(result, 1, response); err != nil {
		t.Fatalf("applyResponse: %v", err)
	}
	if result.Expected != "foo" {
		t.Fatalf("expected %q, got %q", "foo", result.Expected)
	}
	if result.Actual != "bar" {
		t.Fatalf("expected %q, got %q", "bar", result.Actual)
	}
	if result.Label != "custom-label" {
		t.Fatalf("expected %q, got %q", "custom-label", result.Label)
	}
	if result.Message != "mismatch" {
		t.Fatalf("expected %q, got %q", "mismatch", result.Message)
	}
}

func TestApplyResponseKeepsDefaultLabelWhenEmpty(t *testing.T) {
	result := &core.CaseResult{
		ID:    core.SpecID{HeadingPath: []string{"A"}},
		Label: "default-label",
	}
	response := adapterprotocol.Response{
		ID:      1,
		Type:    "failed",
		Message: "error",
	}
	if err := applyResponse(result, 1, response); err != nil {
		t.Fatalf("applyResponse: %v", err)
	}
	if result.Label != "default-label" {
		t.Fatalf("expected default label preserved, got %q", result.Label)
	}
}

func TestResolveCommandResolvesRelativePathsAgainstBaseDir(t *testing.T) {
	baseDir := "/workspace/project"
	command := []string{"python3", "./tools/adapter.py"}

	resolved := resolveCommand(baseDir, command)
	want := []string{"python3", filepath.Clean("/workspace/project/tools/adapter.py")}
	if !reflect.DeepEqual(resolved, want) {
		t.Fatalf("unexpected resolved command %#v", resolved)
	}
}
