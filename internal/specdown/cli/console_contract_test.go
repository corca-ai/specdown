package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

func TestConsoleOutputContract(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := command{stdout: &stdout, stderr: &stderr}
	failedExitCode := 7

	results := []core.CaseResult{
		{
			ID:         consoleContractID("Success", 1, 10),
			Kind:       core.CaseKindCode,
			Status:     core.StatusPassed,
			DurationMs: 2,
			Code:       &core.CodeResultDetail{Block: "run:shell"},
		},
		{
			ID:              consoleContractID("Failure", 2, 20),
			Kind:            core.CaseKindCode,
			Status:          core.StatusFailed,
			DurationMs:      3,
			Message:         "command failed\nsecond line",
			Expected:        "expected value",
			Actual:          "actual value",
			VisibleBindings: []core.Binding{{Name: "value", Value: 42}},
			Code: &core.CodeResultDetail{
				Block:          "run:shell",
				RenderedSource: "printf actual",
				ExitCode:       &failedExitCode,
				Stderr:         "adapter stderr",
			},
		},
		{
			ID:         consoleContractID("Expected failure", 3, 30),
			Kind:       core.CaseKindInlineExpect,
			Status:     core.StatusFailed,
			DurationMs: 4,
			ExpectFail: true,
			Message:    "known mismatch",
			Expected:   "green",
			Actual:     "red",
		},
		{
			ID:         consoleContractID("Skipped", 4, 40),
			Kind:       core.CaseKindAlloy,
			Status:     core.StatusSkipped,
			DurationMs: 0,
			Alloy: &core.AlloyResultDetail{
				Model:     "board",
				Assertion: "ownership",
				Scope:     "for 5",
			},
		},
	}
	for index := range results {
		cmd.printCaseResult(results[index], index+1, len(results))
	}

	cmd.printLifecycleFailures(core.Report{
		LifecycleEvents: []core.LifecycleEvent{{
			Scope:      core.LifecycleScopeGlobal,
			Phase:      core.HookTeardown,
			Status:     core.StatusFailed,
			Message:    "global cleanup failed",
			DurationMs: 5,
		}},
		Results: []core.DocumentResult{{
			LifecycleEvents: []core.LifecycleEvent{{
				Scope:       core.LifecycleScopeSection,
				Phase:       core.HookSetup,
				Status:      core.StatusFailed,
				File:        "specs/contract.md",
				HeadingPath: core.HeadingPath{"Contract", "Lifecycle"},
				Each:        true,
				Message:     "section setup failed\nretry exhausted",
				DurationMs:  6,
			}},
		}},
	})

	cmd.printDryRun(core.Report{
		Results: []core.DocumentResult{{
			Document: core.Document{RelativeTo: "specs/contract.md"},
			Cases: []core.CaseResult{
				{ID: consoleContractID("Code", 1, 10), Code: &core.CodeResultDetail{Block: "run:shell"}},
				{ID: consoleContractID("Table", 2, 20), Table: &core.TableResultDetail{Check: "equals"}},
				{ID: consoleContractID("Inline", 3, 30)},
				{
					ID: consoleContractID("Alloy", 4, 40),
					Alloy: &core.AlloyResultDetail{
						Model:     "board",
						Assertion: "ownership",
						Scope:     "for 5",
					},
				},
			},
		}},
		Summary: core.Summary{SpecsTotal: 1, CasesTotal: 4},
	})

	got := "=== stdout ===\n" + stdout.String() + "=== stderr ===\n" + stderr.String()
	want, err := os.ReadFile("testdata/console.golden.txt")
	if err != nil {
		t.Fatalf("read console golden: %v", err)
	}
	if got != string(want) {
		t.Fatalf("console output changed:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func consoleContractID(heading string, ordinal, line int) core.SpecID {
	return core.SpecID{
		File:        "specs/contract.md",
		HeadingPath: core.HeadingPath{"Contract", heading},
		Ordinal:     ordinal,
		Line:        line,
	}
}
