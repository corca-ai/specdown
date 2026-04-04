package html

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/corca-ai/specdown/internal/specdown/config"
	"github.com/corca-ai/specdown/internal/specdown/core"
)

func buildMainTestReport() core.Report {
	return core.Report{
		GeneratedAt: time.Date(2026, 3, 6, 1, 2, 3, 0, time.UTC),
		Summary: core.Summary{
			SpecsTotal:  1,
			SpecsFailed: 1,
			CasesTotal:  4,
			CasesPassed: 3,
			CasesFailed: 1,
		},
		Results: []core.DocumentResult{
			{
				Status: core.StatusFailed,
				Document: core.Document{
					Title:      "Pocket Board",
					RelativeTo: "specs/pocket-board.spec.md",
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "Pocket Board", Raw: "# Pocket Board\n", HeadingPath: []string{"Pocket Board"}},
						core.ProseNode{Raw: "\nDescription paragraph.\n\n"},
						core.CodeBlockNode{
							Block:  core.BlockSpec{Raw: "run:board -> $boardName", Kind: core.BlockKindRun, Target: "board", CaptureNames: []string{"boardName"}},
							Source: "create-board",
							Raw:    "```run:board -> $boardName\ncreate-board\n```\n",
							ID: &core.SpecID{
								File:        "specs/pocket-board.spec.md",
								HeadingPath: []string{"Pocket Board", "Variable Flow"},
								Ordinal:     1,
							},
						},
						core.CodeBlockNode{
							Block:  core.BlockSpec{Raw: "run:board", Kind: core.BlockKindRun, Target: "board"},
							Source: "board \"${boardName}\" should exist",
							Raw:    "```run:board\nboard \"${boardName}\" should exist\n```\n",
							ID: &core.SpecID{
								File:        "specs/pocket-board.spec.md",
								HeadingPath: []string{"Pocket Board", "Variable Flow", "Verify Created Board"},
								Ordinal:     2,
							},
						},
						core.TableNode{
							Check:   "board-exists",
							Columns: []string{"board", "exists"},
							Rows: []core.TableRowNode{
								{
									Cells: []string{"${boardName}", "yes"},
									Raw:   "| ${boardName} | yes |\n",
									ID: &core.SpecID{
										File:        "specs/pocket-board.spec.md",
										HeadingPath: []string{"Pocket Board", "Variable Flow", "Table Check"},
										Ordinal:     3,
									},
								},
								{
									Cells: []string{"${boardName}-archive", "yes"},
									Raw:   "| ${boardName}-archive | yes |\n",
									ID: &core.SpecID{
										File:        "specs/pocket-board.spec.md",
										HeadingPath: []string{"Pocket Board", "Variable Flow", "Table Check"},
										Ordinal:     4,
									},
								},
							},
							Raw: "| board | exists |\n| --- | --- |\n| ${boardName} | yes |\n| ${boardName}-archive | yes |\n",
						},
					},
				},
				Cases: []core.CaseResult{
					{
						ID: core.SpecID{
							File:        "specs/pocket-board.spec.md",
							HeadingPath: []string{"Pocket Board", "Variable Flow"},
							Ordinal:     1,
						},
						Kind:   core.CaseKindCode,
						Label:  "run:board @ Variable Flow",
						Status: core.StatusPassed,
						Code: &core.CodeResultDetail{
							Block:          "run:board",
							Template:       "create-board",
							RenderedSource: "create-board",
						},
						Bindings: []core.Binding{{
							Name:  "boardName",
							Value: "board-1",
						}},
					},
					{
						ID: core.SpecID{
							File:        "specs/pocket-board.spec.md",
							HeadingPath: []string{"Pocket Board", "Variable Flow", "Verify Created Board"},
							Ordinal:     2,
						},
						Kind:   core.CaseKindCode,
						Label:  "run:board @ Verify Created Board",
						Status: core.StatusPassed,
						Code: &core.CodeResultDetail{
							Block:          "run:board",
							Template:       "board \"${boardName}\" should exist",
							RenderedSource: "board \"board-1\" should exist",
						},
					},
					{
						ID: core.SpecID{
							File:        "specs/pocket-board.spec.md",
							HeadingPath: []string{"Pocket Board", "Variable Flow", "Table Check"},
							Ordinal:     3,
						},
						Kind:   core.CaseKindTableRow,
						Label:  "check:board-exists @ Table Check row 1",
						Status: core.StatusPassed,
						Table: &core.TableResultDetail{
							Check:         "board-exists",
							Columns:       []string{"board", "exists"},
							TemplateCells: []string{"${boardName}", "yes"},
							RenderedCells: []string{"board-1", "yes"},
							RowNumber:     1,
						},
					},
					{
						ID: core.SpecID{
							File:        "specs/pocket-board.spec.md",
							HeadingPath: []string{"Pocket Board", "Variable Flow", "Table Check"},
							Ordinal:     4,
						},
						Kind:     core.CaseKindTableRow,
						Label:    "check:board-exists @ Table Check row 2",
						Status:   core.StatusFailed,
						Message:  "board existence check failed",
						Expected: "board-1-archive exists",
						Actual:   "not found",
						Table: &core.TableResultDetail{
							Check:         "board-exists",
							Columns:       []string{"board", "exists"},
							TemplateCells: []string{"${boardName}-archive", "yes"},
							RenderedCells: []string{"board-1-archive", "yes"},
							RowNumber:     2,
						},
					},
				},
			},
		},
	}
}

// writeAndReadReport writes the report to a temp directory and reads
// the HTML file for the first (entry) document result.
func writeAndReadReport(t *testing.T, report core.Report) string {
	t.Helper()
	outDir := filepath.Join(t.TempDir(), "report")
	if _, err := Write(report, outDir); err != nil {
		t.Fatalf("write report: %v", err)
	}
	// The first result is the entry; its HTML is at the root of outDir.
	if len(report.Results) == 0 {
		t.Fatal("no results in report")
	}
	entryRel := report.Results[0].Document.RelativeTo
	entryDir := filepath.Dir(entryRel)
	htmlName := docToHTMLPath(entryRel, entryDir)
	body, err := os.ReadFile(filepath.Join(outDir, htmlName))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	return string(body)
}

func assertContains(t *testing.T, html, substr, label string) {
	t.Helper()
	if !strings.Contains(html, substr) {
		t.Fatalf("expected %s, got %q", label, html)
	}
}

func assertNotContains(t *testing.T, html, substr, label string) {
	t.Helper()
	if strings.Contains(html, substr) {
		t.Fatalf("expected %s, got %q", label, html)
	}
}

func TestWriteRendersMarkdownIntoHTML(t *testing.T) {
	report := buildMainTestReport()
	report.Title = "My Report"
	html := writeAndReadReport(t, report)

	t.Run("layout", func(t *testing.T) {
		assertContains(t, html, "<h1>Pocket Board</h1>", "h1 from markdown")
		assertContains(t, html, "id=\"section-specs-pocket-board-spec-md-pocket-board\"", "section anchor for heading")
		assertNotContains(t, html, "report-title", "no artificial report-title h1")
		assertContains(t, html, "aria-label=\"Table of contents\"", "toc sidebar")
		assertContains(t, html, "viewport-fit=cover", "safe-area viewport mode")
		assertContains(t, html, "style.css", "linked stylesheet")
		assertContains(t, html, "script.js", "linked script")
		assertNotContains(t, html, "<h2>report</h2>", "no report heading")
		assertNotContains(t, html, ">Failures<", "no failure summary section")
	})

	t.Run("toc", func(t *testing.T) {
		// Script is now external; check it's linked.
		assertContains(t, html, "script.js", "linked script for toc")
	})

	t.Run("summary_and_results", func(t *testing.T) {
		assertContains(t, html, "3 passed", "pass summary")
		assertContains(t, html, "1 failed", "fail summary")
		assertContains(t, html, "boardName=board-1", "binding note")
		assertContains(t, html, "check:board-exists", "check label")
		assertContains(t, html, "board-1-archive", "resolved table cell")
		assertContains(t, html, `<dt>expected</dt><dd>board-1-archive exists</dd>`, "expected value in failure diff")
		assertContains(t, html, `<dt>actual</dt><dd>not found</dd>`, "actual value in failure diff")
		assertContains(t, html, "id=\"case-specs-pocket-board-spec-md-pocket-board-variable-flow-table-check-4\"", "failure anchor link")
	})
}

func TestWriteRendersAlloyReferencesWithoutArtifactMetadata(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "report")

	report := core.Report{
		GeneratedAt: time.Date(2026, 3, 6, 1, 2, 3, 0, time.UTC),
		Summary: core.Summary{
			SpecsTotal:  1,
			SpecsFailed: 1,
			CasesTotal:  1,
			CasesFailed: 1,
		},
		Results: []core.DocumentResult{
			{
				Status: core.StatusFailed,
				Document: core.Document{
					Title:      "Pocket Board",
					RelativeTo: "specs/pocket-board.spec.md",
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "Pocket Board", Raw: "# Pocket Board\n", HeadingPath: []string{"Pocket Board"}},
						core.HeadingNode{Level: 2, Text: "Formal Rules", Raw: "## Formal Rules\n", HeadingPath: []string{"Pocket Board", "Formal Rules"}},
						core.AlloyModelNode{
							Model:  "board",
							Source: "module board\n\nsig Card {}",
							Raw:    "```alloy:model(board)\nmodule board\n\nsig Card {}\n```\n",
						},
						core.AlloyRefNode{
							Model:     "board",
							Assertion: "cardShape",
							Scope:     "5",
							Raw:       "> alloy:ref(board#cardShape, scope=5)\n",
							ID: &core.SpecID{
								File:        "specs/pocket-board.spec.md",
								HeadingPath: []string{"Pocket Board", "Formal Rules"},
								Ordinal:     1,
							},
						},
					},
				},
				Cases: []core.CaseResult{
					{
						ID: core.SpecID{
							File:        "specs/pocket-board.spec.md",
							HeadingPath: []string{"Pocket Board", "Formal Rules"},
							Ordinal:     1,
						},
						Kind:    core.CaseKindAlloy,
						Label:   "alloy:ref(board#cardShape, scope=5) @ Formal Rules",
						Status:  core.StatusFailed,
						Message: "found counterexample for assertion \"cardShape\" at scope 5",
						Alloy: &core.AlloyResultDetail{
							Model:      "board",
							Assertion:  "cardShape",
							Scope:      "5",
							SourceRef:  "specs/pocket-board.spec.md#Pocket Board/Formal Rules",
							BundleLine: 7,
						},
					},
				},
			},
		},
	}

	if _, err := Write(report, outDir); err != nil {
		t.Fatalf("write report: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(outDir, "pocket-board.html"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	html := string(body)
	if !strings.Contains(html, "0 passed") || !strings.Contains(html, "1 failed") {
		t.Fatalf("expected compact summary, got %q", html)
	}
	if !strings.Contains(html, "alloy:model(board)") {
		t.Fatalf("expected alloy model label, got %q", html)
	}
	if !strings.Contains(html, "found counterexample for assertion") {
		t.Fatalf("expected alloy failure inline in model block, got %q", html)
	}
	if strings.Contains(html, "bundle artifact") || strings.Contains(html, "source map") {
		t.Fatalf("expected no artifact metadata, got %q", html)
	}
	if strings.Contains(html, "source ref") || strings.Contains(html, "bundle line") {
		t.Fatalf("expected no source metadata, got %q", html)
	}
	if !strings.Contains(html, "class=\"toc-link toc-level-2 failed\"") {
		t.Fatalf("expected failed alloy heading in toc, got %q", html)
	}
}

func TestWriteRendersAlloyCounterexampleDetails(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "report")

	report := core.Report{
		GeneratedAt: time.Date(2026, 3, 6, 1, 2, 3, 0, time.UTC),
		Summary: core.Summary{
			SpecsTotal:  1,
			SpecsFailed: 1,
			CasesTotal:  1,
			CasesFailed: 1,
		},
		Results: []core.DocumentResult{
			{
				Status: core.StatusFailed,
				Document: core.Document{
					Title:      "Pocket Board",
					RelativeTo: "specs/pocket-board.spec.md",
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "Pocket Board", Raw: "# Pocket Board\n", HeadingPath: []string{"Pocket Board"}},
						core.AlloyModelNode{
							Model:  "board",
							Source: "module board\n\nsig Card {}",
							Raw:    "```alloy:model(board)\nmodule board\n\nsig Card {}\n```\n",
						},
					},
				},
				Cases: []core.CaseResult{
					{
						ID: core.SpecID{
							File:        "specs/pocket-board.spec.md",
							HeadingPath: []string{"Pocket Board"},
							Ordinal:     1,
						},
						Kind:    core.CaseKindAlloy,
						Label:   "alloy:ref(board#cardShape, scope=5) @ Pocket Board",
						Status:  core.StatusFailed,
						Message: "found counterexample for assertion \"cardShape\" at scope 5\n\nCounterexample:\n  Card$0 = {Card$0}\n  Card$1 = {Card$1}",
						Alloy: &core.AlloyResultDetail{
							Model:     "board",
							Assertion: "cardShape",
							Scope:     "5",
						},
					},
				},
			},
		},
	}

	if _, err := Write(report, outDir); err != nil {
		t.Fatalf("write report: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(outDir, "pocket-board.html"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	html := string(body)
	if !strings.Contains(html, "found counterexample for assertion") {
		t.Fatalf("expected counterexample summary in report, got %q", html)
	}
	if !strings.Contains(html, "Card$0") {
		t.Fatalf("expected counterexample detail (Card$0) in report, got %q", html)
	}
	if !strings.Contains(html, "Card$1") {
		t.Fatalf("expected counterexample detail (Card$1) in report, got %q", html)
	}
}

func TestWriteUnescapesNewlinesInTableCells(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "report")

	report := core.Report{
		GeneratedAt: time.Date(2026, 3, 6, 1, 2, 3, 0, time.UTC),
		Summary:     core.Summary{SpecsTotal: 1, SpecsPassed: 1, CasesTotal: 1, CasesPassed: 1},
		Results: []core.DocumentResult{
			{
				Status: core.StatusPassed,
				Document: core.Document{
					Title:      "Editor",
					RelativeTo: "specs/editor.spec.md",
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "Editor", Raw: "# Editor\n", HeadingPath: []string{"Editor"}},
						core.TableNode{
							Check:   "editor-op",
							Columns: []string{"initial", "expected"},
							Rows: []core.TableRowNode{
								{
									Cells: []string{`alpha\n\nbeta`, `alpha\n\nbeta`},
									Raw:   `| alpha\n\nbeta | alpha\n\nbeta |` + "\n",
									ID:    &core.SpecID{File: "specs/editor.spec.md", HeadingPath: []string{"Editor"}, Ordinal: 1},
								},
							},
							Raw: "| initial | expected |\n| --- | --- |\n| alpha\\n\\nbeta | alpha\\n\\nbeta |\n",
						},
					},
				},
				Cases: []core.CaseResult{
					{
						ID:     core.SpecID{File: "specs/editor.spec.md", HeadingPath: []string{"Editor"}, Ordinal: 1},
						Kind:   core.CaseKindTableRow,
						Label:  "check:editor-op @ Editor row 1",
						Status: core.StatusPassed,
						Table: &core.TableResultDetail{
							Check:         "editor-op",
							Columns:       []string{"initial", "expected"},
							TemplateCells: []string{`alpha\n\nbeta`, `alpha\n\nbeta`},
							RenderedCells: []string{"alpha\n\nbeta", "alpha\n\nbeta"},
							RowNumber:     1,
						},
					},
				},
			},
		},
	}

	if _, err := Write(report, outDir); err != nil {
		t.Fatalf("write report: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(outDir, "editor.html"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	html := string(body)
	// \n should be unescaped to real newlines, not rendered as literal \n
	if strings.Contains(html, `<div class="cell-template">alpha\n\nbeta</div>`) {
		t.Fatal("expected \\n to be unescaped in table cells, but found literal \\n")
	}
	if !strings.Contains(html, "<div class=\"cell-template\">alpha\n\nbeta</div>") {
		t.Fatal("expected real newlines in table cell output")
	}
}

func TestRenderTableRowDoesNotDoubleUnescape(t *testing.T) {
	// When RenderedCells already contain unescaped values (as produced by the
	// engine's prepareTableRowCase), the renderer must NOT unescape again.
	// A literal backslash-n in RenderedCells means the user wrote \\n in their
	// spec (which the engine correctly unescaped once to \n). The renderer
	// should preserve it, not turn it into an actual newline.
	result := core.CaseResult{
		ID:     core.SpecID{File: "test.spec.md", HeadingPath: []string{"Root"}, Ordinal: 1},
		Kind:   core.CaseKindTableRow,
		Status: core.StatusPassed,
		Label:  "check:test @ Root row 1",
		Table: &core.TableResultDetail{
			Check:         "test",
			Columns:       []string{"input"},
			TemplateCells: []string{`\\n`},
			RenderedCells: []string{`\n`}, // engine already unescaped \\n → \n
			RowNumber:     1,
		},
	}
	row := core.TableRowNode{
		Cells: []string{`\\n`},
		Raw:   `| \\n |` + "\n",
		ID:    &result.ID,
	}

	var out htmlBuilder
	renderTableRow(&out, row, result)
	html := out.String()

	// The renderer should preserve the literal \n, not convert it to a newline.
	if strings.Contains(html, "<div class=\"cell-template\">\n</div>") {
		t.Fatal("renderer double-unescaped: literal \\n was turned into a newline")
	}
	if !strings.Contains(html, `<div class="cell-template">\n</div>`) {
		t.Fatalf("expected literal \\n in output, got: %s", html)
	}
}

func TestCollectHeadingStatusesPropagatesFailureToParent(t *testing.T) {
	result := core.DocumentResult{
		Document: core.Document{
			RelativeTo: "test.spec.md",
			Nodes: []core.Node{
				core.HeadingNode{Level: 1, Text: "Root", HeadingPath: []string{"Root"}},
				core.HeadingNode{Level: 2, Text: "Parent", HeadingPath: []string{"Root", "Parent"}},
				core.HeadingNode{Level: 3, Text: "Child", HeadingPath: []string{"Root", "Parent", "Child"}},
			},
		},
		Cases: []core.CaseResult{
			{
				ID:     core.SpecID{HeadingPath: []string{"Root", "Parent", "Child"}},
				Status: core.StatusFailed,
			},
		},
	}
	statuses := collectHeadingStatuses(result)

	// Child heading should be failed
	if statuses[headingPathKey([]string{"Root", "Parent", "Child"})] != "failed" {
		t.Fatal("child heading should be failed")
	}
	// Parent should also be failed (propagated)
	if statuses[headingPathKey([]string{"Root", "Parent"})] != "failed" {
		t.Fatal("parent heading should be failed via propagation")
	}
	// Root should also be failed
	if statuses[headingPathKey([]string{"Root"})] != "failed" {
		t.Fatal("root heading should be failed via propagation")
	}
}

func TestCollectHeadingStatusesPassedDoesNotOverwriteFailed(t *testing.T) {
	result := core.DocumentResult{
		Cases: []core.CaseResult{
			{ID: core.SpecID{HeadingPath: []string{"A", "B"}}, Status: core.StatusFailed},
			{ID: core.SpecID{HeadingPath: []string{"A", "C"}}, Status: core.StatusPassed},
		},
	}
	statuses := collectHeadingStatuses(result)
	if statuses[headingPathKey([]string{"A"})] != "failed" {
		t.Fatal("parent should stay failed even after a passed sibling")
	}
}

func TestWriteLeavesExecutableBlocksReadableWhenNoCaseResultExists(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "report")

	report := core.Report{
		GeneratedAt: time.Date(2026, 3, 6, 1, 2, 3, 0, time.UTC),
		Summary: core.Summary{
			SpecsTotal:  1,
			SpecsPassed: 1,
			CasesTotal:  1,
			CasesPassed: 1,
		},
		Results: []core.DocumentResult{
			{
				Status: core.StatusPassed,
				Document: core.Document{
					Title:      "Pocket Board",
					RelativeTo: "specs/pocket-board.spec.md",
					Nodes: []core.Node{
						core.CodeBlockNode{
							Block:  core.BlockSpec{Raw: "run:board -> $boardName", Kind: core.BlockKindRun, Target: "board", CaptureNames: []string{"boardName"}},
							Source: "create-board",
							Raw:    "```run:board -> $boardName\ncreate-board\n```\n",
							ID: &core.SpecID{
								File:        "specs/pocket-board.spec.md",
								HeadingPath: []string{"Pocket Board", "Board Creation"},
								Ordinal:     1,
							},
						},
						core.AlloyRefNode{
							Model:     "board",
							Assertion: "cardShape",
							Scope:     "5",
							Raw:       "> alloy:ref(board#cardShape, scope=5)\n",
							ID: &core.SpecID{
								File:        "specs/pocket-board.spec.md",
								HeadingPath: []string{"Pocket Board", "Formal Rules"},
								Ordinal:     2,
							},
						},
					},
				},
				Cases: []core.CaseResult{
					{
						ID: core.SpecID{
							File:        "specs/pocket-board.spec.md",
							HeadingPath: []string{"Pocket Board", "Formal Rules"},
							Ordinal:     2,
						},
						Kind:   core.CaseKindAlloy,
						Label:  "alloy:ref(board#cardShape, scope=5) @ Formal Rules",
						Status: core.StatusPassed,
						Alloy: &core.AlloyResultDetail{
							Model:     "board",
							Assertion: "cardShape",
							Scope:     "5",
						},
					},
				},
			},
		},
	}

	if _, err := Write(report, outDir); err != nil {
		t.Fatalf("write report: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(outDir, "pocket-board.html"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !strings.Contains(string(body), "create-board") {
		t.Fatalf("expected raw executable block, got %q", string(body))
	}
}

func TestWriteCreatesSharedAssets(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "report")
	report := buildMainTestReport()
	if _, err := Write(report, outDir); err != nil {
		t.Fatalf("write report: %v", err)
	}

	// Check that style.css and script.js are created.
	if _, err := os.Stat(filepath.Join(outDir, "style.css")); err != nil {
		t.Fatal("expected style.css to exist")
	}
	if _, err := os.Stat(filepath.Join(outDir, "script.js")); err != nil {
		t.Fatal("expected script.js to exist")
	}
}

func TestWriteRewritesMarkdownLinksToHTML(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "report")

	report := core.Report{
		GeneratedAt: time.Date(2026, 3, 6, 1, 2, 3, 0, time.UTC),
		Summary:     core.Summary{SpecsTotal: 1, SpecsPassed: 1},
		Results: []core.DocumentResult{
			{
				Status: core.StatusPassed,
				Document: core.Document{
					Title:      "Index",
					RelativeTo: "specs/index.spec.md",
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "Index", Raw: "# Index\n", HeadingPath: []string{"Index"}},
						core.ProseNode{Raw: "[Board](board.spec.md) and [Guide](guide.md#intro)\n"},
					},
				},
			},
		},
	}

	if _, err := Write(report, outDir); err != nil {
		t.Fatalf("write report: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(outDir, "index.html"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	html := string(body)
	assertContains(t, html, `href="board.html"`, "rewritten .spec.md link")
	assertContains(t, html, `href="guide.html#intro"`, "rewritten .md link with fragment")
	assertNotContains(t, html, `.spec.md`, "no .spec.md links in output")
	assertNotContains(t, html, `guide.md`, "no .md links in output")
}

func TestWriteOverwritesStaleFile(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "report")

	// Create a plain file at the output path (simulates stale artifact).
	if err := os.WriteFile(outDir, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	report := buildMainTestReport()
	if _, err := Write(report, outDir); err != nil {
		t.Fatalf("Write should overwrite stale file: %v", err)
	}

	info, err := os.Stat(outDir)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal("expected outDir to be a directory after Write")
	}
}

func TestWriteTraceContextLinksUseCorrectRelativePaths(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "report")

	report := core.Report{
		GeneratedAt: time.Date(2026, 3, 6, 1, 2, 3, 0, time.UTC),
		Summary:     core.Summary{SpecsTotal: 4, SpecsPassed: 4},
		TraceGraph: &core.TraceGraphData{
			Documents: []core.TraceDocument{
				{Path: "specs/at/add-multiple.spec.md", Type: "at"},
				{Path: "specs/stories/add-todo.md", Type: "story"},
				{Path: "specs/epics/todo-management.md", Type: "epic"},
			},
			Edges: []core.TraceEdge{
				{Source: "specs/stories/add-todo.md", Target: "specs/at/add-multiple.spec.md", EdgeName: "covered_by"},
				{Source: "specs/epics/todo-management.md", Target: "specs/stories/add-todo.md", EdgeName: "decomposes"},
			},
		},
		Results: []core.DocumentResult{
			{
				Status: core.StatusPassed,
				Document: core.Document{
					Title:      "Overview",
					RelativeTo: "specs/overview.spec.md",
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "Overview", Raw: "# Overview\n", HeadingPath: []string{"Overview"}},
					},
				},
			},
			{
				Status: core.StatusPassed,
				Document: core.Document{
					Title:      "Add Multiple",
					RelativeTo: "specs/at/add-multiple.spec.md",
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "Add Multiple", Raw: "# Add Multiple\n", HeadingPath: []string{"Add Multiple"}},
					},
				},
			},
			{
				Status: core.StatusPassed,
				Document: core.Document{
					Title:      "Add Todo",
					RelativeTo: "specs/stories/add-todo.md",
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "Add Todo", Raw: "# Add Todo\n", HeadingPath: []string{"Add Todo"}},
					},
				},
			},
			{
				Status: core.StatusPassed,
				Document: core.Document{
					Title:      "Todo Management",
					RelativeTo: "specs/epics/todo-management.md",
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "Todo Management", Raw: "# Todo Management\n", HeadingPath: []string{"Todo Management"}},
					},
				},
			},
		},
	}

	if _, err := Write(report, outDir); err != nil {
		t.Fatalf("write report: %v", err)
	}

	// Read the page in at/ subdirectory and check trace links use ../ prefix.
	body, err := os.ReadFile(filepath.Join(outDir, "at", "add-multiple.html"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	html := string(body)

	// The trace panel link to stories/add-todo.html should use ../stories/add-todo.html
	assertContains(t, html, `href="../stories/add-todo.html"`, "trace link should use ../ for sibling directory")
	assertNotContains(t, html, `href="stories/add-todo.html"`, "trace link should not omit ../ prefix")
}

func TestWriteAutoGroupsByDirectory(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "report")

	report := core.Report{
		GeneratedAt: time.Date(2026, 3, 6, 1, 2, 3, 0, time.UTC),
		Summary:     core.Summary{SpecsTotal: 3, SpecsPassed: 3},
		Results: []core.DocumentResult{
			{
				Status: core.StatusPassed,
				Document: core.Document{
					Title:      "Overview",
					RelativeTo: "specs/overview.spec.md",
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "Overview", Raw: "# Overview\n", HeadingPath: []string{"Overview"}},
					},
				},
			},
			{
				Status: core.StatusPassed,
				Document: core.Document{
					Title:      "Add Item",
					RelativeTo: "specs/stories/add-item.spec.md",
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "Add Item", Raw: "# Add Item\n", HeadingPath: []string{"Add Item"}},
					},
				},
			},
			{
				Status: core.StatusFailed,
				Document: core.Document{
					Title:      "Delete Item",
					RelativeTo: "specs/stories/delete-item.spec.md",
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "Delete Item", Raw: "# Delete Item\n", HeadingPath: []string{"Delete Item"}},
					},
				},
				Cases: []core.CaseResult{
					{
						ID:     core.SpecID{File: "specs/stories/delete-item.spec.md", HeadingPath: []string{"Delete Item"}},
						Status: core.StatusFailed,
					},
				},
			},
		},
	}

	if _, err := Write(report, outDir); err != nil {
		t.Fatalf("write report: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(outDir, "overview.html"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	html := string(body)

	// Should have a group for the "stories" subdirectory.
	assertContains(t, html, "toc-group", "should have toc group for subdirectory")
	assertContains(t, html, ">Stories</button>", "group name derived from directory")
	// Group should show failed status since delete-item is failed.
	assertContains(t, html, `toc-group-title failed`, "group status should propagate failure")
	// Overview should be ungrouped (at root level).
	assertContains(t, html, `>Overview</span>`, "root doc should be ungrouped")
}

func TestWriteExplicitTOCConfig(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "report")

	report := core.Report{
		GeneratedAt: time.Date(2026, 3, 6, 1, 2, 3, 0, time.UTC),
		Summary:     core.Summary{SpecsTotal: 3, SpecsPassed: 3},
		Results: []core.DocumentResult{
			{
				Status: core.StatusPassed,
				Document: core.Document{
					Title:      "Overview",
					RelativeTo: "specs/overview.spec.md",
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "Overview", Raw: "# Overview\n", HeadingPath: []string{"Overview"}},
					},
				},
			},
			{
				Status: core.StatusPassed,
				Document: core.Document{
					Title:      "Syntax",
					RelativeTo: "specs/syntax.spec.md",
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "Syntax", Raw: "# Syntax\n", HeadingPath: []string{"Syntax"}},
					},
				},
			},
			{
				Status: core.StatusPassed,
				Document: core.Document{
					Title:      "CLI",
					RelativeTo: "specs/cli.spec.md",
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "CLI", Raw: "# CLI\n", HeadingPath: []string{"CLI"}},
					},
				},
			},
		},
	}

	tocCfg := []config.TOCEntry{
		{Group: "Reference", Docs: []string{"specs/syntax.spec.md", "specs/cli.spec.md"}},
		{Doc: "specs/overview.spec.md"},
	}

	if _, err := Write(report, outDir, tocCfg); err != nil {
		t.Fatalf("write report: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(outDir, "overview.html"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	html := string(body)

	assertContains(t, html, ">Reference</button>", "explicit group name")
	assertContains(t, html, "toc-group", "should have toc group")
}

func TestWriteTOCWarnsOnMissingDoc(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "report")

	report := core.Report{
		GeneratedAt: time.Date(2026, 3, 6, 1, 2, 3, 0, time.UTC),
		Summary:     core.Summary{SpecsTotal: 1, SpecsPassed: 1},
		Results: []core.DocumentResult{
			{
				Status: core.StatusPassed,
				Document: core.Document{
					Title:      "Overview",
					RelativeTo: "specs/overview.spec.md",
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "Overview", Raw: "# Overview\n", HeadingPath: []string{"Overview"}},
					},
				},
			},
		},
	}

	tocCfg := []config.TOCEntry{
		{Doc: "specs/overview.spec.md"},
		{Doc: "specs/nonexistent.spec.md"},
		{Group: "Missing", Docs: []string{"specs/also-missing.spec.md"}},
	}

	warnings, err := Write(report, outDir, tocCfg)
	if err != nil {
		t.Fatalf("write report: %v", err)
	}

	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d: %v", len(warnings), warnings)
	}
	assertContains(t, warnings[0], "nonexistent.spec.md", "standalone warning should mention missing path")
	assertContains(t, warnings[1], "also-missing.spec.md", "group warning should mention missing path")
}

func TestWriteAutoGroupEntryDirWithExplicitClaim(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "report")

	report := core.Report{
		GeneratedAt: time.Date(2026, 3, 6, 1, 2, 3, 0, time.UTC),
		Summary:     core.Summary{SpecsTotal: 3, SpecsPassed: 3},
		Results: []core.DocumentResult{
			{
				Status: core.StatusPassed,
				Document: core.Document{
					Title:      "Overview",
					RelativeTo: "specs/overview.spec.md",
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "Overview", Raw: "# Overview\n", HeadingPath: []string{"Overview"}},
					},
				},
			},
			{
				Status: core.StatusPassed,
				Document: core.Document{
					Title:      "Add Item",
					RelativeTo: "specs/stories/add-item.spec.md",
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "Add Item", Raw: "# Add Item\n", HeadingPath: []string{"Add Item"}},
					},
				},
			},
			{
				Status: core.StatusPassed,
				Document: core.Document{
					Title:      "Internals",
					RelativeTo: "specs/internals/arch.spec.md",
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "Internals", Raw: "# Internals\n", HeadingPath: []string{"Internals"}},
					},
				},
			},
		},
	}

	// Explicitly claim the root-level doc; stories and internals should still auto-group correctly.
	tocCfg := []config.TOCEntry{
		{Doc: "specs/overview.spec.md"},
	}

	if _, err := Write(report, outDir, tocCfg); err != nil {
		t.Fatalf("write report: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(outDir, "overview.html"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	html := string(body)

	// Both subdirectories should form separate groups, not standalone entries.
	assertContains(t, html, ">Stories</button>", "stories should be a group")
	assertContains(t, html, ">Internals</button>", "internals should be a group")
}

func TestWriteDocTypeBadgeInTOC(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "report")

	report := core.Report{
		GeneratedAt: time.Date(2026, 3, 6, 1, 2, 3, 0, time.UTC),
		Summary:     core.Summary{SpecsTotal: 1, SpecsPassed: 1},
		Results: []core.DocumentResult{
			{
				Status: core.StatusPassed,
				Document: core.Document{
					Title:      "Overview",
					RelativeTo: "specs/overview.spec.md",
					Frontmatter: core.Frontmatter{Type: "spec"},
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "Overview", Raw: "# Overview\n", HeadingPath: []string{"Overview"}},
					},
				},
			},
		},
	}

	if _, err := Write(report, outDir); err != nil {
		t.Fatalf("write report: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(outDir, "overview.html"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	html := string(body)

	assertContains(t, html, `toc-type-badge`, "type badge in TOC")
	assertContains(t, html, `>spec</span>`, "type value in badge")
}

func TestWriteRendersMermaidBlock(t *testing.T) {
	mermaidSource := "graph LR\n    A[Core] --> B[Adapter]"
	report := core.Report{
		GeneratedAt: time.Date(2026, 3, 6, 1, 2, 3, 0, time.UTC),
		Summary:     core.Summary{SpecsTotal: 1, SpecsPassed: 1},
		Results: []core.DocumentResult{
			{
				Status: core.StatusPassed,
				Document: core.Document{
					Title:      "Diagram Test",
					RelativeTo: "specs/diagram.spec.md",
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "Diagram Test", Raw: "# Diagram Test\n", HeadingPath: []string{"Diagram Test"}},
						// Standard mermaid block — no ID means non-executable.
						core.CodeBlockNode{
							Block:  core.BlockSpec{Raw: "mermaid"},
							Source: mermaidSource,
							Raw:    "```mermaid\n" + mermaidSource + "\n```\n",
						},
					},
				},
			},
		},
	}

	html := writeAndReadReport(t, report)

	t.Run("renders_mermaid_container", func(t *testing.T) {
		assertContains(t, html, `class="mermaid-diagram"`, "mermaid wrapper div")
		assertContains(t, html, `<pre class="mermaid">`, "mermaid pre element")
	})

	t.Run("source_is_html_escaped", func(t *testing.T) {
		// Source with HTML special chars must be escaped, not injected raw.
		xssReport := core.Report{
			GeneratedAt: time.Date(2026, 3, 6, 1, 2, 3, 0, time.UTC),
			Summary:     core.Summary{SpecsTotal: 1, SpecsPassed: 1},
			Results: []core.DocumentResult{
				{
					Status: core.StatusPassed,
					Document: core.Document{
						Title:      "XSS Test",
						RelativeTo: "specs/xss.spec.md",
						Nodes: []core.Node{
							core.HeadingNode{Level: 1, Text: "XSS Test", Raw: "# XSS Test\n", HeadingPath: []string{"XSS Test"}},
							core.CodeBlockNode{
								Block:  core.BlockSpec{Raw: "mermaid"},
								Source: `graph LR\n    A --> B["<script>alert(1)</script>"]`,
								Raw:    "```mermaid\n...\n```\n",
							},
						},
					},
				},
			},
		}
		xssHTML := writeAndReadReport(t, xssReport)
		assertNotContains(t, xssHTML, "<script>alert(1)</script>", "raw script tag must not appear unescaped")
		assertContains(t, xssHTML, "&lt;script&gt;", "script tag must be HTML-escaped")
	})

	t.Run("capitalised_mermaid_does_not_match", func(t *testing.T) {
		// Detection is case-sensitive: "Mermaid" should fall through to normal code rendering.
		capsReport := core.Report{
			GeneratedAt: time.Date(2026, 3, 6, 1, 2, 3, 0, time.UTC),
			Summary:     core.Summary{SpecsTotal: 1, SpecsPassed: 1},
			Results: []core.DocumentResult{
				{
					Status: core.StatusPassed,
					Document: core.Document{
						Title:      "Caps Test",
						RelativeTo: "specs/caps.spec.md",
						Nodes: []core.Node{
							core.HeadingNode{Level: 1, Text: "Caps Test", Raw: "# Caps Test\n", HeadingPath: []string{"Caps Test"}},
							core.CodeBlockNode{
								Block:  core.BlockSpec{Raw: "Mermaid"},
								Source: "graph LR",
								Raw:    "```Mermaid\ngraph LR\n```\n",
							},
						},
					},
				},
			},
		}
		capsHTML := writeAndReadReport(t, capsReport)
		assertNotContains(t, capsHTML, `class="mermaid-diagram"`, "capitalised Mermaid must not match")
	})
}
