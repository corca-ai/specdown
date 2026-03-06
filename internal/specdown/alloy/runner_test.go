package alloy

import (
	"strings"
	"testing"

	"specdown/internal/specdown/core"
)

func TestBuildBundleSourceCombinesFragmentsAndGeneratedChecks(t *testing.T) {
	source, refs := buildBundleSource("specs/pocket-board.spec.md", core.AlloyModelSpec{
		Name: "board",
		Fragments: []core.AlloyFragmentSpec{
			{
				Model:       "board",
				HeadingPath: []string{"Pocket Board", "형식 규칙"},
				Source:      "module board\n\nsig Card {}",
			},
			{
				Model:       "board",
				HeadingPath: []string{"Pocket Board", "형식 규칙"},
				Source:      "assert cardShape { some Card }",
			},
		},
	}, []core.AlloyCheckSpec{
		{
			ID: core.SpecID{
				File:        "specs/pocket-board.spec.md",
				HeadingPath: []string{"Pocket Board", "형식 규칙"},
				Ordinal:     1,
			},
			Model:     "board",
			Assertion: "cardShape",
			Scope:     "5",
		},
	})

	if !strings.Contains(source, "-- specdown-source: specs/pocket-board.spec.md#Pocket Board/형식 규칙") {
		t.Fatalf("expected source mapping comment, got %q", source)
	}
	if !strings.Contains(source, "module board") || !strings.Contains(source, "assert cardShape { some Card }") {
		t.Fatalf("expected concatenated fragments, got %q", source)
	}
	if !strings.Contains(source, "check cardShape for 5") {
		t.Fatalf("expected generated check command, got %q", source)
	}
	if len(refs) == 0 {
		t.Fatal("expected line refs")
	}
}

func TestAnnotateAlloyFailureMapsBundleLineToSource(t *testing.T) {
	message := annotateAlloyFailure("Syntax error at line 3 column 1", []string{
		"specs/pocket-board.spec.md#Pocket Board/형식 규칙",
		"specs/pocket-board.spec.md#Pocket Board/형식 규칙",
		"specs/pocket-board.spec.md#Pocket Board/형식 규칙",
	})
	if !strings.Contains(message, "source: specs/pocket-board.spec.md#Pocket Board/형식 규칙") {
		t.Fatalf("expected source annotation, got %q", message)
	}
}
