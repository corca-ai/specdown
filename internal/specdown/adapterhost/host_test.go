package adapterhost

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestResolveCommandPreservesAbsolutePaths(t *testing.T) {
	baseDir := "/workspace/project"
	command := []string{"/usr/bin/python3", "/tmp/adapter.py"}

	resolved := resolveCommand(baseDir, command)
	if !reflect.DeepEqual(resolved, command) {
		t.Fatalf("unexpected resolved command %#v", resolved)
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
