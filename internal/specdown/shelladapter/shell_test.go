package shelladapter

import (
	"testing"
)

func TestIsDoctestContent(t *testing.T) {
	t.Run("doctest", func(t *testing.T) {
		if !IsDoctestContent("$ echo hello\nhello") {
			t.Error("expected true")
		}
	})

	t.Run("not doctest", func(t *testing.T) {
		if IsDoctestContent("echo hello") {
			t.Error("expected false")
		}
	})

	t.Run("empty lines before dollar", func(t *testing.T) {
		if !IsDoctestContent("\n\n$ true") {
			t.Error("expected true")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		if IsDoctestContent("") {
			t.Error("expected false")
		}
	})
}

func TestParseDoctestSource(t *testing.T) {
	t.Run("single step", func(t *testing.T) {
		steps := ParseDoctestSource("$ echo hello\nhello")
		if len(steps) != 1 {
			t.Fatalf("expected 1 step, got %d", len(steps))
		}
		if steps[0].Command != "echo hello" {
			t.Errorf("command = %q", steps[0].Command)
		}
		if steps[0].Expected != "hello" {
			t.Errorf("expected = %q", steps[0].Expected)
		}
	})

	t.Run("multiple steps", func(t *testing.T) {
		steps := ParseDoctestSource("$ echo a\na\n$ echo b\nb")
		if len(steps) != 2 {
			t.Fatalf("expected 2 steps, got %d", len(steps))
		}
		if steps[0].Command != "echo a" || steps[1].Command != "echo b" {
			t.Errorf("commands = %q, %q", steps[0].Command, steps[1].Command)
		}
	})

	t.Run("no expected output", func(t *testing.T) {
		steps := ParseDoctestSource("$ true")
		if len(steps) != 1 {
			t.Fatalf("expected 1 step, got %d", len(steps))
		}
		if steps[0].Expected != "" {
			t.Errorf("expected empty, got %q", steps[0].Expected)
		}
	})
}

func TestMatchWithWildcard(t *testing.T) {
	t.Run("exact match", func(t *testing.T) {
		if !MatchWithWildcard("hello", "hello") {
			t.Error("expected match")
		}
	})

	t.Run("mismatch", func(t *testing.T) {
		if MatchWithWildcard("hello", "world") {
			t.Error("expected no match")
		}
	})

	t.Run("wildcard matches all", func(t *testing.T) {
		if !MatchWithWildcard("line1\nline2\nline3", "...") {
			t.Error("expected match")
		}
	})

	t.Run("wildcard in middle", func(t *testing.T) {
		if !MatchWithWildcard("first\nmiddle\nlast", "first\n...\nlast") {
			t.Error("expected match")
		}
	})

	t.Run("wildcard matches zero lines", func(t *testing.T) {
		if !MatchWithWildcard("first\nlast", "first\n...\nlast") {
			t.Error("expected match with zero skipped lines")
		}
	})
}

func TestStepStatus(t *testing.T) {
	if StepStatus("hello", "hello") != "passed" {
		t.Error("expected passed")
	}
	if StepStatus("hello", "world") != "failed" {
		t.Error("expected failed")
	}
}

func TestExec(t *testing.T) {
	t.Run("echo hello", func(t *testing.T) {
		resp := Exec(1, "echo hello")
		if output, ok := resp["output"]; !ok || output != "hello" {
			t.Errorf("expected output=hello, got %v", resp)
		}
	})

	t.Run("false command", func(t *testing.T) {
		resp := Exec(1, "false")
		if _, ok := resp["error"]; !ok {
			t.Errorf("expected error key, got %v", resp)
		}
	})
}
