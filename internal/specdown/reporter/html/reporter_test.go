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
							Block:  core.BlockSpec{Raw: "run:board", Kind: core.BlockKindRun, Target: "board"},
							Source: "create-board \"demo\"",
							Raw:    "```run:board\ncreate-board \"demo\"\n```\n",
							ID: &core.SpecID{
								File:        "pocket-board.spec.md",
								HeadingPath: []string{"Pocket Board", "First Executable Check"},
								Ordinal:     1,
							},
						},
						core.CodeBlockNode{
							Block:  core.BlockSpec{Raw: "verify:board", Kind: core.BlockKindVerify, Target: "board"},
							Source: "board \"demo\" should exist",
							Raw:    "```verify:board\nboard \"demo\" should exist\n```\n",
							ID: &core.SpecID{
								File:        "pocket-board.spec.md",
								HeadingPath: []string{"Pocket Board", "Verify Created Board"},
								Ordinal:     2,
							},
						},
						core.CodeBlockNode{
							Block:  core.BlockSpec{Raw: "verify:board", Kind: core.BlockKindVerify, Target: "board"},
							Source: "board \"archive\" should exist",
							Raw:    "```verify:board\nboard \"archive\" should exist\n```\n",
							ID: &core.SpecID{
								File:        "pocket-board.spec.md",
								HeadingPath: []string{"Pocket Board", "Missing Boards Fail Verification"},
								Ordinal:     3,
							},
						},
					},
				},
				Cases: []core.CaseResult{
					{
						ID: core.SpecID{
							File:        "pocket-board.spec.md",
							HeadingPath: []string{"Pocket Board", "First Executable Check"},
							Ordinal:     1,
						},
						Info:   "run:board",
						Label:  "run:board @ First Executable Check",
						Source: "create-board \"demo\"",
						Status: core.StatusPassed,
					},
					{
						ID: core.SpecID{
							File:        "pocket-board.spec.md",
							HeadingPath: []string{"Pocket Board", "Verify Created Board"},
							Ordinal:     2,
						},
						Info:   "verify:board",
						Label:  "verify:board @ Verify Created Board",
						Source: "board \"demo\" should exist",
						Status: core.StatusPassed,
					},
					{
						ID: core.SpecID{
							File:        "pocket-board.spec.md",
							HeadingPath: []string{"Pocket Board", "Missing Boards Fail Verification"},
							Ordinal:     3,
						},
						Info:    "verify:board",
						Label:   "verify:board @ Missing Boards Fail Verification",
						Source:  "board \"archive\" should exist",
						Status:  core.StatusFailed,
						Message: "expected board \"archive\" to exist; actual boards: [\"demo\"]",
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
	if !strings.Contains(html, "create-board &#34;demo&#34;") {
		t.Fatalf("expected executable source, got %q", html)
	}
	if !strings.Contains(html, "board &#34;demo&#34; should exist") {
		t.Fatalf("expected verification source, got %q", html)
	}
	if !strings.Contains(html, "Failed cases") {
		t.Fatalf("expected failed case summary, got %q", html)
	}
	if !strings.Contains(html, "expected board &#34;archive&#34; to exist; actual boards: [&#34;demo&#34;]") {
		t.Fatalf("expected failure message, got %q", html)
	}
	if !strings.Contains(html, "href=\"#case-pocket-board-spec-md-pocket-board-missing-boards-fail-verification-3\"") {
		t.Fatalf("expected failure anchor link, got %q", html)
	}
}
