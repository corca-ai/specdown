package core

import (
	"strings"
	"testing"
)

func TestParseDocumentBuildsHeadingPathAndExecutableID(t *testing.T) {
	doc, err := ParseDocument("pocket-board.spec.md", strings.Join([]string{
		"# Pocket Board",
		"",
		"Intro prose.",
		"",
		"## First Executable Check",
		"",
		"```run:board",
		"create-board \"demo\"",
		"```",
		"",
	}, "\n"))
	if err != nil {
		t.Fatalf("parse document: %v", err)
	}

	if doc.Title != "Pocket Board" {
		t.Fatalf("unexpected title %q", doc.Title)
	}

	var code CodeBlockNode
	found := false
	for _, node := range doc.Nodes {
		current, ok := node.(CodeBlockNode)
		if ok {
			code = current
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected code block node")
	}
	if code.ID == nil {
		t.Fatal("expected executable block id")
	}
	if got, want := code.ID.HeadingPath, []string{"Pocket Board", "First Executable Check"}; strings.Join(got, " / ") != strings.Join(want, " / ") {
		t.Fatalf("unexpected heading path %#v", got)
	}
	if code.ID.Ordinal != 1 {
		t.Fatalf("unexpected ordinal %d", code.ID.Ordinal)
	}
}

func TestParseDocumentRejectsUnsupportedReservedBlock(t *testing.T) {
	_, err := ParseDocument("bad.spec.md", "```verify:board\nnoop\n```\n")
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "unsupported spec block") {
		t.Fatalf("unexpected error %v", err)
	}
}
