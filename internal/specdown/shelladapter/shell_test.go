package shelladapter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corca-ai/specdown/internal/specdown/adapterprotocol"
	"github.com/corca-ai/specdown/internal/specdown/testutil"
)

// --- IsDoctestContent ---

func TestIsDoctestContent(t *testing.T) {
	t.Run("doctest", func(t *testing.T) {
		testutil.True(t, IsDoctestContent("$ echo hello\nhello"))
	})
	t.Run("not doctest", func(t *testing.T) {
		testutil.False(t, IsDoctestContent("echo hello"))
	})
	t.Run("empty lines before dollar", func(t *testing.T) {
		testutil.True(t, IsDoctestContent("\n\n$ true"))
	})
	t.Run("empty input", func(t *testing.T) {
		testutil.False(t, IsDoctestContent(""))
	})
}

// --- ParseDoctestSource ---

func TestParseDoctestSource(t *testing.T) {
	t.Run("single step", func(t *testing.T) {
		steps := ParseDoctestSource("$ echo hello\nhello")
		testutil.Len(t, steps, 1)
		testutil.Equal(t, steps[0].Command, "echo hello")
		testutil.Equal(t, steps[0].Expected, "hello")
	})
	t.Run("multiple steps", func(t *testing.T) {
		steps := ParseDoctestSource("$ echo a\na\n$ echo b\nb")
		testutil.Len(t, steps, 2)
		testutil.Equal(t, steps[0].Command, "echo a")
		testutil.Equal(t, steps[1].Command, "echo b")
	})
	t.Run("no expected output", func(t *testing.T) {
		steps := ParseDoctestSource("$ true")
		testutil.Len(t, steps, 1)
		testutil.Equal(t, steps[0].Expected, "")
	})
}

// --- MatchWithWildcard ---

func TestMatchWithWildcard(t *testing.T) {
	t.Run("exact match", func(t *testing.T) {
		testutil.True(t, MatchWithWildcard("hello", "hello"))
	})
	t.Run("mismatch", func(t *testing.T) {
		testutil.False(t, MatchWithWildcard("hello", "world"))
	})
	t.Run("wildcard matches all", func(t *testing.T) {
		testutil.True(t, MatchWithWildcard("line1\nline2\nline3", "..."))
	})
	t.Run("wildcard in middle", func(t *testing.T) {
		testutil.True(t, MatchWithWildcard("first\nmiddle\nlast", "first\n...\nlast"))
	})
	t.Run("wildcard matches zero lines", func(t *testing.T) {
		testutil.True(t, MatchWithWildcard("first\nlast", "first\n...\nlast"))
	})
}

// --- StepStatus ---

func TestStepStatus(t *testing.T) {
	testutil.Equal(t, StepStatus("hello", "hello"), "passed")
	testutil.Equal(t, StepStatus("hello", "world"), "failed")
}

// --- Exec ---

func TestExec(t *testing.T) {
	t.Run("echo hello", func(t *testing.T) {
		resp := Exec(1, "echo hello")
		output, ok := resp["output"]
		testutil.True(t, ok)
		testutil.Equal(t, output, "hello")
	})
	t.Run("false command", func(t *testing.T) {
		resp := Exec(1, "false")
		_, ok := resp["error"]
		testutil.True(t, ok)
	})
	t.Run("preserves id", func(t *testing.T) {
		resp := Exec(42, "echo ok")
		testutil.Equal(t, resp["id"], 42)
	})
	t.Run("stderr on failure", func(t *testing.T) {
		resp := Exec(1, "echo badness >&2; false")
		testutil.Equal(t, resp["error"], "badness")
	})
	t.Run("trailing newlines stripped", func(t *testing.T) {
		resp := Exec(1, "printf 'hi\n\n'")
		testutil.Equal(t, resp["output"], "hi")
	})
}

// --- Assert ---

func TestAssertNilRequest(t *testing.T) {
	resp := Assert(1, nil, "")
	testutil.Equal(t, resp.Type, "failed")
	testutil.Equal(t, resp.ID, 1)
	testutil.Contains(t, resp.Message, "missing assert request")
}

func TestAssertMissingScript(t *testing.T) {
	checksDir := t.TempDir()
	req := &adapterprotocol.AssertRequest{
		ID:    2,
		Check: "nonexistent",
	}
	resp := Assert(2, req, checksDir)
	testutil.Equal(t, resp.Type, "failed")
	testutil.Contains(t, resp.Message, "check script not found")
}

func TestAssertPassingScript(t *testing.T) {
	checksDir := t.TempDir()
	script := filepath.Join(checksDir, "my-check.sh")
	testutil.NilErr(t, os.WriteFile(script, []byte("#!/bin/sh\necho ok"), 0o755))

	req := &adapterprotocol.AssertRequest{
		ID:    3,
		Check: "my-check",
	}
	resp := Assert(3, req, checksDir)
	testutil.Equal(t, resp.Type, "passed")
	testutil.Equal(t, resp.Actual, "ok")
}

func TestAssertFailingScript(t *testing.T) {
	checksDir := t.TempDir()
	script := filepath.Join(checksDir, "fail-check.sh")
	testutil.NilErr(t, os.WriteFile(script, []byte("#!/bin/sh\necho 'reason' >&2\nexit 1"), 0o755))

	req := &adapterprotocol.AssertRequest{
		ID:    4,
		Check: "fail-check",
	}
	resp := Assert(4, req, checksDir)
	testutil.Equal(t, resp.Type, "failed")
	testutil.Equal(t, resp.Message, "reason")
}

func TestAssertFailingScriptFallsBackToStdout(t *testing.T) {
	checksDir := t.TempDir()
	script := filepath.Join(checksDir, "check.sh")
	testutil.NilErr(t, os.WriteFile(script, []byte("#!/bin/sh\necho 'stdout msg'\nexit 1"), 0o755))

	req := &adapterprotocol.AssertRequest{ID: 5, Check: "check"}
	resp := Assert(5, req, checksDir)
	testutil.Equal(t, resp.Type, "failed")
	testutil.Equal(t, resp.Message, "stdout msg")
}

func TestAssertEnvironmentVariables(t *testing.T) {
	checksDir := t.TempDir()
	// Script prints env vars to verify they are set.
	script := filepath.Join(checksDir, "env-check.sh")
	testutil.NilErr(t, os.WriteFile(script, []byte("#!/bin/sh\necho \"$CHECK_PARAM_FOO|$COL_NAME|$COL_DASH_COL|$CHECK\""), 0o755))

	req := &adapterprotocol.AssertRequest{
		ID:          6,
		Check:       "env-check",
		CheckParams: map[string]string{"foo": "bar"},
		Columns:     []string{"name", "dash-col"},
		Cells:       []string{"alice", "val2"},
	}
	resp := Assert(6, req, checksDir)
	testutil.Equal(t, resp.Type, "passed")
	testutil.Equal(t, resp.Actual, "bar|alice|val2|env-check")
}

func TestAssertColumnsExceedCells(t *testing.T) {
	checksDir := t.TempDir()
	script := filepath.Join(checksDir, "check.sh")
	testutil.NilErr(t, os.WriteFile(script, []byte("#!/bin/sh\necho \"$COL_A|$COL_B\""), 0o755))

	req := &adapterprotocol.AssertRequest{
		ID:      7,
		Check:   "check",
		Columns: []string{"a", "b"},
		Cells:   []string{"val"}, // only 1 cell for 2 columns
	}
	resp := Assert(7, req, checksDir)
	testutil.Equal(t, resp.Type, "passed")
	testutil.Equal(t, resp.Actual, "val|") // second column should get empty value
}

// --- ExecForDoctest ---

func TestExecForDoctest(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		stdout, errMsg, ok := ExecForDoctest("echo hello")
		testutil.True(t, ok)
		testutil.Equal(t, stdout, "hello")
		testutil.Equal(t, errMsg, "")
	})
	t.Run("failure", func(t *testing.T) {
		stdout, errMsg, ok := ExecForDoctest("echo failure >&2; false")
		testutil.False(t, ok)
		testutil.Equal(t, stdout, "")
		testutil.Equal(t, errMsg, "failure")
	})
	t.Run("failure fallback to exit error", func(t *testing.T) {
		_, errMsg, ok := ExecForDoctest("false")
		testutil.False(t, ok)
		testutil.True(t, errMsg != "")
	})
}
