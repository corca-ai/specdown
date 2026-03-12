package core

import "testing"

func TestParseBlockSpecSupportsRunBoard(t *testing.T) {
	runBlock, err := parseBlockSpec("run:board")
	if err != nil {
		t.Fatalf("parse run block: %v", err)
	}
	if runBlock.Kind != BlockKindRun || runBlock.Target != "board" || !runBlock.Executable() {
		t.Fatalf("unexpected run block %#v", runBlock)
	}
}

func TestParseBlockSpecSupportsCaptureNames(t *testing.T) {
	block, err := parseBlockSpec("run:board -> $boardName, $boardAlias")
	if err != nil {
		t.Fatalf("parse captured block: %v", err)
	}
	if block.Descriptor() != "run:board" {
		t.Fatalf("unexpected descriptor %q", block.Descriptor())
	}
	if len(block.CaptureNames) != 2 || block.CaptureNames[0] != "boardName" || block.CaptureNames[1] != "boardAlias" {
		t.Fatalf("unexpected captures %#v", block.CaptureNames)
	}
}

func TestParseBlockSpecSupportsGenericRunBlocks(t *testing.T) {
	runBlock, err := parseBlockSpec("run:shell")
	if err != nil {
		t.Fatalf("parse generic run block: %v", err)
	}
	if runBlock.Kind != BlockKindRun || runBlock.Target != "shell" {
		t.Fatalf("unexpected generic run block %#v", runBlock)
	}
}

func TestParseBlockSpecLeavesPlainCodeBlocksNonExecutable(t *testing.T) {
	block, err := parseBlockSpec("go")
	if err != nil {
		t.Fatalf("parse plain code block: %v", err)
	}
	if block.Executable() {
		t.Fatalf("expected non-executable block %#v", block)
	}
}

func TestParseBlockSpecSupportsDoctestContentInRunBlocks(t *testing.T) {
	// doctest: prefix is no longer recognized — content with $ lines is auto-detected at runtime
	block, err := parseBlockSpec("doctest:shell")
	if err != nil {
		t.Fatalf("parse doctest block: %v", err)
	}
	// doctest: is now treated as an unknown prefix (non-executable)
	if block.Executable() {
		t.Fatalf("expected non-executable block for doctest: prefix, got %#v", block)
	}
}

func TestParseBlockSpecSupportsExpectFail(t *testing.T) {
	cases := []struct {
		input  string
		kind   BlockKind
		target string
	}{
		{"run:shell !fail", BlockKindRun, "shell"},
	}
	for _, tc := range cases {
		block, err := parseBlockSpec(tc.input)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.input, err)
		}
		if !block.ExpectFail {
			t.Fatalf("%q: expected ExpectFail=true", tc.input)
		}
		if block.Kind != tc.kind || block.Target != tc.target {
			t.Fatalf("%q: kind=%q target=%q", tc.input, block.Kind, block.Target)
		}
		if block.Descriptor() != string(tc.kind)+":"+tc.target {
			t.Fatalf("%q: descriptor=%q", tc.input, block.Descriptor())
		}
	}
}

func TestParseBlockSpecRejectsExpectFailWithCaptures(t *testing.T) {
	if _, err := parseBlockSpec("run:shell !fail -> $x"); err == nil {
		t.Fatal("expected error for !fail with captures")
	}
}

func TestParseBlockSpecRejectsUnsupportedReservedBlocks(t *testing.T) {
	cases := []string{"run:", "run:board ->", "run:board -> boardName", "alloy:model(board)"}
	for _, input := range cases {
		if _, err := parseBlockSpec(input); err == nil {
			t.Fatalf("expected parse error for %q", input)
		}
	}
}

func TestUnknownBlockPrefix(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"verify:shell", "verify"},
		{"test:webapp", "test"},
		{"example:python", "example"},
		{"run:shell", ""},            // recognized
		{"doctest:shell", "doctest"}, // no longer recognized
		{"alloy:model(x)", ""},       // handled separately
		{"json", ""},                 // no colon
		{"go", ""},                   // no colon
		{"", ""},                     // empty
		{"run:", ""},                 // no target
		{"text/plain", ""},           // non-alpha before colon
		{"http://example", ""},       // non-alpha before colon
	}
	for _, tc := range cases {
		got := unknownBlockPrefix(tc.input)
		if got != tc.want {
			t.Errorf("unknownBlockPrefix(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestParseBlockSpecRejectsDuplicateCaptureNames(t *testing.T) {
	if _, err := parseBlockSpec("run:shell -> $a, $a"); err == nil {
		t.Fatal("expected error for duplicate capture names")
	}
}
