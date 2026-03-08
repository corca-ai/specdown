package core

import (
	"fmt"
	"regexp"
	"strings"
)

type BlockKind string

const (
	BlockKindNone    BlockKind = ""
	BlockKindRun     BlockKind = "run"
	BlockKindVerify  BlockKind = "verify"
	BlockKindTest    BlockKind = "test"
	BlockKindDoctest BlockKind = "doctest"
)

type BlockSpec struct {
	Raw          string    `json:"raw"`
	Kind         BlockKind `json:"kind"`
	Target       string    `json:"target"`
	CaptureNames []string  `json:"captureNames,omitempty"`
	ExpectFail   bool      `json:"expectFail,omitempty"`
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

	if strings.HasPrefix(trimmed, "alloy:") {
		return BlockSpec{}, fmt.Errorf("unsupported spec block %q", trimmed)
	}

	expectFail, working := extractExpectFail(trimmed)
	infoPart, captureNames, err := extractCaptures(working)
	if err != nil {
		return BlockSpec{}, err
	}

	parts := strings.SplitN(infoPart, ":", 2)
	if len(parts) != 2 {
		return BlockSpec{Raw: trimmed}, nil
	}

	kind := BlockKind(parts[0])
	target := strings.TrimSpace(parts[1])
	switch kind {
	case BlockKindRun, BlockKindVerify, BlockKindTest:
		if target == "" {
			return BlockSpec{}, fmt.Errorf("invalid spec block %q", trimmed)
		}
		if expectFail && len(captureNames) > 0 {
			return BlockSpec{}, fmt.Errorf("!fail blocks do not support captures")
		}
		return BlockSpec{Raw: trimmed, Kind: kind, Target: target, CaptureNames: captureNames, ExpectFail: expectFail}, nil
	case BlockKindDoctest:
		if target == "" {
			return BlockSpec{}, fmt.Errorf("invalid spec block %q", trimmed)
		}
		if len(captureNames) > 0 {
			return BlockSpec{}, fmt.Errorf("doctest blocks do not support captures")
		}
		return BlockSpec{Raw: trimmed, Kind: kind, Target: target, ExpectFail: expectFail}, nil
	}

	return BlockSpec{Raw: trimmed}, nil
}

func extractExpectFail(s string) (bool, string) {
	idx := strings.Index(s, " !fail")
	if idx < 0 {
		return false, s
	}
	rest := s[idx+len(" !fail"):]
	if rest != "" && rest[0] != ' ' {
		return false, s
	}
	return true, strings.TrimSpace(s[:idx] + rest)
}

func extractCaptures(working string) (string, []string, error) {
	if !strings.Contains(working, "->") {
		return working, nil, nil
	}
	parts := strings.SplitN(working, "->", 2)
	names, err := parseCaptureNames(parts[1])
	if err != nil {
		return "", nil, err
	}
	return strings.TrimSpace(parts[0]), names, nil
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
