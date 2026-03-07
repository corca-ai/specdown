package html

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"specdown/internal/specdown/core"
)

func TestWriteRendersMarkdownIntoHTML(t *testing.T) {
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "report.html")

	report := core.Report{
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
							Block:  core.BlockSpec{Raw: "verify:board", Kind: core.BlockKindVerify, Target: "board"},
							Source: "board \"${boardName}\" should exist",
							Raw:    "```verify:board\nboard \"${boardName}\" should exist\n```\n",
							ID: &core.SpecID{
								File:        "specs/pocket-board.spec.md",
								HeadingPath: []string{"Pocket Board", "Variable Flow", "Verify Created Board"},
								Ordinal:     2,
							},
						},
						core.TableNode{
							Fixture: "board-exists",
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
						Kind:           core.CaseKindCode,
						Block:          "run:board",
						Label:          "run:board @ Variable Flow",
						Template:       "create-board",
						RenderedSource: "create-board",
						Status:         core.StatusPassed,
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
						Kind:           core.CaseKindCode,
						Block:          "verify:board",
						Label:          "verify:board @ Verify Created Board",
						Template:       "board \"${boardName}\" should exist",
						RenderedSource: "board \"board-1\" should exist",
						Status:         core.StatusPassed,
					},
					{
						ID: core.SpecID{
							File:        "specs/pocket-board.spec.md",
							HeadingPath: []string{"Pocket Board", "Variable Flow", "Table Check"},
							Ordinal:     3,
						},
						Kind:          core.CaseKindTableRow,
						Fixture:       "board-exists",
						Label:         "fixture:board-exists @ Table Check row 1",
						Columns:       []string{"board", "exists"},
						TemplateCells: []string{"${boardName}", "yes"},
						RenderedCells: []string{"board-1", "yes"},
						RowNumber:     1,
						Status:        core.StatusPassed,
					},
					{
						ID: core.SpecID{
							File:        "specs/pocket-board.spec.md",
							HeadingPath: []string{"Pocket Board", "Variable Flow", "Table Check"},
							Ordinal:     4,
						},
						Kind:          core.CaseKindTableRow,
						Fixture:       "board-exists",
						Label:         "fixture:board-exists @ Table Check row 2",
						Columns:       []string{"board", "exists"},
						TemplateCells: []string{"${boardName}-archive", "yes"},
						RenderedCells: []string{"board-1-archive", "yes"},
						RowNumber:     2,
						Status:        core.StatusFailed,
						Message:       "expected board \"board-1-archive\" to exist; actual boards: [\"board-1\"]",
					},
				},
			},
		},
	}

	if err := Write(report, "My Report", outPath); err != nil {
		t.Fatalf("write report: %v", err)
	}

	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	html := string(body)
	if !strings.Contains(html, "<h1 class=\"report-title\">My Report</h1>") {
		t.Fatalf("expected h1 report title, got %q", html)
	}
	if !strings.Contains(html, "<h2 id=\"section-specs-pocket-board-spec-md-pocket-board\">Pocket Board</h2>") {
		t.Fatalf("expected markdown heading in html, got %q", html)
	}
	if !strings.Contains(html, "aria-label=\"Table of contents\"") {
		t.Fatalf("expected toc sidebar, got %q", html)
	}
	if !strings.Contains(html, "position: sticky;") {
		t.Fatalf("expected sticky toc styles, got %q", html)
	}
	if strings.Contains(html, "<h2>report</h2>") {
		t.Fatalf("expected no report heading, got %q", html)
	}
	if strings.Contains(html, ">Failures<") {
		t.Fatalf("expected no failure summary section, got %q", html)
	}
	if strings.Contains(html, "border-left: 1px solid var(--rule);") {
		t.Fatalf("expected no left rule in toc, got %q", html)
	}
	if !strings.Contains(html, "font-family: \"Avenir Next\", \"Helvetica Neue\", \"Segoe UI\", sans-serif;") {
		t.Fatalf("expected sans body typography, got %q", html)
	}
	if !strings.Contains(html, "text-wrap: balance;") {
		t.Fatalf("expected balanced heading wrap, got %q", html)
	}
	if !strings.Contains(html, "& h2 {") {
		t.Fatalf("expected heading typography rules, got %q", html)
	}
	if !strings.Contains(html, "align-items: baseline;") {
		t.Fatalf("expected baseline-aligned failure diff, got %q", html)
	}
	if !strings.Contains(html, "toc-spec-title") {
		t.Fatalf("expected spec title in toc, got %q", html)
	}
	if !strings.Contains(html, `<a class="toc-spec-title`) {
		t.Fatalf("expected spec title to be a link, got %q", html)
	}
	if !strings.Contains(html, "classList.toggle('active'") {
		t.Fatalf("expected active toc script, got %q", html)
	}
	if strings.Contains(html, "class=\"toc-link toc-level-1 failed\"") {
		t.Fatalf("expected no propagated status on parent heading, got %q", html)
	}
	if strings.Contains(html, "class=\"toc-link toc-level-1 passed\"") || strings.Contains(html, "class=\"toc-link toc-level-2 passed\"") {
		t.Fatalf("expected no success marker in toc, got %q", html)
	}
	if !strings.Contains(html, "pass 3") {
		t.Fatalf("expected pass summary, got %q", html)
	}
	if !strings.Contains(html, "fail 1") {
		t.Fatalf("expected fail summary, got %q", html)
	}
	if !strings.Contains(html, "boardName=board-1") {
		t.Fatalf("expected binding note, got %q", html)
	}
	if !strings.Contains(html, "fixture:board-exists") {
		t.Fatalf("expected fixture label, got %q", html)
	}
	if !strings.Contains(html, "board-1-archive") {
		t.Fatalf("expected resolved table cell, got %q", html)
	}
	if !strings.Contains(html, `<div class="cell-actual">expected board &#34;board-1-archive&#34; to exist; actual boards: [&#34;board-1&#34;]</div>`) {
		t.Fatalf("expected cell-actual with failure message, got %q", html)
	}
	if !strings.Contains(html, "id=\"case-specs-pocket-board-spec-md-pocket-board-variable-flow-table-check-4\"") {
		t.Fatalf("expected failure anchor link, got %q", html)
	}
}

func TestWriteRendersAlloyReferencesWithoutArtifactMetadata(t *testing.T) {
	outDir := t.TempDir()
	reportPath := filepath.Join(outDir, "report.html")

	report := core.Report{
		GeneratedAt: time.Date(2026, 3, 6, 1, 2, 3, 0, time.UTC),
		Summary: core.Summary{
			SpecsTotal:        1,
			SpecsFailed:       1,
			AlloyChecksTotal:  1,
			AlloyChecksFailed: 1,
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
							Raw:       "<!-- alloy:ref(board#cardShape, scope=5) -->\n",
							ID: &core.SpecID{
								File:        "specs/pocket-board.spec.md",
								HeadingPath: []string{"Pocket Board", "Formal Rules"},
								Ordinal:     1,
							},
						},
					},
				},
				AlloyChecks: []core.AlloyCheckResult{
					{
						ID: core.SpecID{
							File:        "specs/pocket-board.spec.md",
							HeadingPath: []string{"Pocket Board", "Formal Rules"},
							Ordinal:     1,
						},
						Model:      "board",
						Assertion:  "cardShape",
						Scope:      "5",
						Label:      "alloy:ref(board#cardShape, scope=5) @ Formal Rules",
						Status:     core.StatusFailed,
						Message:    "found counterexample for assertion \"cardShape\" at scope 5",
						Expected:   "assertion \"cardShape\" holds for scope 5",
						Actual:     "counterexample found",
						SourceRef:  "specs/pocket-board.spec.md#Pocket Board/Formal Rules",
						BundleLine: 7,
					},
				},
			},
		},
	}

	if err := Write(report, "", reportPath); err != nil {
		t.Fatalf("write report: %v", err)
	}

	body, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	html := string(body)
	if !strings.Contains(html, "pass 0") || !strings.Contains(html, "fail 1") {
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
	if statuses[headingPathKey([]string{"Root", "Parent", "Child"})] != core.StatusFailed {
		t.Fatal("child heading should be failed")
	}
	// Parent should also be failed (propagated)
	if statuses[headingPathKey([]string{"Root", "Parent"})] != core.StatusFailed {
		t.Fatal("parent heading should be failed via propagation")
	}
	// Root should also be failed
	if statuses[headingPathKey([]string{"Root"})] != core.StatusFailed {
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
	if statuses[headingPathKey([]string{"A"})] != core.StatusFailed {
		t.Fatal("parent should stay failed even after a passed sibling")
	}
}

func TestWriteLeavesExecutableBlocksReadableWhenNoCaseResultExists(t *testing.T) {
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "report.html")

	report := core.Report{
		GeneratedAt: time.Date(2026, 3, 6, 1, 2, 3, 0, time.UTC),
		Summary: core.Summary{
			SpecsTotal:        1,
			SpecsPassed:       1,
			AlloyChecksTotal:  1,
			AlloyChecksPassed: 1,
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
							Raw:       "<!-- alloy:ref(board#cardShape, scope=5) -->\n",
							ID: &core.SpecID{
								File:        "specs/pocket-board.spec.md",
								HeadingPath: []string{"Pocket Board", "Formal Rules"},
								Ordinal:     2,
							},
						},
					},
				},
				AlloyChecks: []core.AlloyCheckResult{
					{
						ID: core.SpecID{
							File:        "specs/pocket-board.spec.md",
							HeadingPath: []string{"Pocket Board", "Formal Rules"},
							Ordinal:     2,
						},
						Model:     "board",
						Assertion: "cardShape",
						Scope:     "5",
						Label:     "alloy:ref(board#cardShape, scope=5) @ Formal Rules",
						Status:    core.StatusPassed,
					},
				},
			},
		},
	}

	if err := Write(report, "", outPath); err != nil {
		t.Fatalf("write report: %v", err)
	}

	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !strings.Contains(string(body), "create-board") {
		t.Fatalf("expected raw executable block, got %q", string(body))
	}
}
