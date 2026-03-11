package engine

import (
	"testing"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

func TestAlloyPlaceholder(t *testing.T) {
	spec := core.CaseSpec{
		ID: core.SpecID{
			File:        "test.spec.md",
			HeadingPath: []string{"Root", "Section"},
			Ordinal:     3,
		},
		Kind:      core.CaseKindAlloy,
		Model:     "board",
		Assertion: "cardShape",
		Scope:     "5",
	}

	result := alloyPlaceholder(spec)

	if result.ID.Key() != spec.ID.Key() {
		t.Fatalf("expected ID %v, got %v", spec.ID, result.ID)
	}
	if result.Kind != core.CaseKindAlloy {
		t.Fatalf("expected kind alloy, got %q", result.Kind)
	}
	if result.Model != "board" {
		t.Fatalf("expected model 'board', got %q", result.Model)
	}
	if result.Assertion != "cardShape" {
		t.Fatalf("expected assertion 'cardShape', got %q", result.Assertion)
	}
	if result.Scope != "5" {
		t.Fatalf("expected scope '5', got %q", result.Scope)
	}
	if result.Label != "alloy:ref(board#cardShape, scope=5) @ Section" {
		t.Fatalf("unexpected label %q", result.Label)
	}
	if result.Status != "" {
		t.Fatalf("placeholder should have empty status, got %q", result.Status)
	}
}

func TestAlloyPlaceholderNoHeading(t *testing.T) {
	spec := core.CaseSpec{
		ID:        core.SpecID{File: "test.spec.md", Ordinal: 1},
		Kind:      core.CaseKindAlloy,
		Model:     "m",
		Assertion: "a",
		Scope:     "3",
	}
	result := alloyPlaceholder(spec)
	if result.Label != "alloy:ref(m#a, scope=3)" {
		t.Fatalf("unexpected label %q", result.Label)
	}
}

func TestPeekNextPath(t *testing.T) {
	cases := []core.CaseSpec{
		{ID: core.SpecID{HeadingPath: []string{"A"}}},
		{ID: core.SpecID{HeadingPath: []string{"B"}}},
		{ID: core.SpecID{HeadingPath: []string{"C"}}},
	}

	// First element → returns second element's path
	got := peekNextPath(cases, 0)
	if len(got) != 1 || got[0] != "B" {
		t.Fatalf("expected [B], got %v", got)
	}

	// Middle element → returns next element's path
	got = peekNextPath(cases, 1)
	if len(got) != 1 || got[0] != "C" {
		t.Fatalf("expected [C], got %v", got)
	}

	// Last element → returns nil
	got = peekNextPath(cases, 2)
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestPeekNextPathEmpty(t *testing.T) {
	got := peekNextPath(nil, 0)
	if got != nil {
		t.Fatalf("expected nil for empty cases, got %v", got)
	}
}

func TestRecordResultPassedAddsBindings(t *testing.T) {
	ctx := &caseRunContext{
		bindings: newBindingsManager(),
		results:  make([]core.CaseResult, 0),
		status:   core.StatusPassed,
	}

	result := core.CaseResult{
		ID:     core.SpecID{HeadingPath: []string{"Root", "Section"}},
		Status: core.StatusPassed,
		Bindings: []core.Binding{
			{Name: "x", Value: "42"},
		},
	}

	ctx.recordResult(result, core.HeadingPath{"Root", "Section"})

	if len(ctx.results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(ctx.results))
	}
	if ctx.status != core.StatusPassed {
		t.Fatalf("expected status passed, got %q", ctx.status)
	}
	visible := ctx.bindings.VisibleAt(core.HeadingPath{"Root", "Section"})
	if len(visible) != 1 || visible[0].Name != "x" {
		t.Fatalf("expected binding x, got %v", visible)
	}
}

func TestRecordResultFailedSetsStatusAndSkipsBindings(t *testing.T) {
	ctx := &caseRunContext{
		bindings: newBindingsManager(),
		results:  make([]core.CaseResult, 0),
		status:   core.StatusPassed,
	}

	result := core.CaseResult{
		ID:     core.SpecID{HeadingPath: []string{"Root"}},
		Status: core.StatusFailed,
		Bindings: []core.Binding{
			{Name: "x", Value: "42"},
		},
	}

	ctx.recordResult(result, core.HeadingPath{"Root"})

	if ctx.status != core.StatusFailed {
		t.Fatalf("expected status failed, got %q", ctx.status)
	}
	// Bindings should NOT be recorded for failed cases
	visible := ctx.bindings.VisibleAt(core.HeadingPath{"Root"})
	if len(visible) != 0 {
		t.Fatalf("expected no bindings, got %v", visible)
	}
}

func TestRecordResultExpectFailDoesNotSetFailed(t *testing.T) {
	ctx := &caseRunContext{
		bindings: newBindingsManager(),
		results:  make([]core.CaseResult, 0),
		status:   core.StatusPassed,
	}

	result := core.CaseResult{
		ID:         core.SpecID{HeadingPath: []string{"Root"}},
		Status:     core.StatusFailed,
		ExpectFail: true,
	}

	ctx.recordResult(result, core.HeadingPath{"Root"})

	if ctx.status != core.StatusPassed {
		t.Fatalf("expected status to remain passed for expect-fail, got %q", ctx.status)
	}
}

func TestRecordResultMultipleCasesAccumulate(t *testing.T) {
	ctx := &caseRunContext{
		bindings: newBindingsManager(),
		results:  make([]core.CaseResult, 0),
		status:   core.StatusPassed,
	}

	ctx.recordResult(core.CaseResult{
		ID: core.SpecID{HeadingPath: []string{"A"}}, Status: core.StatusPassed,
		Bindings: []core.Binding{{Name: "a", Value: "1"}},
	}, core.HeadingPath{"A"})

	ctx.recordResult(core.CaseResult{
		ID: core.SpecID{HeadingPath: []string{"B"}}, Status: core.StatusPassed,
		Bindings: []core.Binding{{Name: "b", Value: "2"}},
	}, core.HeadingPath{"B"})

	if len(ctx.results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(ctx.results))
	}
	if ctx.status != core.StatusPassed {
		t.Fatalf("expected passed, got %q", ctx.status)
	}
	// Both bindings should be visible from a sibling path
	visible := ctx.bindings.VisibleAt(core.HeadingPath{"C"})
	if len(visible) != 2 {
		t.Fatalf("expected 2 visible bindings, got %v", visible)
	}
}

func TestShouldRunHookSetupOnce(t *testing.T) {
	hook := core.HookSpec{
		Kind:        core.HookSetup,
		Each:        false,
		HeadingPath: core.HeadingPath{"Root"},
	}

	// First entry into scope: should trigger
	if !shouldRunHook(hook, nil, core.HeadingPath{"Root", "A"}) {
		t.Fatal("setup-once should run on first entry")
	}

	// Already inside scope: should NOT trigger again
	if shouldRunHook(hook, core.HeadingPath{"Root", "A"}, core.HeadingPath{"Root", "B"}) {
		t.Fatal("setup-once should not run again inside same scope")
	}

	// Outside scope: should NOT trigger
	if shouldRunHook(hook, nil, core.HeadingPath{"Other"}) {
		t.Fatal("setup-once should not run outside scope")
	}
}

func TestShouldRunHookSetupEach(t *testing.T) {
	hook := core.HookSpec{
		Kind:        core.HookSetup,
		Each:        true,
		HeadingPath: core.HeadingPath{"Root"},
	}

	// First entry into child section
	if !shouldRunHook(hook, nil, core.HeadingPath{"Root", "A"}) {
		t.Fatal("setup-each should run on first entry")
	}

	// Same section: should NOT trigger
	if shouldRunHook(hook, core.HeadingPath{"Root", "A"}, core.HeadingPath{"Root", "A"}) {
		t.Fatal("setup-each should not re-run within same section")
	}

	// Different child section: should trigger
	if !shouldRunHook(hook, core.HeadingPath{"Root", "A"}, core.HeadingPath{"Root", "B"}) {
		t.Fatal("setup-each should run when section changes")
	}

	// Case at hook depth (no child): should NOT trigger
	if shouldRunHook(hook, nil, core.HeadingPath{"Root"}) {
		t.Fatal("setup-each should not run at hook depth without child")
	}
}

func TestShouldRunTeardownOnce(t *testing.T) {
	hook := core.HookSpec{
		Kind:        core.HookTeardown,
		Each:        false,
		HeadingPath: core.HeadingPath{"Root"},
	}

	// Leaving scope (next is outside): should trigger
	if !shouldRunTeardownHook(hook, core.HeadingPath{"Root", "A"}, nil) {
		t.Fatal("teardown-once should run when leaving scope")
	}

	// Still inside scope: should NOT trigger
	if shouldRunTeardownHook(hook, core.HeadingPath{"Root", "A"}, core.HeadingPath{"Root", "B"}) {
		t.Fatal("teardown-once should not run while still in scope")
	}

	// Outside scope: should NOT trigger
	if shouldRunTeardownHook(hook, core.HeadingPath{"Other"}, nil) {
		t.Fatal("teardown-once should not run outside scope")
	}
}

func TestShouldRunTeardownEach(t *testing.T) {
	hook := core.HookSpec{
		Kind:        core.HookTeardown,
		Each:        true,
		HeadingPath: core.HeadingPath{"Root"},
	}

	// Section changes: should trigger
	if !shouldRunTeardownHook(hook, core.HeadingPath{"Root", "A"}, core.HeadingPath{"Root", "B"}) {
		t.Fatal("teardown-each should run when section changes")
	}

	// Last case (next is nil): should trigger
	if !shouldRunTeardownHook(hook, core.HeadingPath{"Root", "A"}, nil) {
		t.Fatal("teardown-each should run on last case")
	}

	// Same section: should NOT trigger
	if shouldRunTeardownHook(hook, core.HeadingPath{"Root", "A"}, core.HeadingPath{"Root", "A"}) {
		t.Fatal("teardown-each should not run within same section")
	}

	// Case at hook depth: should NOT trigger
	if shouldRunTeardownHook(hook, core.HeadingPath{"Root"}, nil) {
		t.Fatal("teardown-each should not run at hook depth without child")
	}
}

func TestMergeAlloyResultsEmpty(t *testing.T) {
	cases := []core.CaseResult{
		{ID: core.SpecID{Ordinal: 1}, Kind: core.CaseKindCode, Status: core.StatusPassed},
	}
	merged, failed := mergeAlloyResults(cases, nil)
	if failed {
		t.Fatal("should not report failure with no alloy results")
	}
	if len(merged) != 1 {
		t.Fatalf("expected 1 case, got %d", len(merged))
	}
}

func TestMergeAlloyResultsReplacesPlaceholder(t *testing.T) {
	placeholder := core.CaseResult{
		ID:   core.SpecID{File: "a.md", HeadingPath: []string{"X"}, Ordinal: 1},
		Kind: core.CaseKindAlloy,
	}
	actual := core.CaseResult{
		ID:     core.SpecID{File: "a.md", HeadingPath: []string{"X"}, Ordinal: 1},
		Kind:   core.CaseKindAlloy,
		Status: core.StatusPassed,
		Label:  "real result",
	}
	cases := []core.CaseResult{placeholder}
	merged, failed := mergeAlloyResults(cases, []core.CaseResult{actual})
	if failed {
		t.Fatal("should not report failure for passing alloy")
	}
	if merged[0].Label != "real result" {
		t.Fatalf("placeholder was not replaced, label=%q", merged[0].Label)
	}
}

func TestMergeAlloyResultsReportsFailure(t *testing.T) {
	placeholder := core.CaseResult{
		ID:   core.SpecID{File: "a.md", HeadingPath: []string{"X"}, Ordinal: 1},
		Kind: core.CaseKindAlloy,
	}
	actual := core.CaseResult{
		ID:     core.SpecID{File: "a.md", HeadingPath: []string{"X"}, Ordinal: 1},
		Kind:   core.CaseKindAlloy,
		Status: core.StatusFailed,
	}
	_, failed := mergeAlloyResults([]core.CaseResult{placeholder}, []core.CaseResult{actual})
	if !failed {
		t.Fatal("should report failure for failing alloy result")
	}
}

func TestAccumulateSummaryPassed(t *testing.T) {
	summary := core.Summary{SpecsTotal: 1}
	accumulateSummary(&summary, core.DocumentResult{
		Status: core.StatusPassed,
		Cases: []core.CaseResult{
			{Status: core.StatusPassed},
			{Status: core.StatusPassed},
		},
	})
	if summary.SpecsPassed != 1 || summary.SpecsFailed != 0 {
		t.Fatalf("unexpected spec counts: %+v", summary)
	}
	if summary.CasesPassed != 2 || summary.CasesFailed != 0 {
		t.Fatalf("unexpected case counts: %+v", summary)
	}
}

func TestAccumulateSummaryFailedWithExpectFail(t *testing.T) {
	summary := core.Summary{SpecsTotal: 1}
	accumulateSummary(&summary, core.DocumentResult{
		Status: core.StatusFailed,
		Cases: []core.CaseResult{
			{Status: core.StatusPassed},
			{Status: core.StatusFailed, ExpectFail: true},
			{Status: core.StatusFailed},
		},
	})
	if summary.SpecsFailed != 1 {
		t.Fatalf("expected 1 failed spec, got %d", summary.SpecsFailed)
	}
	if summary.CasesPassed != 1 || summary.CasesFailed != 1 || summary.CasesExpectedFail != 1 {
		t.Fatalf("unexpected case counts: %+v", summary)
	}
}
