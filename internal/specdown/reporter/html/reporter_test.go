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
			CasesTotal:  3,
			CasesPassed: 2,
			CasesFailed: 1,
		},
		Results: []core.DocumentResult{
			{
				Status: core.StatusFailed,
				Document: core.Document{
					Title:      "Pocket Board",
					RelativeTo: "pocket-board.spec.md",
					Nodes: []core.Node{
						core.HeadingNode{Level: 1, Text: "Pocket Board", Raw: "# Pocket Board\n"},
						core.ProseNode{Raw: "\nPlain prose only.\n\n"},
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
						core.CodeBlockNode{
							Block:  core.BlockSpec{Raw: "verify:board", Kind: core.BlockKindVerify, Target: "board"},
							Source: "board \"${boardName}-archive\" should exist",
							Raw:    "```verify:board\nboard \"${boardName}-archive\" should exist\n```\n",
							ID: &core.SpecID{
								File:        "specs/pocket-board.spec.md",
								HeadingPath: []string{"Pocket Board", "Variable Flow", "Missing Boards Fail Verification"},
								Ordinal:     3,
							},
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
						Block:          "verify:board",
						Label:          "verify:board @ Verify Created Board",
						Template:       "board \"${boardName}\" should exist",
						RenderedSource: "board \"board-1\" should exist",
						Status:         core.StatusPassed,
					},
					{
						ID: core.SpecID{
							File:        "specs/pocket-board.spec.md",
							HeadingPath: []string{"Pocket Board", "Variable Flow", "Missing Boards Fail Verification"},
							Ordinal:     3,
						},
						Block:          "verify:board",
						Label:          "verify:board @ Missing Boards Fail Verification",
						Template:       "board \"${boardName}-archive\" should exist",
						RenderedSource: "board \"board-1-archive\" should exist",
						Status:         core.StatusFailed,
						Message:        "expected board \"board-1-archive\" to exist; actual boards: [\"board-1\"]",
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
	if !strings.Contains(html, "case pass 2") {
		t.Fatalf("expected pass summary, got %q", html)
	}
	if !strings.Contains(html, "case fail 1") {
		t.Fatalf("expected fail summary, got %q", html)
	}
	if !strings.Contains(html, "run:board") {
		t.Fatalf("expected executable block info, got %q", html)
	}
	if !strings.Contains(html, "verify:board") {
		t.Fatalf("expected verification block info, got %q", html)
	}
	if !strings.Contains(html, "create-board") {
		t.Fatalf("expected executable source, got %q", html)
	}
	if !strings.Contains(html, "board &#34;${boardName}&#34; should exist") {
		t.Fatalf("expected template verification source, got %q", html)
	}
	if !strings.Contains(html, "resolved input") || !strings.Contains(html, "board &#34;board-1&#34; should exist") {
		t.Fatalf("expected resolved verification source, got %q", html)
	}
	if !strings.Contains(html, "captured bindings: boardName=board-1") {
		t.Fatalf("expected binding note, got %q", html)
	}
	if !strings.Contains(html, "Failed cases") {
		t.Fatalf("expected failed case summary, got %q", html)
	}
	if !strings.Contains(html, "expected board &#34;board-1-archive&#34; to exist; actual boards: [&#34;board-1&#34;]") {
		t.Fatalf("expected failure message, got %q", html)
	}
	if !strings.Contains(html, "href=\"#case-specs-pocket-board-spec-md-pocket-board-variable-flow-missing-boards-fail-verification-3\"") {
		t.Fatalf("expected failure anchor link, got %q", html)
	}
}
