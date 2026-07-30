package json

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

func TestWriteMatchesNormalizedReportGolden(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "report.json")
	if err := Write(contractReport(), outPath); err != nil {
		t.Fatalf("write report: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	want, err := os.ReadFile("testdata/report.golden.json")
	if err != nil {
		t.Fatalf("read golden report: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("report JSON changed (-want +got):\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func contractReport() core.Report {
	successExitCode := 0
	return core.Report{
		SchemaVersion: core.ReportSchemaVersion,
		Title:         "Contract report",
		GeneratedAt:   time.Date(2026, 7, 30, 1, 2, 3, 0, time.UTC),
		Results: []core.DocumentResult{{
			Document: core.Document{
				RelativeTo: "specs/contract.md",
				Title:      "Contract",
				Markdown:   "# Contract\n",
				Nodes:      []core.Node{},
				Frontmatter: core.Frontmatter{
					Type: "spec",
				},
				Warnings: []string{"example warning"},
			},
			Status: core.StatusFailed,
			Cases: []core.CaseResult{
				{
					ID: core.SpecID{
						File:        "specs/contract.md",
						HeadingPath: core.HeadingPath{"Contract", "Code"},
						Ordinal:     1,
						Line:        10,
					},
					Kind:       core.CaseKindCode,
					Status:     core.StatusPassed,
					Label:      "captures output",
					DurationMs: 12,
					Bindings: []core.Binding{{
						Name:  "captured",
						Value: "42",
					}},
					VisibleBindings: []core.Binding{{
						Name:  "seed",
						Value: 41,
					}},
					Events: []core.Event{
						{
							Type:  core.EventCaseStarted,
							ID:    core.SpecID{File: "specs/contract.md", HeadingPath: core.HeadingPath{"Contract", "Code"}, Ordinal: 1, Line: 10},
							Label: "captures output",
						},
						{
							Type: core.EventCasePassed,
							ID:   core.SpecID{File: "specs/contract.md", HeadingPath: core.HeadingPath{"Contract", "Code"}, Ordinal: 1, Line: 10},
							Bindings: []core.Binding{{
								Name:  "captured",
								Value: "42",
							}},
						},
					},
					Code: &core.CodeResultDetail{
						Block:          "run:shell",
						Template:       "echo {{seed}}",
						RenderedSource: "echo 41",
						ExitCode:       &successExitCode,
						Stderr:         "diagnostic",
					},
				},
				{
					ID: core.SpecID{
						File:        "specs/contract.md",
						HeadingPath: core.HeadingPath{"Contract", "Table"},
						Ordinal:     2,
						Line:        20,
					},
					Kind:       core.CaseKindTableRow,
					Status:     core.StatusFailed,
					Label:      "invalid row",
					DurationMs: 3,
					Message:    "values differ",
					Expected:   "yes",
					Actual:     "no",
					Events: []core.Event{{
						Type:     core.EventCaseFailed,
						ID:       core.SpecID{File: "specs/contract.md", HeadingPath: core.HeadingPath{"Contract", "Table"}, Ordinal: 2, Line: 20},
						Label:    "invalid row",
						Message:  "values differ",
						Expected: "yes",
						Actual:   "no",
					}},
					Table: &core.TableResultDetail{
						Check:         "equals",
						Columns:       []string{"actual", "expected"},
						TemplateCells: []string{"{{value}}", "yes"},
						RenderedCells: []string{"no", "yes"},
						RowNumber:     1,
					},
				},
				{
					ID: core.SpecID{
						File:        "specs/contract.md",
						HeadingPath: core.HeadingPath{"Contract", "Inline"},
						Ordinal:     3,
						Line:        30,
					},
					Kind:       core.CaseKindInlineExpect,
					Status:     core.StatusFailed,
					Label:      "known mismatch",
					ExpectFail: true,
					Message:    "expected mismatch",
					Expected:   "green",
					Actual:     "red",
					Events: []core.Event{{
						Type:     core.EventCaseFailed,
						ID:       core.SpecID{File: "specs/contract.md", HeadingPath: core.HeadingPath{"Contract", "Inline"}, Ordinal: 3, Line: 30},
						Label:    "known mismatch",
						Message:  "expected mismatch",
						Expected: "green",
						Actual:   "red",
					}},
				},
				{
					ID: core.SpecID{
						File:        "specs/contract.md",
						HeadingPath: core.HeadingPath{"Contract", "Alloy"},
						Ordinal:     4,
						Line:        40,
					},
					Kind:    core.CaseKindAlloy,
					Status:  core.StatusSkipped,
					Label:   "ownership",
					Message: "not executed after failure",
					Events: []core.Event{{
						Type:    core.EventCaseSkipped,
						ID:      core.SpecID{File: "specs/contract.md", HeadingPath: core.HeadingPath{"Contract", "Alloy"}, Ordinal: 4, Line: 40},
						Label:   "ownership",
						Message: "not executed after failure",
					}},
					Alloy: &core.AlloyResultDetail{
						Model:         "board",
						Assertion:     "singleOwnership",
						Scope:         "for 5",
						BundlePath:    ".artifacts/board.als",
						SourceMapPath: ".artifacts/board.map.json",
						BundleLine:    8,
						SourceRef:     "specs/contract.md:40",
					},
				},
			},
			LifecycleEvents: []core.LifecycleEvent{{
				Scope:       core.LifecycleScopeSection,
				Phase:       core.HookSetup,
				Status:      core.StatusPassed,
				File:        "specs/contract.md",
				HeadingPath: core.HeadingPath{"Contract"},
				Each:        true,
				DurationMs:  2,
			}},
		}},
		Summary: core.Summary{
			SpecsTotal:        1,
			SpecsFailed:       1,
			CasesTotal:        4,
			CasesPassed:       1,
			CasesFailed:       1,
			CasesSkipped:      1,
			CasesExpectedFail: 1,
			LifecycleTotal:    2,
			LifecyclePassed:   1,
			LifecycleFailed:   1,
			TraceErrorCount:   1,
		},
		LifecycleEvents: []core.LifecycleEvent{{
			Scope:      core.LifecycleScopeGlobal,
			Phase:      core.HookTeardown,
			Status:     core.StatusFailed,
			Message:    "exit status 7",
			DurationMs: 4,
		}},
		TraceErrors: []string{"specs/contract.md: missing covers edge"},
		TraceGraph: &core.TraceGraphData{
			Documents: []core.TraceDocument{
				{Path: "specs/contract.md", Type: "spec"},
				{Path: "goals/quality.md", Type: "goal"},
			},
			Edges: []core.TraceEdge{{
				Source:   "specs/contract.md",
				Target:   "goals/quality.md",
				EdgeName: "covers",
			}},
			TransitiveEdges: []core.TraceEdge{{
				Source:   "specs/contract.md",
				Target:   "goals/root.md",
				EdgeName: "covers",
			}},
		},
	}
}
