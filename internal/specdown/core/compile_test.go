package core

import (
	"strings"
	"testing"
)

func TestCompileDocumentAcceptsVisibleCapturedVariable(t *testing.T) {
	doc, err := ParseDocument("specs/pocket-board.spec.md", strings.Join([]string{
		"# Pocket Board",
		"",
		"## Create",
		"",
		"```run:board -> $boardName",
		"create-board",
		"```",
		"",
		"### Verify",
		"",
		"```run:board",
		"board \"${boardName}\" should exist",
		"```",
		"",
	}, "\n"), nil)

	if err != nil {
		t.Fatalf("parse document: %v", err)
	}

	plan, err := CompileDocument(doc)
	if err != nil {
		t.Fatalf("compile document: %v", err)
	}
	if len(plan.Cases) != 2 {
		t.Fatalf("expected 2 cases, got %d", len(plan.Cases))
	}
	if got := plan.Cases[1].References; len(got) != 1 || got[0] != "boardName" {
		t.Fatalf("unexpected references %#v", got)
	}
}

func TestCompileDocumentAcceptsVariablesInCheckRows(t *testing.T) {
	doc, err := ParseDocument("specs/pocket-board.spec.md", strings.Join([]string{
		"# Pocket Board",
		"",
		"## Variable Flow",
		"",
		"```run:board -> $boardName",
		"create-board",
		"```",
		"",
		"### Table Check",
		"",
		"> check:board-exists",
		"| board | exists |",
		"| --- | --- |",
		"| ${boardName} | yes |",
		"| ${boardName}-archive | yes |",
		"",
	}, "\n"), nil)

	if err != nil {
		t.Fatalf("parse document: %v", err)
	}

	plan, err := CompileDocument(doc)
	if err != nil {
		t.Fatalf("compile document: %v", err)
	}
	if len(plan.Cases) != 3 {
		t.Fatalf("expected 3 cases, got %d", len(plan.Cases))
	}
	if plan.Cases[1].Kind != CaseKindTableRow || plan.Cases[1].TableRow.Check != "board-exists" {
		t.Fatalf("unexpected table case %#v", plan.Cases[1])
	}
	if got := plan.Cases[1].References; len(got) != 1 || got[0] != "boardName" {
		t.Fatalf("unexpected references %#v", got)
	}
}

func TestCompileDocumentAllowsSiblingVariableReference(t *testing.T) {
	doc, err := ParseDocument("specs/pocket-board.spec.md", strings.Join([]string{
		"# Pocket Board",
		"",
		"## Create",
		"",
		"```run:board -> $boardName",
		"create-board",
		"```",
		"",
		"## Verify",
		"",
		"```run:board",
		"board \"${boardName}\" should exist",
		"```",
		"",
	}, "\n"), nil)

	if err != nil {
		t.Fatalf("parse document: %v", err)
	}

	_, err = CompileDocument(doc)
	if err != nil {
		t.Fatalf("expected sibling variable to be visible: %v", err)
	}
}

func TestCompileDocumentCollectsAlloyModelsAndChecks(t *testing.T) {
	doc, err := ParseDocument("specs/pocket-board.spec.md", strings.Join([]string{
		"# Pocket Board",
		"",
		"## Formal Rules",
		"",
		"```alloy:model(board)",
		"module board",
		"",
		"sig Card {}",
		"```",
		"",
		"```alloy:model(board)",
		"assert cardExists { some Card }",
		"```",
		"",
		"> alloy:ref(board#cardExists, scope=5)",
		"",
	}, "\n"), nil)

	if err != nil {
		t.Fatalf("parse document: %v", err)
	}

	plan, err := CompileDocument(doc)
	if err != nil {
		t.Fatalf("compile document: %v", err)
	}
	if len(plan.AlloyModels) != 1 {
		t.Fatalf("expected 1 alloy model, got %d", len(plan.AlloyModels))
	}
	if got := len(plan.AlloyModels[0].Fragments); got != 2 {
		t.Fatalf("expected 2 fragments, got %d", got)
	}
	// Find alloy cases
	var alloyCases []CaseSpec
	for _, c := range plan.Cases {
		if c.Kind == CaseKindAlloy {
			alloyCases = append(alloyCases, c)
		}
	}
	if len(alloyCases) != 1 {
		t.Fatalf("expected 1 alloy case, got %d", len(alloyCases))
	}
	if got := alloyCases[0]; got.Alloy.Model != "board" || got.Alloy.Assertion != "cardExists" || got.Alloy.Scope != "5" {
		t.Fatalf("unexpected alloy case %#v", got)
	}
}

func TestCompileDocumentRejectsAlloyModuleRedeclarationInLaterFragment(t *testing.T) {
	doc, err := ParseDocument("specs/pocket-board.spec.md", strings.Join([]string{
		"# Pocket Board",
		"",
		"## Formal Rules",
		"",
		"```alloy:model(board)",
		"module board",
		"```",
		"",
		"```alloy:model(board)",
		"module board",
		"```",
		"",
	}, "\n"), nil)

	if err != nil {
		t.Fatalf("parse document: %v", err)
	}

	_, err = CompileDocument(doc)
	if err == nil {
		t.Fatal("expected compile error")
	}
	if !strings.Contains(err.Error(), "may declare module only in its first fragment") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestCompileDocumentRejectsAlloyReferenceToUnknownModel(t *testing.T) {
	doc, err := ParseDocument("specs/pocket-board.spec.md", strings.Join([]string{
		"# Pocket Board",
		"",
		"## Formal Rules",
		"",
		"> alloy:ref(board#cardExists, scope=5)",
		"",
	}, "\n"), nil)

	if err != nil {
		t.Fatalf("parse document: %v", err)
	}

	_, err = CompileDocument(doc)
	if err == nil {
		t.Fatal("expected compile error")
	}
	if !strings.Contains(err.Error(), `targets unknown model "board"`) {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestCompileDocumentParsesAlloyRunWithNestedBraces(t *testing.T) {
	// Alloy run predicates can contain set comprehensions with nested braces.
	// The parser must handle: run foo { some x: T | x in {y: T | ...} } for 5
	doc, err := ParseDocument("specs/nested.spec.md", strings.Join([]string{
		"# Nested",
		"",
		"## Model",
		"",
		"```alloy:model(m)",
		"module m",
		"sig T { f: lone T }",
		"pred linked { some x: T | x.f in {y: T | some y.f} }",
		"run linked { some x: T | x in {y: T | some y.f} } for 5",
		"```",
		"",
	}, "\n"), nil)
	if err != nil {
		t.Fatalf("parse document: %v", err)
	}

	plan, err := CompileDocument(doc)
	if err != nil {
		t.Fatalf("compile document: %v", err)
	}

	var alloyCases []CaseSpec
	for _, c := range plan.Cases {
		if c.Kind == CaseKindAlloy {
			alloyCases = append(alloyCases, c)
		}
	}
	if len(alloyCases) != 1 {
		t.Fatalf("expected 1 alloy case from implicit run, got %d", len(alloyCases))
	}
	ac := alloyCases[0]
	if ac.Alloy.Assertion != "linked" {
		t.Fatalf("expected assertion 'linked', got %q", ac.Alloy.Assertion)
	}
	if ac.Alloy.Scope != "5" {
		t.Fatalf("expected scope '5', got %q", ac.Alloy.Scope)
	}
	if !ac.Alloy.IsRun {
		t.Fatal("expected IsRun=true for implicit run statement")
	}
}

func TestCompileDocumentAllowsUnresolvedVariablesInRawBlock(t *testing.T) {
	// !raw blocks skip interpolation, so ${shell_var} should not cause
	// an "unresolved variable" compile error.
	doc, err := ParseDocument("raw.spec.md", strings.Join([]string{
		"# Raw",
		"",
		"```run:shell !raw",
		`for f in *.md; do echo "${f}"; done`,
		"```",
		"",
	}, "\n"), nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = CompileDocument(doc)
	if err != nil {
		t.Fatalf("compile should succeed for !raw block with shell variables, got: %v", err)
	}
}

func TestCompileDocumentRejectsUnresolvedVariableInBlock(t *testing.T) {
	doc, err := ParseDocument("bad.spec.md", strings.Join([]string{
		"# Bad",
		"",
		"```run:shell",
		"echo ${missing}",
		"```",
		"",
	}, "\n"), nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = CompileDocument(doc)
	if err == nil {
		t.Fatal("expected compile error for unresolved variable")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestCompileDocumentRejectsUnresolvedVariableInProse(t *testing.T) {
	doc, err := ParseDocument("bad.spec.md", "# Bad\n\nThe value is ${undefined}.\n", nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = CompileDocument(doc)
	if err == nil {
		t.Fatal("expected compile error for unresolved variable in prose")
	}
	if !strings.Contains(err.Error(), "undefined") {
		t.Fatalf("unexpected error %v", err)
	}
}
