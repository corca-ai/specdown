package alloy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corca-ai/specdown/internal/specdown/core"
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
	}, []core.CaseSpec{
		{
			ID: core.SpecID{
				File:        "specs/pocket-board.spec.md",
				HeadingPath: []string{"Pocket Board", "Formal Rules"},
				Ordinal:     1,
			},
			Kind:      core.CaseKindAlloy,
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
	}, []core.CaseSpec{
		{
			ID: core.SpecID{
				File:        "specs/pocket-board.spec.md",
				HeadingPath: []string{"Pocket Board", "Formal Rules"},
				Ordinal:     1,
			},
			Kind:      core.CaseKindAlloy,
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

func TestParseReceiptWithMapScopes(t *testing.T) {
	dir := t.TempDir()
	receiptPath := filepath.Join(dir, "receipt.json")
	data := `{
		"commands": {
			"0": {
				"type": "check",
				"source": "check cardShape for 5",
				"scopes": {"Card": "5"},
				"solution": []
			}
		}
	}`
	if err := os.WriteFile(receiptPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	results, err := parseReceipt(receiptPath)
	if err != nil {
		t.Fatalf("parseReceipt with map scopes: %v", err)
	}
	cmd, ok := results["check cardShape for 5"]
	if !ok {
		t.Fatal("expected command entry for 'check cardShape for 5'")
	}
	var m map[string]string
	if err := json.Unmarshal(cmd.Scopes, &m); err != nil {
		t.Fatalf("scopes should be a valid map: %v", err)
	}
	if m["Card"] != "5" {
		t.Fatalf("expected Card scope '5', got %q", m["Card"])
	}
}

func TestParseReceiptWithArrayScopes(t *testing.T) {
	dir := t.TempDir()
	receiptPath := filepath.Join(dir, "receipt.json")
	data := `{
		"commands": {
			"0": {
				"type": "check",
				"source": "check cardShape for 5 but 6 Int",
				"scopes": [{"sig": "univ", "scope": "5"}, {"sig": "Int", "scope": "6"}],
				"solution": []
			}
		}
	}`
	if err := os.WriteFile(receiptPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	results, err := parseReceipt(receiptPath)
	if err != nil {
		t.Fatalf("parseReceipt with array scopes: %v", err)
	}
	cmd, ok := results["check cardShape for 5 but 6 Int"]
	if !ok {
		t.Fatal("expected command entry for 'check cardShape for 5 but 6 Int'")
	}
	var arr []map[string]string
	if err := json.Unmarshal(cmd.Scopes, &arr); err != nil {
		t.Fatalf("scopes should be a valid array: %v", err)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 scope entries, got %d", len(arr))
	}
	if arr[0]["sig"] != "univ" || arr[1]["sig"] != "Int" || arr[1]["scope"] != "6" {
		t.Fatalf("unexpected scope values: %v", arr)
	}
}

func TestReceiptCommandScopesRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
	}{
		{"map", `{"Card":"5"}`},
		{"array", `[{"sig":"univ","scope":"5"},{"sig":"Int","scope":"6"}]`},
		{"null", `null`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var cmd receiptCommand
			raw := `{"type":"check","source":"check x for 5","scopes":` + tc.input + `,"solution":[]}`
			if err := json.Unmarshal([]byte(raw), &cmd); err != nil {
				t.Fatalf("unmarshal %s scopes: %v", tc.name, err)
			}
			compact := func(s string) string {
				var buf json.RawMessage
				if err := json.Unmarshal([]byte(s), &buf); err != nil {
					return s
				}
				out, _ := json.Marshal(buf)
				return string(out)
			}
			if got := compact(string(cmd.Scopes)); got != compact(tc.input) {
				t.Fatalf("round-trip mismatch: got %s, want %s", got, compact(tc.input))
			}
		})
	}
}
