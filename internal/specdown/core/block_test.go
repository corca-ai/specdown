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

func TestParseBlockSpecLeavesPlainCodeBlocksNonExecutable(t *testing.T) {
	block, err := parseBlockSpec("go")
	if err != nil {
		t.Fatalf("parse plain code block: %v", err)
	}
	if block.Executable() {
		t.Fatalf("expected non-executable block %#v", block)
	}
}

func TestParseBlockSpecRejectsUnsupportedReservedBlocks(t *testing.T) {
	cases := []string{"run:shell", "verify:http", "test:webapp", "expect", "alloy:model(board)"}
	for _, input := range cases {
		if _, err := parseBlockSpec(input); err == nil {
			t.Fatalf("expected parse error for %q", input)
		}
	}
}
