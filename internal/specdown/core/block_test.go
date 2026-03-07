package core

import "testing"

func TestParseBlockSpecSupportsRunAndVerifyBoard(t *testing.T) {
	runBlock, err := parseBlockSpec("run:board")
	if err != nil {
		t.Fatalf("parse run block: %v", err)
	}
	if runBlock.Kind != BlockKindRun || runBlock.Target != "board" || !runBlock.Executable() {
		t.Fatalf("unexpected run block %#v", runBlock)
	}

	verifyBlock, err := parseBlockSpec("verify:board")
	if err != nil {
		t.Fatalf("parse verify block: %v", err)
	}
	if verifyBlock.Kind != BlockKindVerify || verifyBlock.Target != "board" || !verifyBlock.Executable() {
		t.Fatalf("unexpected verify block %#v", verifyBlock)
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

func TestParseBlockSpecSupportsGenericRunVerifyAndTestBlocks(t *testing.T) {
	runBlock, err := parseBlockSpec("run:shell")
	if err != nil {
		t.Fatalf("parse generic run block: %v", err)
	}
	if runBlock.Kind != BlockKindRun || runBlock.Target != "shell" {
		t.Fatalf("unexpected generic run block %#v", runBlock)
	}

	verifyBlock, err := parseBlockSpec("verify:http")
	if err != nil {
		t.Fatalf("parse generic verify block: %v", err)
	}
	if verifyBlock.Kind != BlockKindVerify || verifyBlock.Target != "http" {
		t.Fatalf("unexpected generic verify block %#v", verifyBlock)
	}

	testBlock, err := parseBlockSpec("test:webapp")
	if err != nil {
		t.Fatalf("parse test block: %v", err)
	}
	if testBlock.Kind != BlockKindTest || testBlock.Target != "webapp" {
		t.Fatalf("unexpected test block %#v", testBlock)
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

func TestParseBlockSpecSupportsDoctestBlocks(t *testing.T) {
	block, err := parseBlockSpec("doctest:shell")
	if err != nil {
		t.Fatalf("parse doctest block: %v", err)
	}
	if block.Kind != BlockKindDoctest || block.Target != "shell" || !block.Executable() {
		t.Fatalf("unexpected doctest block %#v", block)
	}
	if block.Descriptor() != "doctest:shell" {
		t.Fatalf("unexpected descriptor %q", block.Descriptor())
	}
}

func TestParseBlockSpecRejectsDoctestWithCaptures(t *testing.T) {
	if _, err := parseBlockSpec("doctest:shell -> $x"); err == nil {
		t.Fatal("expected error for doctest with captures")
	}
}

func TestParseBlockSpecSupportsExpectFail(t *testing.T) {
	cases := []struct {
		input  string
		kind   BlockKind
		target string
	}{
		{"run:shell !fail", BlockKindRun, "shell"},
		{"verify:shell !fail", BlockKindVerify, "shell"},
		{"doctest:shell !fail", BlockKindDoctest, "shell"},
		{"test:webapp !fail", BlockKindTest, "webapp"},
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
	cases := []string{"run:", "verify:", "test:", "run:board ->", "run:board -> boardName", "expect", "alloy:model(board)"}
	for _, input := range cases {
		if _, err := parseBlockSpec(input); err == nil {
			t.Fatalf("expected parse error for %q", input)
		}
	}
}
