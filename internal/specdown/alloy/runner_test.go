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
				HeadingPath: []string{"Pocket Board", "Formal Rules"},
				Source:      "module board\n\nsig Card {}",
			},
			{
				Model:       "board",
				HeadingPath: []string{"Pocket Board", "Formal Rules"},
				Source:      "assert cardShape { some Card }",
			},
		},
	}, []core.AlloyCheckSpec{
		{
			ID: core.SpecID{
				File:        "specs/pocket-board.spec.md",
				HeadingPath: []string{"Pocket Board", "Formal Rules"},
				Ordinal:     1,
			},
			Model:     "board",
			Assertion: "cardShape",
			Scope:     "5",
		},
	})

	if !strings.Contains(source, "-- specdown-source: specs/pocket-board.spec.md#Pocket Board/Formal Rules") {
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
		"specs/pocket-board.spec.md#Pocket Board/Formal Rules",
		"specs/pocket-board.spec.md#Pocket Board/Formal Rules",
		"specs/pocket-board.spec.md#Pocket Board/Formal Rules",
	}, "Syntax error at line 3 column 1")
	if !ok {
		t.Fatal("expected failure location")
	}
	message := annotateAlloyFailure("Syntax error at line 3 column 1", location, ok)
	if !strings.Contains(message, "bundle line 3") {
		t.Fatalf("expected bundle line, got %q", message)
	}
	if !strings.Contains(message, "source: specs/pocket-board.spec.md#Pocket Board/Formal Rules") {
		t.Fatalf("expected source annotation, got %q", message)
	}
}

func TestClassifyAlloyError(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Syntax error at line 3 column 1", "syntax error in Alloy model"},
		{"Parse error near sig", "syntax error in Alloy model"},
		{"Type error: name cannot be resolved", "type error in Alloy model"},
		{"The name 'Foo' cannot be resolved", "type error in Alloy model"},
		{"java: not found", "java not found in PATH"},
		{"/usr/bin/java: No such file or directory", "java not found in PATH"},
		{"out of memory", "Alloy execution error: out of memory"},
		{"line1\nline2", "Alloy execution error: line1"},
	}
	for _, tt := range tests {
		got := classifyAlloyError(tt.input)
		if got != tt.want {
			t.Errorf("classifyAlloyError(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDumpModelsWritesBundleWithoutRunning(t *testing.T) {
	root := t.TempDir()
	runner := Runner{BaseDir: root}
	plan := core.DocumentPlan{
		Document: core.Document{RelativeTo: "test.spec.md"},
		AlloyModels: []core.AlloyModelSpec{
			{
				Name: "m",
				Fragments: []core.AlloyFragmentSpec{
					{Model: "m", HeadingPath: []string{"T"}, Source: "module m\nsig A {}"},
				},
			},
		},
	}
	paths, err := runner.DumpModels(plan)
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	body, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), "module m") {
		t.Fatalf("expected module m in bundle, got %q", string(body))
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
				HeadingPath: []string{"Pocket Board", "Formal Rules"},
				Source:      "module board\n\nsig Card {}",
			},
		},
	}, []core.AlloyCheckSpec{
		{
			ID: core.SpecID{
				File:        "specs/pocket-board.spec.md",
				HeadingPath: []string{"Pocket Board", "Formal Rules"},
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
	if !strings.Contains(text, `"sourceRef": "specs/pocket-board.spec.md#Pocket Board/Formal Rules"`) {
		t.Fatalf("expected source ref in source map, got %q", text)
	}
	if ext := filepath.Ext(bundle.SourceMapAbsolutePath); ext != ".json" {
		t.Fatalf("unexpected source map extension %q", ext)
	}
}
