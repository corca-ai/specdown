package alloy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corca-ai/specdown/internal/specdown/core"
	"github.com/corca-ai/specdown/internal/specdown/testutil"
)

// --- helpers ---

func alloyCheck(model, assertion, scope string, ordinal int) core.CaseSpec {
	return core.CaseSpec{
		ID: core.SpecID{
			File:        "specs/test.spec.md",
			HeadingPath: []string{"Test"},
			Ordinal:     ordinal,
		},
		Kind: core.CaseKindAlloy,
		Alloy: &core.AlloyCaseSpec{
			Model:     model,
			Assertion: assertion,
			Scope:     scope,
		},
	}
}

func nonAlloyCase() core.CaseSpec {
	return core.CaseSpec{
		ID:   core.SpecID{File: "specs/test.spec.md", HeadingPath: []string{"Test"}, Ordinal: 1},
		Kind: core.CaseKindCode,
		Code: &core.CodeCaseSpec{Template: "echo hello"},
	}
}

// --- buildBundleSource ---

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
			Kind: core.CaseKindAlloy,
			Alloy: &core.AlloyCaseSpec{
				Model:     "board",
				Assertion: "cardShape",
				Scope:     "5",
			},
		},
	})

	testutil.Contains(t, source, "-- specdown-source: specs/pocket-board.spec.md#Pocket Board/Formal Rules")
	testutil.Contains(t, source, "module board")
	testutil.Contains(t, source, "assert cardShape { some Card }")
	testutil.Contains(t, source, "check cardShape for 5")
	testutil.True(t, len(refs) > 0)
}

func TestBuildBundleSourceSkipsDuplicateChecks(t *testing.T) {
	checks := []core.CaseSpec{
		alloyCheck("m", "a1", "5", 1),
		alloyCheck("m", "a1", "5", 2), // same assertion+scope → same check command
	}
	source, _ := buildBundleSource("test.md", core.AlloyModelSpec{
		Name: "m",
		Fragments: []core.AlloyFragmentSpec{
			{Model: "m", HeadingPath: []string{"T"}, Source: "module m"},
		},
	}, checks)
	testutil.Equal(t, strings.Count(source, "check a1 for 5"), 1)
}

func TestBuildBundleSourceDoesNotDuplicateExistingCommand(t *testing.T) {
	source, _ := buildBundleSource("test.md", core.AlloyModelSpec{
		Name: "m",
		Fragments: []core.AlloyFragmentSpec{
			{Model: "m", HeadingPath: []string{"T"}, Source: "module m\ncheck a1 for 5"},
		},
	}, []core.CaseSpec{alloyCheck("m", "a1", "5", 1)})
	// The command already appears in the fragment, so it should not be appended.
	testutil.Equal(t, strings.Count(source, "check a1 for 5"), 1)
	testutil.NotContains(t, source, "-- specdown-generated-checks")
}

// --- annotateAlloyFailure / locateAlloyFailure ---

func TestAnnotateAlloyFailureMapsBundleLineToSource(t *testing.T) {
	location, ok := locateAlloyFailure([]string{
		"specs/pocket-board.spec.md#Pocket Board/Formal Rules",
		"specs/pocket-board.spec.md#Pocket Board/Formal Rules",
		"specs/pocket-board.spec.md#Pocket Board/Formal Rules",
	}, "Syntax error at line 3 column 1")
	testutil.True(t, ok)

	message := annotateAlloyFailure("Syntax error at line 3 column 1", location, ok)
	testutil.Contains(t, message, "bundle line 3")
	testutil.Contains(t, message, "source: specs/pocket-board.spec.md#Pocket Board/Formal Rules")
}

func TestAnnotateAlloyFailureNoLocation(t *testing.T) {
	msg := annotateAlloyFailure("some error", failureLocation{}, false)
	testutil.Equal(t, msg, "some error")
}

func TestLocateAlloyFailureNoLineRef(t *testing.T) {
	_, ok := locateAlloyFailure([]string{"ref"}, "no line number here")
	testutil.False(t, ok)
}

func TestLocateAlloyFailureOutOfRange(t *testing.T) {
	_, ok := locateAlloyFailure([]string{"ref"}, "error at line 99")
	testutil.False(t, ok)
}

func TestLocateAlloyFailureEmptySourceRef(t *testing.T) {
	_, ok := locateAlloyFailure([]string{""}, "error at line 1")
	testutil.False(t, ok)
}

// --- DumpModels ---

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
	testutil.NilErr(t, err)
	testutil.Len(t, paths, 1)

	body, err := os.ReadFile(paths[0])
	testutil.NilErr(t, err)
	testutil.Contains(t, string(body), "module m")
}

func TestDumpModelsReturnsNilWithNoModels(t *testing.T) {
	runner := Runner{BaseDir: t.TempDir()}
	paths, err := runner.DumpModels(core.DocumentPlan{})
	testutil.NilErr(t, err)
	testutil.Len(t, paths, 0)
}

// --- writeBundle / writeSourceMap ---

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
	}, []core.CaseSpec{alloyCheck("board", "cardShape", "5", 1)})
	testutil.NilErr(t, err)

	_, err = os.Stat(bundle.AbsolutePath)
	testutil.NilErr(t, err)
	_, err = os.Stat(bundle.SourceMapAbsolutePath)
	testutil.NilErr(t, err)

	body, err := os.ReadFile(bundle.SourceMapAbsolutePath)
	testutil.NilErr(t, err)
	text := string(body)
	testutil.Contains(t, text, `"bundlePath": ".artifacts/specdown/models/specs-pocket-board-spec-md-board.als"`)
	testutil.Contains(t, text, `"sourceRef": "specs/pocket-board.spec.md#Pocket Board/Formal Rules"`)
	testutil.Equal(t, filepath.Ext(bundle.SourceMapAbsolutePath), ".json")
}

// --- parseReceipt ---

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
	testutil.NilErr(t, os.WriteFile(receiptPath, []byte(data), 0o644))

	results, err := parseReceipt(receiptPath)
	testutil.NilErr(t, err)

	cmd, ok := results["check cardShape for 5"]
	testutil.True(t, ok)

	var m map[string]string
	testutil.NilErr(t, json.Unmarshal(cmd.Scopes, &m))
	testutil.Equal(t, m["Card"], "5")
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
	testutil.NilErr(t, os.WriteFile(receiptPath, []byte(data), 0o644))

	results, err := parseReceipt(receiptPath)
	testutil.NilErr(t, err)

	cmd, ok := results["check cardShape for 5 but 6 Int"]
	testutil.True(t, ok)

	var arr []map[string]string
	testutil.NilErr(t, json.Unmarshal(cmd.Scopes, &arr))
	testutil.Len(t, arr, 2)
	testutil.Equal(t, arr[0]["sig"], "univ")
	testutil.Equal(t, arr[1]["sig"], "Int")
}

func TestParseReceiptMissingFile(t *testing.T) {
	_, err := parseReceipt("/no/such/file.json")
	testutil.WantErr(t, err)
	testutil.Contains(t, err.Error(), "read alloy receipt")
}

func TestParseReceiptInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	receiptPath := filepath.Join(dir, "receipt.json")
	testutil.NilErr(t, os.WriteFile(receiptPath, []byte("{invalid"), 0o644))

	_, err := parseReceipt(receiptPath)
	testutil.WantErr(t, err)
	testutil.Contains(t, err.Error(), "decode alloy receipt")
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
			testutil.NilErr(t, json.Unmarshal([]byte(raw), &cmd))

			compact := func(s string) string {
				var buf json.RawMessage
				if err := json.Unmarshal([]byte(s), &buf); err != nil {
					return s
				}
				out, _ := json.Marshal(buf)
				return string(out)
			}
			testutil.Equal(t, compact(string(cmd.Scopes)), compact(tc.input))
		})
	}
}

// --- filterAlloyCases ---

func TestFilterAlloyCases(t *testing.T) {
	t.Run("filters only alloy cases", func(t *testing.T) {
		cases := []core.CaseSpec{
			alloyCheck("m", "a1", "5", 1),
			nonAlloyCase(),
			alloyCheck("m", "a2", "5", 2),
		}
		result := filterAlloyCases(cases)
		testutil.Len(t, result, 2)
	})

	t.Run("returns nil for no alloy cases", func(t *testing.T) {
		result := filterAlloyCases([]core.CaseSpec{nonAlloyCase()})
		testutil.Len(t, result, 0)
	})

	t.Run("returns nil for empty input", func(t *testing.T) {
		result := filterAlloyCases(nil)
		testutil.Len(t, result, 0)
	})
}

// --- collectOrderedResults ---

func TestCollectOrderedResults(t *testing.T) {
	checks := []core.CaseSpec{
		alloyCheck("m", "a1", "5", 1),
		alloyCheck("m", "a2", "10", 2),
	}
	resultsByKey := map[string]core.CaseResult{
		checks[0].ID.Key(): {ID: checks[0].ID, Status: core.StatusPassed},
		checks[1].ID.Key(): {ID: checks[1].ID, Status: core.StatusFailed},
	}

	results, err := collectOrderedResults(checks, resultsByKey)
	testutil.NilErr(t, err)
	testutil.Len(t, results, 2)
	testutil.Equal(t, results[0].Status, core.StatusPassed)
	testutil.Equal(t, results[1].Status, core.StatusFailed)
}

func TestCollectOrderedResultsMissingKey(t *testing.T) {
	checks := []core.CaseSpec{alloyCheck("m", "a1", "5", 1)}
	_, err := collectOrderedResults(checks, map[string]core.CaseResult{})
	testutil.WantErr(t, err)
	testutil.Contains(t, err.Error(), "missing alloy result")
}

// --- RunDocument ---

func TestRunDocumentNoModels(t *testing.T) {
	runner := Runner{BaseDir: t.TempDir()}
	results, err := runner.RunDocument(core.DocumentPlan{
		Cases: []core.CaseSpec{alloyCheck("m", "a1", "5", 1)},
	})
	testutil.NilErr(t, err)
	testutil.Len(t, results, 0)
}

func TestRunDocumentNoAlloyCases(t *testing.T) {
	runner := Runner{BaseDir: t.TempDir()}
	results, err := runner.RunDocument(core.DocumentPlan{
		AlloyModels: []core.AlloyModelSpec{{Name: "m"}},
		Cases:       []core.CaseSpec{nonAlloyCase()},
	})
	testutil.NilErr(t, err)
	testutil.Len(t, results, 0)
}

func TestRunDocumentNoJava(t *testing.T) {
	// Set PATH to empty so java is not found.
	t.Setenv("PATH", "")
	runner := Runner{BaseDir: t.TempDir()}
	results, err := runner.RunDocument(core.DocumentPlan{
		Document: core.Document{RelativeTo: "test.md"},
		AlloyModels: []core.AlloyModelSpec{
			{Name: "m", Fragments: []core.AlloyFragmentSpec{{Model: "m", HeadingPath: []string{"T"}, Source: "module m"}}},
		},
		Cases: []core.CaseSpec{alloyCheck("m", "a1", "5", 1)},
	})
	testutil.NilErr(t, err)
	testutil.Len(t, results, 1)
	testutil.Equal(t, results[0].Status, core.StatusFailed)
	testutil.Contains(t, results[0].Message, "java not found")
}

// --- failedChecksAll ---

func TestFailedChecksAll(t *testing.T) {
	checks := []core.CaseSpec{
		alloyCheck("m", "a1", "5", 1),
		alloyCheck("m", "a2", "5", 2),
	}
	results := failedChecksAll(checks, "some error")
	testutil.Len(t, results, 2)
	for _, r := range results {
		testutil.Equal(t, r.Status, core.StatusFailed)
		testutil.Equal(t, r.Message, "some error")
		testutil.Equal(t, r.Kind, core.CaseKindAlloy)
	}
}

// --- failedChecks ---

func TestFailedChecksWithLocation(t *testing.T) {
	checks := []core.CaseSpec{alloyCheck("m", "a1", "5", 1)}
	loc := failureLocation{BundleLine: 3, SourceRef: "test.md#Section"}
	results := failedChecks(checks, "/bundle.als", "/bundle.als.map.json", "error msg", loc, true)
	testutil.Len(t, results, 1)
	testutil.Equal(t, results[0].BundleLine, 3)
	testutil.Equal(t, results[0].SourceRef, "test.md#Section")
	testutil.Equal(t, results[0].BundlePath, "/bundle.als")
}

func TestFailedChecksWithoutLocation(t *testing.T) {
	checks := []core.CaseSpec{alloyCheck("m", "a1", "5", 1)}
	results := failedChecks(checks, "/bundle.als", "/map.json", "error msg", failureLocation{}, false)
	testutil.Len(t, results, 1)
	testutil.Equal(t, results[0].BundleLine, 0)
	testutil.Equal(t, results[0].SourceRef, "")
}

// --- evaluateCheck ---

func TestEvaluateCheckPasses(t *testing.T) {
	root := t.TempDir()
	runner := Runner{BaseDir: root}
	check := alloyCheck("m", "a1", "5", 1)
	bundle := modelBundle{Model: "m", AbsolutePath: "/b.als", SourceMapAbsolutePath: "/b.als.map.json"}
	commands := map[string]receiptCommand{
		"check a1 for 5": {Type: "check", Source: "check a1 for 5", Solution: nil},
	}
	result, err := runner.evaluateCheck(check, bundle, commands)
	testutil.NilErr(t, err)
	testutil.Equal(t, result.Status, core.StatusPassed)
}

func TestEvaluateCheckMissingCommand(t *testing.T) {
	runner := Runner{BaseDir: t.TempDir()}
	check := alloyCheck("m", "a1", "5", 1)
	bundle := modelBundle{Model: "m"}
	result, err := runner.evaluateCheck(check, bundle, map[string]receiptCommand{})
	testutil.NilErr(t, err)
	testutil.Equal(t, result.Status, core.StatusFailed)
	testutil.Contains(t, result.Message, "missing Alloy result")
}

func TestEvaluateCheckCounterexample(t *testing.T) {
	root := t.TempDir()
	runner := Runner{BaseDir: root}
	check := alloyCheck("m", "a1", "5", 1)
	bundle := modelBundle{Model: "m", AbsolutePath: "/b.als", SourceMapAbsolutePath: "/b.als.map.json"}
	commands := map[string]receiptCommand{
		"check a1 for 5": {
			Type:   "check",
			Source: "check a1 for 5",
			Solution: []receiptSolution{
				{Instances: []json.RawMessage{[]byte(`{"values":{}}`)}},
			},
		},
	}
	result, err := runner.evaluateCheck(check, bundle, commands)
	testutil.NilErr(t, err)
	testutil.Equal(t, result.Status, core.StatusFailed)
	testutil.Contains(t, result.Message, "counterexample")
	testutil.True(t, result.CounterexamplePath != "")
}

// --- summarizeCounterexample ---

func TestSummarizeCounterexample(t *testing.T) {
	t.Run("no solution", func(t *testing.T) {
		testutil.Equal(t, summarizeCounterexample(receiptCommand{}), "counterexample found")
	})

	t.Run("empty instances", func(t *testing.T) {
		cmd := receiptCommand{Solution: []receiptSolution{{}}}
		testutil.Equal(t, summarizeCounterexample(cmd), "counterexample found")
	})

	t.Run("malformed JSON", func(t *testing.T) {
		cmd := receiptCommand{
			Solution: []receiptSolution{
				{Instances: []json.RawMessage{[]byte("{invalid")}},
			},
		}
		result := summarizeCounterexample(cmd)
		testutil.Contains(t, result, "unable to parse instance")
	})

	t.Run("valid instance", func(t *testing.T) {
		instance := `{"values":{"Atom":{"rel":[["v1","v2"]]}}}`
		cmd := receiptCommand{
			Solution: []receiptSolution{
				{Instances: []json.RawMessage{[]byte(instance)}},
			},
		}
		result := summarizeCounterexample(cmd)
		testutil.Contains(t, result, "Atom.rel")
		testutil.Contains(t, result, "v1, v2")
	})

	t.Run("empty values", func(t *testing.T) {
		cmd := receiptCommand{
			Solution: []receiptSolution{
				{Instances: []json.RawMessage{[]byte(`{"values":{}}`)}},
			},
		}
		testutil.Equal(t, summarizeCounterexample(cmd), "counterexample found")
	})
}

// --- writeCounterexample ---

func TestWriteCounterexample(t *testing.T) {
	root := t.TempDir()
	check := alloyCheck("m", "a1", "5", 1)
	cmd := receiptCommand{Type: "check", Source: "check a1 for 5"}

	outPath, err := writeCounterexample(root, check, cmd)
	testutil.NilErr(t, err)
	testutil.True(t, strings.HasSuffix(outPath, ".json"))

	body, err := os.ReadFile(outPath)
	testutil.NilErr(t, err)
	testutil.Contains(t, string(body), "check a1 for 5")
}

// --- ensureAlloyJar ---

func TestEnsureAlloyJarCacheHit(t *testing.T) {
	cacheDir := t.TempDir()
	jarPath := filepath.Join(cacheDir, alloyJarName)
	testutil.NilErr(t, os.WriteFile(jarPath, []byte("fake-jar"), 0o644))

	t.Setenv("XDG_CACHE_HOME", cacheDir)
	// No parent "specdown" dir here — recreate the expected structure.
	expected := filepath.Join(cacheDir, "specdown", alloyJarName)
	testutil.NilErr(t, os.MkdirAll(filepath.Dir(expected), 0o755))
	testutil.NilErr(t, os.WriteFile(expected, []byte("fake-jar"), 0o644))

	runner := Runner{}
	path, err := runner.ensureAlloyJar()
	testutil.NilErr(t, err)
	testutil.Equal(t, path, expected)
}

func TestEnsureAlloyJarDownloads(t *testing.T) {
	jarContent := "PK-fake-alloy-jar-content"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, jarContent)
	}))
	defer server.Close()

	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	// Override the URL for the test
	origURL := alloyJarURL
	alloyJarURL = server.URL + "/" + alloyJarName
	defer func() { alloyJarURL = origURL }()

	runner := Runner{HTTPClient: server.Client()}
	path, err := runner.ensureAlloyJar()
	testutil.NilErr(t, err)

	body, err := os.ReadFile(path)
	testutil.NilErr(t, err)
	testutil.Equal(t, string(body), jarContent)
}

func TestEnsureAlloyJarDownloadFailsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	origURL := alloyJarURL
	alloyJarURL = server.URL + "/" + alloyJarName
	defer func() { alloyJarURL = origURL }()

	runner := Runner{HTTPClient: server.Client()}
	_, err := runner.ensureAlloyJar()
	testutil.WantErr(t, err)
	testutil.Contains(t, err.Error(), "unexpected status")
}

// --- alloyCacheDir ---

func TestAlloyCacheDirXDG(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/custom/cache")
	dir, err := alloyCacheDir()
	testutil.NilErr(t, err)
	testutil.Equal(t, dir, "/custom/cache/specdown")
}

func TestAlloyCacheDirDefault(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	dir, err := alloyCacheDir()
	testutil.NilErr(t, err)
	testutil.Contains(t, dir, ".cache/specdown")
}

// --- strconvQuote ---

func TestStrconvQuote(t *testing.T) {
	testutil.Equal(t, strconvQuote("hello"), `"hello"`)
	testutil.Equal(t, strconvQuote(`say "hi"`), `"say \"hi\""`)
}

// --- splitBundleLines ---

func TestSplitBundleLines(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		testutil.Len(t, splitBundleLines(""), 1)
		testutil.Equal(t, splitBundleLines("")[0], "")
	})

	t.Run("single line", func(t *testing.T) {
		testutil.Len(t, splitBundleLines("hello"), 1)
	})

	t.Run("crlf", func(t *testing.T) {
		result := splitBundleLines("a\r\nb")
		testutil.Len(t, result, 2)
		testutil.Equal(t, result[0], "a")
	})
}

// --- formatSourceRef ---

func TestFormatSourceRef(t *testing.T) {
	testutil.Equal(t, formatSourceRef("test.md", nil), "test.md")
	testutil.Equal(t, formatSourceRef("test.md", []string{"A", "B"}), "test.md#A/B")
}

// --- bundleContainsCommand ---

func TestBundleContainsCommand(t *testing.T) {
	lines := []string{"module m", "  check a1 for 5  ", "sig A {}"}
	testutil.True(t, bundleContainsCommand(lines, "check a1 for 5"))
	testutil.False(t, bundleContainsCommand(lines, "check a2 for 5"))
}

// --- bundleFileName ---

func TestBundleFileName(t *testing.T) {
	name := bundleFileName("specs/test.spec.md", "board")
	testutil.Equal(t, name, "specs-test-spec-md-board.als")
}

// --- baseCheckResult ---

func TestBaseCheckResult(t *testing.T) {
	check := alloyCheck("m", "a1", "5", 1)
	bundle := modelBundle{Model: "m", AbsolutePath: "/b.als", SourceMapAbsolutePath: "/b.als.map.json"}
	result := baseCheckResult(check, bundle)
	testutil.Equal(t, result.Kind, core.CaseKindAlloy)
	testutil.Equal(t, result.Model, "m")
	testutil.Equal(t, result.Assertion, "a1")
	testutil.Equal(t, result.Scope, "5")
	testutil.Equal(t, result.BundlePath, "/b.als")
}
