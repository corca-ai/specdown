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
