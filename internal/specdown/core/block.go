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
	BlockKindTest   BlockKind = "test"
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

	if trimmed == "expect" || strings.HasPrefix(trimmed, "alloy:") {
		return BlockSpec{}, fmt.Errorf("unsupported spec block %q", trimmed)
	}

	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 {
		return BlockSpec{Raw: trimmed}, nil
	}

	kind := BlockKind(parts[0])
	target := strings.TrimSpace(parts[1])
	switch kind {
	case BlockKindRun, BlockKindVerify:
		if target == "" {
			return BlockSpec{}, fmt.Errorf("invalid spec block %q", trimmed)
		}
		return BlockSpec{Raw: trimmed, Kind: kind, Target: target}, nil
	case BlockKindTest:
		if target == "" {
			return BlockSpec{}, fmt.Errorf("invalid spec block %q", trimmed)
		}
		return BlockSpec{Raw: trimmed, Kind: kind, Target: target}, nil
	}

	return BlockSpec{Raw: trimmed}, nil
}
