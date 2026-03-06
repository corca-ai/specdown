package core

import (
	"fmt"
	"strings"
)

type BlockKind string

const (
	BlockKindNone   BlockKind = ""
	BlockKindRun    BlockKind = "run"
	BlockKindVerify BlockKind = "verify"
)

type BlockSpec struct {
	Raw    string
	Kind   BlockKind
	Target string
}

func (b BlockSpec) String() string {
	return b.Raw
}

func (b BlockSpec) Executable() bool {
	return b.Kind != BlockKindNone
}

func parseBlockSpec(info string) (BlockSpec, error) {
	trimmed := strings.TrimSpace(info)
	if trimmed == "" {
		return BlockSpec{}, nil
	}

	switch trimmed {
	case "run:board":
		return BlockSpec{Raw: trimmed, Kind: BlockKindRun, Target: "board"}, nil
	case "verify:board":
		return BlockSpec{Raw: trimmed, Kind: BlockKindVerify, Target: "board"}, nil
	}

	if trimmed == "expect" || strings.HasPrefix(trimmed, "alloy:") {
		return BlockSpec{}, fmt.Errorf("unsupported spec block %q", trimmed)
	}

	parts := strings.SplitN(trimmed, ":", 2)
	kind := BlockKind(parts[0])
	switch kind {
	case BlockKindRun, BlockKindVerify:
		return BlockSpec{}, fmt.Errorf("unsupported spec block %q", trimmed)
	}

	if kind == "test" {
		return BlockSpec{}, fmt.Errorf("unsupported spec block %q", trimmed)
	}

	return BlockSpec{Raw: trimmed}, nil
}
