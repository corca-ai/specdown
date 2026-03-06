package alloy

import (
	"os"
	"path/filepath"
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
	location, ok := locateAlloyFailure([]string{
		"specs/pocket-board.spec.md#Pocket Board/형식 규칙",
		"specs/pocket-board.spec.md#Pocket Board/형식 규칙",
		"specs/pocket-board.spec.md#Pocket Board/형식 규칙",
	}, "Syntax error at line 3 column 1")
	if !ok {
		t.Fatal("expected failure location")
	}
	message := annotateAlloyFailure("Syntax error at line 3 column 1", location, ok)
	if !strings.Contains(message, "bundle line 3") {
		t.Fatalf("expected bundle line, got %q", message)
	}
	if !strings.Contains(message, "source: specs/pocket-board.spec.md#Pocket Board/형식 규칙") {
		t.Fatalf("expected source annotation, got %q", message)
	}
}

func TestWriteBundleWritesSourceMapArtifact(t *testing.T) {
	root := t.TempDir()
	runner := Runner{BaseDir: root}

	bundle, err := runner.writeBundle("specs/pocket-board.spec.md", core.AlloyModelSpec{
		Name: "board",
		Fragments: []core.AlloyFragmentSpec{
			{
				Model:       "board",
				HeadingPath: []string{"Pocket Board", "형식 규칙"},
				Source:      "module board\n\nsig Card {}",
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
	if err != nil {
		t.Fatalf("write bundle: %v", err)
	}

	if _, err := os.Stat(bundle.AbsolutePath); err != nil {
		t.Fatalf("stat bundle: %v", err)
	}
	if _, err := os.Stat(bundle.SourceMapAbsolutePath); err != nil {
		t.Fatalf("stat source map: %v", err)
	}
	body, err := os.ReadFile(bundle.SourceMapAbsolutePath)
	if err != nil {
		t.Fatalf("read source map: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, `"bundlePath": ".artifacts/specdown/models/specs-pocket-board-spec-md-board.als"`) {
		t.Fatalf("expected bundle path in source map, got %q", text)
	}
	if !strings.Contains(text, `"sourceRef": "specs/pocket-board.spec.md#Pocket Board/형식 규칙"`) {
		t.Fatalf("expected source ref in source map, got %q", text)
	}
	if ext := filepath.Ext(bundle.SourceMapAbsolutePath); ext != ".json" {
		t.Fatalf("unexpected source map extension %q", ext)
	}
}
