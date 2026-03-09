package main

import (
	"testing"

	"github.com/corca-ai/specdown/internal/specdown/shelladapter"
)

func TestParseDoctestSource(t *testing.T) {
	source := "$ echo hello\nhello\n$ echo world\nworld"
	steps := shelladapter.ParseDoctestSource(source)
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if steps[0].Command != "echo hello" || steps[0].Expected != "hello" {
		t.Fatalf("step 0: command=%q expected=%q", steps[0].Command, steps[0].Expected)
	}
	if steps[1].Command != "echo world" || steps[1].Expected != "world" {
		t.Fatalf("step 1: command=%q expected=%q", steps[1].Command, steps[1].Expected)
	}
}

func TestParseDoctestSourceMultilineOutput(t *testing.T) {
	source := "$ printf 'a\\nb\\nc'\na\nb\nc"
	steps := shelladapter.ParseDoctestSource(source)
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Expected != "a\nb\nc" {
		t.Fatalf("expected multi-line output, got %q", steps[0].Expected)
	}
}

func TestParseDoctestSourceNoOutput(t *testing.T) {
	source := "$ mkdir -p /tmp/test\n$ echo done\ndone"
	steps := shelladapter.ParseDoctestSource(source)
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if steps[0].Command != "mkdir -p /tmp/test" || steps[0].Expected != "" {
		t.Fatalf("step 0: command=%q expected=%q", steps[0].Command, steps[0].Expected)
	}
	if steps[1].Command != "echo done" || steps[1].Expected != "done" {
		t.Fatalf("step 1: command=%q expected=%q", steps[1].Command, steps[1].Expected)
	}
}

func TestParseDoctestSourceEmpty(t *testing.T) {
	steps := shelladapter.ParseDoctestSource("just some text")
	if len(steps) != 0 {
		t.Fatalf("expected 0 steps, got %d", len(steps))
	}
}

func TestMatchWithWildcard(t *testing.T) {
	tests := []struct {
		name     string
		actual   string
		expected string
		match    bool
	}{
		{"exact match", "hello", "hello", true},
		{"exact mismatch", "hello", "world", false},
		{"wildcard only", "anything\ngoes\nhere", "...", true},
		{"wildcard at end", "hello\nworld\nextra", "hello\n...", true},
		{"wildcard at start", "extra\nhello\nworld", "...\nworld", true},
		{"wildcard in middle", "hello\nskip\nthis\nworld", "hello\n...\nworld", true},
		{"wildcard no middle lines", "hello\nworld", "hello\n...\nworld", true},
		{"wildcard mismatch", "hello\nskip\nfoo", "hello\n...\nworld", false},
		{"multiple wildcards", "a\nb\nc\nd\ne", "a\n...\nc\n...\ne", true},
		{"no wildcard exact", "a\nb", "a\nb", true},
		{"no wildcard mismatch", "a\nb", "a\nc", false},
		{"empty both", "", "", true},
		{"wildcard empty actual", "", "...", true},
		{"leading wildcard match", "x\ny\nhello", "...\nhello", true},
		{"trailing exact required", "hello\nworld", "hello\n...\nfoo", false},
		{"consecutive wildcards", "a\nb\nc", "a\n...\n...\nc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shelladapter.MatchWithWildcard(tt.actual, tt.expected)
			if got != tt.match {
				t.Errorf("MatchWithWildcard(%q, %q) = %v, want %v", tt.actual, tt.expected, got, tt.match)
			}
		})
	}
}
