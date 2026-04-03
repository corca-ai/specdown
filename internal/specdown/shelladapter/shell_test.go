package shelladapter

import (
	"testing"

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
	t.Run("escaped wildcard matches literal dots", func(t *testing.T) {
		testutil.True(t, MatchWithWildcard("...", `\...`))
	})
	t.Run("escaped wildcard does not match other text", func(t *testing.T) {
		testutil.False(t, MatchWithWildcard("hello", `\...`))
	})
	t.Run("escaped wildcard with surrounding lines", func(t *testing.T) {
		testutil.True(t, MatchWithWildcard("first\n...\nlast", "first\n\\...\nlast"))
	})
	t.Run("wildcard and escaped wildcard together", func(t *testing.T) {
		testutil.True(t, MatchWithWildcard("a\nb\n...\nc", "a\n...\n\\...\nc"))
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
		resp := Exec(1, "echo hello", "")
		output, ok := resp["output"]
		testutil.True(t, ok)
		testutil.Equal(t, output, "hello")
	})
	t.Run("false command", func(t *testing.T) {
		resp := Exec(1, "false", "")
		_, ok := resp["error"]
		testutil.True(t, ok)
	})
	t.Run("preserves id", func(t *testing.T) {
		resp := Exec(42, "echo ok", "")
		testutil.Equal(t, resp["id"], 42)
	})
	t.Run("stderr on failure", func(t *testing.T) {
		resp := Exec(1, "echo badness >&2; false", "")
		testutil.Equal(t, resp["error"], "badness")
	})
	t.Run("trailing newlines stripped", func(t *testing.T) {
		resp := Exec(1, "printf 'hi\n\n'", "")
		testutil.Equal(t, resp["output"], "hi")
	})
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
