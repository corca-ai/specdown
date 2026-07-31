package adapterhost

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

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

func TestResolveCommandPreservesDotPrefixedNonPathArguments(t *testing.T) {
	baseDir := "/workspace/project"
	command := []string{"sh", "-c", ". ./env && exec ./adapter"}

	resolved := resolveCommand(baseDir, command)
	if !reflect.DeepEqual(resolved, command) {
		t.Fatalf("resolved command = %#v, want argv preserved as %#v", resolved, command)
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

func TestBuiltinShellTimeoutTerminatesProcessTree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the built-in shell adapter requires a POSIX shell")
	}
	host := Host{BaseDir: t.TempDir()}
	session, err := host.StartBuiltinShellSession(shellAdapter())
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	marker := filepath.Join(host.BaseDir, "late-marker")

	start := time.Now()
	resp, err := session.Exec("sleep 5; touch "+marker, 100)
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if !strings.HasPrefix(resp.Error, `timeout after 100ms (exec: "sleep 5; touch `) {
		t.Fatalf("unexpected error %q", resp.Error)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("timeout returned after %s, want under 2s", elapsed)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("timed-out shell child survived and created %s", marker)
	}
}

func TestBuiltinJQTimeoutStopsEvaluation(t *testing.T) {
	session, err := (Host{}).StartBuiltinJQSession(config.AdapterConfig{
		Name:      "jq",
		BuiltinJQ: true,
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	start := time.Now()
	resp, err := session.Assert("jq", map[string]string{
		"input":    `{}`,
		"expr":     "def forever: forever; forever",
		"expected": "never",
	}, nil, nil, 100)
	if err != nil {
		t.Fatalf("assert: %v", err)
	}
	if resp.Message != `timeout after 100ms (assert: check "jq")` {
		t.Fatalf("unexpected message %q", resp.Message)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("jq timeout returned after %s, want under 2s", elapsed)
	}
}

func TestExternalAdapterExecTimeoutTerminatesProcess(t *testing.T) {
	session := startIgnoringAdapter(t)

	start := time.Now()
	resp, err := session.Exec("ignored", 100)
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if resp.Error != `timeout after 100ms (exec: "ignored")` {
		t.Fatalf("unexpected error %q", resp.Error)
	}
	assertSessionTerminatedPromptly(t, session, start)
}

func TestExternalAdapterAssertTimeoutTerminatesProcess(t *testing.T) {
	session := startIgnoringAdapter(t)

	start := time.Now()
	resp, err := session.Assert("stuck", nil, nil, nil, 100)
	if err != nil {
		t.Fatalf("assert: %v", err)
	}
	if resp.Message != `timeout after 100ms (assert: check "stuck")` {
		t.Fatalf("unexpected message %q", resp.Message)
	}
	assertSessionTerminatedPromptly(t, session, start)
}

func TestExternalAdapterFinalResponseIsReadBeforeProcessExit(t *testing.T) {
	host := Host{BaseDir: t.TempDir()}
	session, err := host.StartSession(config.AdapterConfig{
		Name:    "responds-then-exits",
		Command: []string{os.Args[0], "-test.run=^TestRespondingAdapterProcess$", "--", "adapter-response-helper"},
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	resp, err := session.Exec("hello", 1000)
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if !resp.HasOutput || string(resp.Output) != `"final response"` {
		t.Fatalf("unexpected response %+v", resp)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestExternalAdapterAcceptsResponseAtOneMegabyteLimit(t *testing.T) {
	host := Host{BaseDir: t.TempDir()}
	session, err := host.StartSession(config.AdapterConfig{
		Name:    "one-megabyte-response",
		Command: []string{os.Args[0], "-test.run=^TestResponseSizeLimitAdapterProcess$", "--", "adapter-size-limit-helper"},
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	resp, err := session.Exec("hello", 1000)
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if !resp.HasOutput {
		t.Fatalf("response = %+v, want output at documented size limit", resp)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestExternalAdapterAssertRejectsInvalidProtocolResponse(t *testing.T) {
	host := Host{BaseDir: t.TempDir()}
	session, err := host.StartSession(config.AdapterConfig{
		Name:    "invalid-assert",
		Command: []string{os.Args[0], "-test.run=^TestInvalidAssertAdapterProcess$", "--", "adapter-invalid-assert-helper"},
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	_, err = session.Assert("check", nil, nil, nil, 1000)
	if err == nil || !strings.Contains(err.Error(), `assert response type must be "passed" or "failed"`) {
		t.Fatalf("assert error = %v, want protocol type diagnostic", err)
	}
	if closeErr := session.Close(); closeErr != nil {
		t.Fatalf("close: %v", closeErr)
	}
}

func startIgnoringAdapter(t *testing.T) *Session {
	t.Helper()
	host := Host{BaseDir: t.TempDir()}
	session, err := host.StartSession(config.AdapterConfig{
		Name:    "ignores-eof",
		Command: []string{os.Args[0], "-test.run=^TestIgnoringAdapterProcess$", "--", "adapter-helper"},
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	return session
}

func assertSessionTerminatedPromptly(t *testing.T, session *Session, start time.Time) {
	t.Helper()
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("timeout cleanup returned after %s, want under 2s", elapsed)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if session.cmd.ProcessState == nil {
		t.Fatal("adapter process was not reaped")
	}
}

func TestIgnoringAdapterProcess(t *testing.T) {
	if len(os.Args) == 0 || os.Args[len(os.Args)-1] != "adapter-helper" {
		return
	}
	time.Sleep(30 * time.Second)
}

func TestRespondingAdapterProcess(t *testing.T) {
	if len(os.Args) == 0 || os.Args[len(os.Args)-1] != "adapter-response-helper" {
		return
	}
	if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
		os.Exit(1)
	}
	_, _ = fmt.Fprintln(os.Stdout, `{"id":1,"output":"final response"}`)
}

func TestInvalidAssertAdapterProcess(t *testing.T) {
	if len(os.Args) == 0 || os.Args[len(os.Args)-1] != "adapter-invalid-assert-helper" {
		return
	}
	if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
		os.Exit(1)
	}
	_, _ = fmt.Fprintln(os.Stdout, `{"id":1,"type":"unknown"}`)
	os.Exit(0)
}

func TestResponseSizeLimitAdapterProcess(t *testing.T) {
	if len(os.Args) == 0 || os.Args[len(os.Args)-1] != "adapter-size-limit-helper" {
		return
	}
	if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
		os.Exit(1)
	}

	const lineSize = 1024 * 1024
	const prefix = `{"id":1,"output":"`
	const suffix = `"}`
	payloadSize := lineSize - len(prefix) - len(suffix)
	if _, err := os.Stdout.WriteString(prefix + strings.Repeat("x", payloadSize) + suffix + "\n"); err != nil {
		os.Exit(1)
	}
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
