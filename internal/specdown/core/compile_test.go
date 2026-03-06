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
		"```verify:board",
		"board \"${boardName}\" should exist",
		"```",
		"",
	}, "\n"))
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

func TestCompileDocumentAcceptsVariablesInFixtureRows(t *testing.T) {
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
		"<!-- fixture:board-exists -->",
		"| board | exists |",
		"| --- | --- |",
		"| ${boardName} | 예 |",
		"| ${boardName}-archive | 예 |",
		"",
	}, "\n"))
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
	if plan.Cases[1].Kind != CaseKindTableRow || plan.Cases[1].Fixture != "board-exists" {
		t.Fatalf("unexpected table case %#v", plan.Cases[1])
	}
	if got := plan.Cases[1].References; len(got) != 1 || got[0] != "boardName" {
		t.Fatalf("unexpected references %#v", got)
	}
}

func TestCompileDocumentRejectsSiblingVariableReference(t *testing.T) {
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
		"```verify:board",
		"board \"${boardName}\" should exist",
		"```",
		"",
	}, "\n"))
	if err != nil {
		t.Fatalf("parse document: %v", err)
	}

	_, err = CompileDocument(doc)
	if err == nil {
		t.Fatal("expected compile error")
	}
	if !strings.Contains(err.Error(), `unresolved variable "boardName"`) {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestCompileDocumentCollectsAlloyModelsAndChecks(t *testing.T) {
	doc, err := ParseDocument("specs/pocket-board.spec.md", strings.Join([]string{
		"# Pocket Board",
		"",
		"## 형식 규칙",
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
		"<!-- alloy:ref(board#cardExists, scope=5) -->",
		"",
	}, "\n"))
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
	if len(plan.AlloyChecks) != 1 {
		t.Fatalf("expected 1 alloy check, got %d", len(plan.AlloyChecks))
	}
	if got := plan.AlloyChecks[0]; got.Model != "board" || got.Assertion != "cardExists" || got.Scope != "5" {
		t.Fatalf("unexpected alloy check %#v", got)
	}
}

func TestCompileDocumentRejectsAlloyModuleRedeclarationInLaterFragment(t *testing.T) {
	doc, err := ParseDocument("specs/pocket-board.spec.md", strings.Join([]string{
		"# Pocket Board",
		"",
		"## 형식 규칙",
		"",
		"```alloy:model(board)",
		"module board",
		"```",
		"",
		"```alloy:model(board)",
		"module board",
		"```",
		"",
	}, "\n"))
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
		"## 형식 규칙",
		"",
		"<!-- alloy:ref(board#cardExists, scope=5) -->",
		"",
	}, "\n"))
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
