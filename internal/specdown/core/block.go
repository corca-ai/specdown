package core

import (
	"fmt"
	"regexp"
	"strings"
)

type BlockKind string

const (
	BlockKindNone BlockKind = ""
	BlockKindRun  BlockKind = "run"
)

type BlockSpec struct {
	Raw          string    `json:"raw"`
	Kind         BlockKind `json:"kind"`
	Target       string    `json:"target"`
	CaptureNames []string  `json:"captureNames,omitempty"`
	ExpectFail   bool      `json:"expectFail,omitempty"`
	Literal      bool      `json:"literal,omitempty"` // !raw: skip variable interpolation
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
		return BlockSpec{}, fmt.Errorf("unsupported spec block %q: alloy blocks use the \"<!-- alloy:model#assertion -->\" directive syntax instead", trimmed)
	}

	literal, working := extractModifier(trimmed, " !raw")
	expectFail, working := extractModifier(working, " !fail")
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
	if kind == BlockKindRun {
		if target == "" {
			return BlockSpec{}, fmt.Errorf("invalid spec block %q: run blocks require a target (e.g. \"run:shell\")", trimmed)
		}
		if expectFail && len(captureNames) > 0 {
			return BlockSpec{}, fmt.Errorf("!fail blocks do not support captures")
		}
		return BlockSpec{Raw: trimmed, Kind: kind, Target: target, CaptureNames: captureNames, ExpectFail: expectFail, Literal: literal}, nil
	}

	return BlockSpec{Raw: trimmed}, nil
}

func extractModifier(s, modifier string) (found bool, rest string) {
	idx := strings.Index(s, modifier)
	if idx < 0 {
		return false, s
	}
	rest = s[idx+len(modifier):]
	if rest != "" && rest[0] != ' ' {
		return false, s
	}
	return true, strings.TrimSpace(s[:idx] + rest)
}

func extractCaptures(working string) (remaining string, names []string, err error) {
	if !strings.Contains(working, "->") {
		return working, nil, nil
	}
	parts := strings.SplitN(working, "->", 2)
	names, err = parseCaptureNames(parts[1])
	if err != nil {
		return "", nil, err
	}
	return strings.TrimSpace(parts[0]), names, nil
}

var captureNamePattern = regexp.MustCompile(`^\$([A-Za-z_][A-Za-z0-9_]*)$`)

// unknownBlockPrefix returns the prefix if the info string has a "prefix:target"
// pattern where the prefix is not a recognized specdown block kind.
func unknownBlockPrefix(info string) string {
	trimmed := strings.TrimSpace(info)
	if trimmed == "" {
		return ""
	}
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 {
		return ""
	}
	prefix := parts[0]
	target := strings.TrimSpace(parts[1])
	if target == "" {
		return ""
	}
	// Skip URIs (e.g. http://example)
	if strings.HasPrefix(target, "//") {
		return ""
	}
	// Only flag alphabetic prefixes (avoid false positives on paths etc.)
	for _, r := range prefix {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return ""
		}
	}
	if BlockKind(prefix) == BlockKindRun {
		return ""
	}
	// alloy: is handled separately in the parser
	if prefix == "alloy" {
		return ""
	}
	return prefix
}

// IsDoctestContent returns true if the first non-empty line of source starts with "$ ".
func IsDoctestContent(source string) bool {
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		return strings.HasPrefix(line, "$ ")
	}
	return false
}

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
