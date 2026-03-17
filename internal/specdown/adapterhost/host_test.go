package adapterhost

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/corca-ai/specdown/internal/specdown/config"
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

func TestResolveCommandResolvesFirstArgWithDotPrefix(t *testing.T) {
	baseDir := "/workspace/project"
	command := []string{"./bin/adapter", "arg1"}

	resolved := resolveCommand(baseDir, command)
	want := []string{filepath.Clean("/workspace/project/bin/adapter"), "arg1"}
	if !reflect.DeepEqual(resolved, want) {
		t.Fatalf("unexpected resolved command %#v", resolved)
	}
}

func TestResolveCommandDoesNotMutateOriginal(t *testing.T) {
	baseDir := "/workspace/project"
	original := []string{"python3", "./tools/adapter.py"}
	saved := append([]string(nil), original...)

	resolveCommand(baseDir, original)
	if !reflect.DeepEqual(original, saved) {
		t.Fatalf("original slice was mutated: %#v", original)
	}
}

func shellAdapter() config.AdapterConfig {
	return config.AdapterConfig{
		Name:         "shell",
		BuiltinShell: true,
		Blocks:       []string{"run:shell"},
	}
}

func TestBuiltinShellSessionExec(t *testing.T) {
	host := Host{BaseDir: t.TempDir()}
	session, err := host.StartBuiltinShellSession(shellAdapter())
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	defer func() { _ = session.Close() }()

	resp, err := session.Exec("echo hello", 5000)
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if !resp.HasOutput {
		t.Fatal("expected output")
	}
	var output string
	if err := json.Unmarshal(resp.Output, &output); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if output != "hello" {
		t.Fatalf("unexpected output %q", output)
	}
}

func TestBuiltinShellSessionExecError(t *testing.T) {
	host := Host{BaseDir: t.TempDir()}
	session, err := host.StartBuiltinShellSession(shellAdapter())
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	defer func() { _ = session.Close() }()

	resp, err := session.Exec("exit 1", 5000)
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if resp.Error == "" {
		t.Fatal("expected error response")
	}
}

func TestBuiltinShellSessionExecMultipleRequests(t *testing.T) {
	host := Host{BaseDir: t.TempDir()}
	session, err := host.StartBuiltinShellSession(shellAdapter())
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	defer func() { _ = session.Close() }()

	for i := 0; i < 3; i++ {
		resp, err := session.Exec("echo ok", 5000)
		if err != nil {
			t.Fatalf("exec %d: %v", i, err)
		}
		if resp.Error != "" {
			t.Fatalf("exec %d error: %s", i, resp.Error)
		}
		if resp.ID != i+1 {
			t.Fatalf("exec %d: unexpected id %d", i, resp.ID)
		}
	}
}

func TestBuiltinShellSessionAssert(t *testing.T) {
	dir := t.TempDir()
	checksDir := filepath.Join(dir, "checks")
	writeCheckScript(t, checksDir, "status", "#!/bin/sh\nexit 0\n")

	adapter := config.AdapterConfig{
		Name:         "shell",
		BuiltinShell: true,
		Blocks:       []string{"run:shell"},
		Checks:       []string{"status"},
		ChecksDir:    checksDir,
	}
	host := Host{BaseDir: dir}
	session, err := host.StartBuiltinShellSession(adapter)
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	defer func() { _ = session.Close() }()

	resp, err := session.Assert("status", nil, nil, nil, 5000)
	if err != nil {
		t.Fatalf("assert: %v", err)
	}
	if resp.Type != "passed" {
		t.Fatalf("unexpected type %q, message: %s", resp.Type, resp.Message)
	}
}

func TestBuiltinShellSessionAssertFailure(t *testing.T) {
	dir := t.TempDir()
	checksDir := filepath.Join(dir, "checks")
	writeCheckScript(t, checksDir, "fail-check", "#!/bin/sh\necho 'expected failure' >&2\nexit 1\n")

	adapter := config.AdapterConfig{
		Name:         "shell",
		BuiltinShell: true,
		Blocks:       []string{"run:shell"},
		Checks:       []string{"fail-check"},
		ChecksDir:    checksDir,
	}
	host := Host{BaseDir: dir}
	session, err := host.StartBuiltinShellSession(adapter)
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	defer func() { _ = session.Close() }()

	resp, err := session.Assert("fail-check", nil, nil, nil, 5000)
	if err != nil {
		t.Fatalf("assert: %v", err)
	}
	if resp.Type != "failed" {
		t.Fatalf("unexpected type %q", resp.Type)
	}
	if resp.Message == "" {
		t.Fatal("expected failure message")
	}
}

func TestBuiltinShellSessionCloseIsIdempotent(t *testing.T) {
	host := Host{BaseDir: t.TempDir()}
	session, err := host.StartBuiltinShellSession(shellAdapter())
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	if err := session.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestExecTimeoutReturnsSyntheticError(t *testing.T) {
	// Use a pipe-based session to test the timeout path without spawning
	// a real long-running process that cannot be interrupted.
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, _ := io.Pipe() // never write — simulates a slow adapter

	done := make(chan struct{})
	go func() {
		defer close(done)
		// drain stdin so encoder doesn't block
		buf := make([]byte, 4096)
		for {
			if _, err := stdinReader.Read(buf); err != nil {
				return
			}
		}
	}()

	scanner := bufio.NewScanner(stdoutReader)
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	session := &Session{
		adapter: shellAdapter(),
		stdin:   stdinWriter,
		scanner: scanner,
		encoder: json.NewEncoder(stdinWriter),
		stderr:  &bytes.Buffer{},
		builtin: true,
		done:    done,
	}

	resp, err := session.Exec("ignored", 100)
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if resp.Error != `timeout after 100ms (exec: "ignored")` {
		t.Fatalf("unexpected error %q", resp.Error)
	}

	// Clean up: close stdin to unblock drain goroutine, then close pipes
	_ = stdinWriter.Close()
	_ = stdinReader.Close()
	_ = stdoutReader.Close()
}

func TestBuiltinShellSessionExecNoTimeout(t *testing.T) {
	host := Host{BaseDir: t.TempDir()}
	session, err := host.StartBuiltinShellSession(shellAdapter())
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	defer func() { _ = session.Close() }()

	// timeoutMs=0 means no timeout
	resp, err := session.Exec("echo notimeout", 0)
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
}

func TestHandleBuiltinMessageExec(t *testing.T) {
	raw := []byte(`{"type":"exec","id":1,"source":"echo test"}`)
	var results []json.RawMessage
	encoder := json.NewEncoder(&capturingWriter{results: &results})

	if err := handleBuiltinMessage(raw, encoder, ""); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(results[0], &fields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := fields["output"]; !ok {
		if errRaw, ok := fields["error"]; ok {
			t.Fatalf("expected output key, got error: %s", errRaw)
		}
		t.Fatal("expected output key in response")
	}
}

func TestHandleBuiltinMessageAssertMissingScript(t *testing.T) {
	raw := []byte(`{"type":"assert","id":1,"check":"nonexistent","columns":[],"cells":[]}`)
	var results []json.RawMessage
	encoder := json.NewEncoder(&capturingWriter{results: &results})

	if err := handleBuiltinMessage(raw, encoder, "/nonexistent/dir"); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	var resp struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(results[0], &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Type != "failed" {
		t.Fatalf("expected failed, got %q", resp.Type)
	}
}

func TestHandleBuiltinMessageUnknownType(t *testing.T) {
	raw := []byte(`{"type":"unknown","id":1}`)
	var results []json.RawMessage
	encoder := json.NewEncoder(&capturingWriter{results: &results})

	err := handleBuiltinMessage(raw, encoder, "")
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestHandleBuiltinMessageInvalidJSON(t *testing.T) {
	raw := []byte(`not json`)
	var results []json.RawMessage
	encoder := json.NewEncoder(&capturingWriter{results: &results})

	err := handleBuiltinMessage(raw, encoder, "")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleBuiltinMessageMissingType(t *testing.T) {
	raw := []byte(`{"id":1}`)
	var results []json.RawMessage
	encoder := json.NewEncoder(&capturingWriter{results: &results})

	err := handleBuiltinMessage(raw, encoder, "")
	if err == nil {
		t.Fatal("expected error for missing type")
	}
}

func TestBuiltinShellSessionAssertWithColumnsAndCells(t *testing.T) {
	dir := t.TempDir()
	checksDir := filepath.Join(dir, "checks")
	// Script that prints the COL_NAME env var as actual value
	writeCheckScript(t, checksDir, "echo-col", "#!/bin/sh\necho \"$COL_NAME\"\nexit 0\n")

	adapter := config.AdapterConfig{
		Name:         "shell",
		BuiltinShell: true,
		Blocks:       []string{"run:shell"},
		Checks:       []string{"echo-col"},
		ChecksDir:    checksDir,
	}
	host := Host{BaseDir: dir}
	session, err := host.StartBuiltinShellSession(adapter)
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	defer func() { _ = session.Close() }()

	resp, err := session.Assert("echo-col", nil, []string{"name"}, []string{"Alice"}, 5000)
	if err != nil {
		t.Fatalf("assert: %v", err)
	}
	if resp.Type != "passed" {
		t.Fatalf("unexpected type %q, message: %s", resp.Type, resp.Message)
	}
	if resp.Actual != "Alice" {
		t.Fatalf("unexpected actual %q", resp.Actual)
	}
}

// capturingWriter collects each JSON line written by an encoder.
type capturingWriter struct {
	results *[]json.RawMessage
}

func (w *capturingWriter) Write(p []byte) (int, error) {
	*w.results = append(*w.results, append(json.RawMessage(nil), p...))
	return len(p), nil
}

func writeCheckScript(t *testing.T, checksDir, name, content string) {
	t.Helper()
	if err := mkdirAll(checksDir); err != nil {
		t.Fatalf("mkdir checks: %v", err)
	}
	path := filepath.Join(checksDir, name+".sh")
	if err := writeFile(path, content); err != nil {
		t.Fatalf("write check script: %v", err)
	}
}

func mkdirAll(path string) error {
	return os.MkdirAll(path, 0o755)
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o755)
}
