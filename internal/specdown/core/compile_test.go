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
