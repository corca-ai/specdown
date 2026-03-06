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
						core.HeadingNode{Level: 1, Text: "Pocket Board", Raw: "# Pocket Board\n"},
						core.ProseNode{Raw: "\n설명 문단.\n\n"},
						core.CodeBlockNode{
							Block:  core.BlockSpec{Raw: "run:board -> $boardName", Kind: core.BlockKindRun, Target: "board", CaptureNames: []string{"boardName"}},
							Source: "create-board",
							Raw:    "```run:board -> $boardName\ncreate-board\n```\n",
							ID: &core.SpecID{
								File:        "specs/pocket-board.spec.md",
								HeadingPath: []string{"Pocket Board", "변수 흐름"},
								Ordinal:     1,
							},
						},
						core.CodeBlockNode{
							Block:  core.BlockSpec{Raw: "verify:board", Kind: core.BlockKindVerify, Target: "board"},
							Source: "board \"${boardName}\" should exist",
							Raw:    "```verify:board\nboard \"${boardName}\" should exist\n```\n",
							ID: &core.SpecID{
								File:        "specs/pocket-board.spec.md",
								HeadingPath: []string{"Pocket Board", "변수 흐름", "생성한 보드 확인"},
								Ordinal:     2,
							},
						},
						core.TableNode{
							Fixture: "board-exists",
							Columns: []string{"board", "exists"},
							Rows: []core.TableRowNode{
								{
									Cells: []string{"${boardName}", "예"},
									Raw:   "| ${boardName} | 예 |\n",
									ID: &core.SpecID{
										File:        "specs/pocket-board.spec.md",
										HeadingPath: []string{"Pocket Board", "변수 흐름", "표 기반 확인"},
										Ordinal:     3,
									},
								},
								{
									Cells: []string{"${boardName}-archive", "예"},
									Raw:   "| ${boardName}-archive | 예 |\n",
									ID: &core.SpecID{
										File:        "specs/pocket-board.spec.md",
										HeadingPath: []string{"Pocket Board", "변수 흐름", "표 기반 확인"},
										Ordinal:     4,
									},
								},
							},
							Raw: "| board | exists |\n| --- | --- |\n| ${boardName} | 예 |\n| ${boardName}-archive | 예 |\n",
						},
					},
				},
				Cases: []core.CaseResult{
					{
						ID: core.SpecID{
							File:        "specs/pocket-board.spec.md",
							HeadingPath: []string{"Pocket Board", "변수 흐름"},
							Ordinal:     1,
						},
						Kind:           core.CaseKindCode,
						Block:          "run:board",
						Label:          "run:board @ 변수 흐름",
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
							HeadingPath: []string{"Pocket Board", "변수 흐름", "생성한 보드 확인"},
							Ordinal:     2,
						},
						Kind:           core.CaseKindCode,
						Block:          "verify:board",
						Label:          "verify:board @ 생성한 보드 확인",
						Template:       "board \"${boardName}\" should exist",
						RenderedSource: "board \"board-1\" should exist",
						Status:         core.StatusPassed,
					},
					{
						ID: core.SpecID{
							File:        "specs/pocket-board.spec.md",
							HeadingPath: []string{"Pocket Board", "변수 흐름", "표 기반 확인"},
							Ordinal:     3,
						},
						Kind:          core.CaseKindTableRow,
						Fixture:       "board-exists",
						Label:         "fixture:board-exists @ 표 기반 확인 row 1",
						Columns:       []string{"board", "exists"},
						TemplateCells: []string{"${boardName}", "예"},
						RenderedCells: []string{"board-1", "예"},
						RowNumber:     1,
						Status:        core.StatusPassed,
					},
					{
						ID: core.SpecID{
							File:        "specs/pocket-board.spec.md",
							HeadingPath: []string{"Pocket Board", "변수 흐름", "표 기반 확인"},
							Ordinal:     4,
						},
						Kind:          core.CaseKindTableRow,
						Fixture:       "board-exists",
						Label:         "fixture:board-exists @ 표 기반 확인 row 2",
						Columns:       []string{"board", "exists"},
						TemplateCells: []string{"${boardName}-archive", "예"},
						RenderedCells: []string{"board-1-archive", "예"},
						RowNumber:     2,
						Status:        core.StatusFailed,
						Message:       "expected board \"board-1-archive\" to exist; actual boards: [\"board-1\"]",
						Expected:      "board \"board-1-archive\" exists",
						Actual:        "boards: [\"board-1\"]",
					},
				},
			},
		},
	}

	if err := Write(report, outPath); err != nil {
		t.Fatalf("write report: %v", err)
	}

	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	html := string(body)
	if !strings.Contains(html, "<h1>Pocket Board</h1>") {
		t.Fatalf("expected markdown heading in html, got %q", html)
	}
	if !strings.Contains(html, "case pass 3") {
		t.Fatalf("expected pass summary, got %q", html)
	}
	if !strings.Contains(html, "case fail 1") {
		t.Fatalf("expected fail summary, got %q", html)
	}
	if !strings.Contains(html, "captured bindings: boardName=board-1") {
		t.Fatalf("expected binding note, got %q", html)
	}
	if !strings.Contains(html, "fixture:board-exists") {
		t.Fatalf("expected fixture label, got %q", html)
	}
	if !strings.Contains(html, "board-1-archive") {
		t.Fatalf("expected resolved table cell, got %q", html)
	}
	if !strings.Contains(html, "expected board &#34;board-1-archive&#34; to exist; actual boards: [&#34;board-1&#34;]") {
		t.Fatalf("expected failure message, got %q", html)
	}
	if !strings.Contains(html, "<dt>expected</dt><dd>board &#34;board-1-archive&#34; exists</dd>") {
		t.Fatalf("expected structured expected diff, got %q", html)
	}
	if !strings.Contains(html, "<dt>actual</dt><dd>boards: [&#34;board-1&#34;]</dd>") {
		t.Fatalf("expected structured actual diff, got %q", html)
	}
	if !strings.Contains(html, "fixture:board-exists @ 표 기반 확인 row 2") {
		t.Fatalf("expected failure label, got %q", html)
	}
	if !strings.Contains(html, "id=\"case-specs-pocket-board-spec-md-pocket-board-변수-흐름-표-기반-확인-4\"") {
		t.Fatalf("expected failure anchor link, got %q", html)
	}
}

func TestWriteRendersAlloyReferencesAndArtifacts(t *testing.T) {
	outDir := t.TempDir()
	reportPath := filepath.Join(outDir, "report.html")
	bundlePath := filepath.Join(outDir, "alloy", "board.als")
	sourceMapPath := bundlePath + ".map.json"
	counterexamplePath := filepath.Join(outDir, "counterexamples", "case-board.json")

	if err := os.MkdirAll(filepath.Dir(bundlePath), 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(counterexamplePath), 0o755); err != nil {
		t.Fatalf("mkdir counterexample: %v", err)
	}
	if err := os.WriteFile(bundlePath, []byte("module board\n"), 0o644); err != nil {
		t.Fatalf("write bundle: %v", err)
	}
	if err := os.WriteFile(sourceMapPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write source map: %v", err)
	}
	if err := os.WriteFile(counterexamplePath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write counterexample: %v", err)
	}

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
						core.HeadingNode{Level: 1, Text: "Pocket Board", Raw: "# Pocket Board\n"},
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
								HeadingPath: []string{"Pocket Board", "형식 규칙"},
								Ordinal:     1,
							},
						},
					},
				},
				AlloyChecks: []core.AlloyCheckResult{
					{
						ID: core.SpecID{
							File:        "specs/pocket-board.spec.md",
							HeadingPath: []string{"Pocket Board", "형식 규칙"},
							Ordinal:     1,
						},
						Model:              "board",
						Assertion:          "cardShape",
						Scope:              "5",
						Label:              "alloy:ref(board#cardShape, scope=5) @ 형식 규칙",
						Status:             core.StatusFailed,
						Message:            "found counterexample for assertion \"cardShape\" at scope 5",
						Expected:           "assertion \"cardShape\" holds for scope 5",
						Actual:             "counterexample found",
						BundlePath:         bundlePath,
						SourceMapPath:      sourceMapPath,
						SourceRef:          "specs/pocket-board.spec.md#Pocket Board/형식 규칙",
						BundleLine:         7,
						CounterexamplePath: counterexamplePath,
					},
				},
			},
		},
	}

	if err := Write(report, reportPath); err != nil {
		t.Fatalf("write report: %v", err)
	}

	body, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	html := string(body)
	if !strings.Contains(html, "alloy fail 1") {
		t.Fatalf("expected alloy summary, got %q", html)
	}
	if !strings.Contains(html, "alloy:model(board)") {
		t.Fatalf("expected alloy model label, got %q", html)
	}
	if !strings.Contains(html, "alloy:ref(board#cardShape, scope=5)") {
		t.Fatalf("expected alloy ref label, got %q", html)
	}
	if !strings.Contains(html, "bundle artifact") {
		t.Fatalf("expected bundle artifact note, got %q", html)
	}
	if !strings.Contains(html, "source map") {
		t.Fatalf("expected source map note, got %q", html)
	}
	if !strings.Contains(html, "counterexample") {
		t.Fatalf("expected counterexample note, got %q", html)
	}
	if !strings.Contains(html, "source ref") || !strings.Contains(html, "bundle line") {
		t.Fatalf("expected structured source location, got %q", html)
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
								HeadingPath: []string{"Pocket Board", "보드 생성"},
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
								HeadingPath: []string{"Pocket Board", "형식 규칙"},
								Ordinal:     2,
							},
						},
					},
				},
				AlloyChecks: []core.AlloyCheckResult{
					{
						ID: core.SpecID{
							File:        "specs/pocket-board.spec.md",
							HeadingPath: []string{"Pocket Board", "형식 규칙"},
							Ordinal:     2,
						},
						Model:     "board",
						Assertion: "cardShape",
						Scope:     "5",
						Label:     "alloy:ref(board#cardShape, scope=5) @ 형식 규칙",
						Status:    core.StatusPassed,
					},
				},
			},
		},
	}

	if err := Write(report, outPath); err != nil {
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
