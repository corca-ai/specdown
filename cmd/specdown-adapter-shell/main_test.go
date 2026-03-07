package main

import "testing"

func TestParseDoctestSource(t *testing.T) {
	source := "$ echo hello\nhello\n$ echo world\nworld"
	steps := parseDoctestSource(source)
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
	steps := parseDoctestSource(source)
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Expected != "a\nb\nc" {
		t.Fatalf("expected multi-line output, got %q", steps[0].Expected)
	}
}

func TestParseDoctestSourceNoOutput(t *testing.T) {
	source := "$ mkdir -p /tmp/test\n$ echo done\ndone"
	steps := parseDoctestSource(source)
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
	steps := parseDoctestSource("just some text")
	if len(steps) != 0 {
		t.Fatalf("expected 0 steps, got %d", len(steps))
	}
}
