package core

import (
	"fmt"
	"regexp"
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
	Raw          string
	Kind         BlockKind
	Target       string
	CaptureNames []string
}

func (b BlockSpec) String() string {
	return b.Raw
}

func (b BlockSpec) Descriptor() string {
	if b.Kind == BlockKindNone {
		return b.Raw
	}
	return string(b.Kind) + ":" + b.Target
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

	infoPart := trimmed
	var captureNames []string
	if strings.Contains(trimmed, "->") {
		parts := strings.SplitN(trimmed, "->", 2)
		infoPart = strings.TrimSpace(parts[0])
		names, err := parseCaptureNames(parts[1])
		if err != nil {
			return BlockSpec{}, err
		}
		captureNames = names
	}

	parts := strings.SplitN(infoPart, ":", 2)
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
		return BlockSpec{Raw: trimmed, Kind: kind, Target: target, CaptureNames: captureNames}, nil
	case BlockKindTest:
		if target == "" {
			return BlockSpec{}, fmt.Errorf("invalid spec block %q", trimmed)
		}
		return BlockSpec{Raw: trimmed, Kind: kind, Target: target, CaptureNames: captureNames}, nil
	}

	return BlockSpec{Raw: trimmed}, nil
}

var captureNamePattern = regexp.MustCompile(`^\$([A-Za-z_][A-Za-z0-9_]*)$`)

func parseCaptureNames(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	names := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		matches := captureNamePattern.FindStringSubmatch(trimmed)
		if matches == nil {
			return nil, fmt.Errorf("invalid capture list %q", strings.TrimSpace(raw))
		}
		name := matches[1]
		if _, ok := seen[name]; ok {
			return nil, fmt.Errorf("duplicate capture %q", name)
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("invalid capture list %q", strings.TrimSpace(raw))
	}
	return names, nil
}
