package core

import (
	"strings"
	"testing"
)

func TestParseDocumentBuildsHeadingPathAndExecutableIDs(t *testing.T) {
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
		"## Verify Created Board",
		"",
		"```verify:board",
		"board \"demo\" should exist",
		"```",
		"",
	}, "\n"))
	if err != nil {
		t.Fatalf("parse document: %v", err)
	}

	if doc.Title != "Pocket Board" {
		t.Fatalf("unexpected title %q", doc.Title)
	}

	var blocks []CodeBlockNode
	for _, node := range doc.Nodes {
		current, ok := node.(CodeBlockNode)
		if ok {
			blocks = append(blocks, current)
		}
	}

	if len(blocks) != 2 {
		t.Fatalf("expected 2 code blocks, got %d", len(blocks))
	}
	if blocks[0].Block.Kind != BlockKindRun || blocks[1].Block.Kind != BlockKindVerify {
		t.Fatalf("unexpected block kinds %#v", blocks)
	}
	if blocks[0].ID == nil || blocks[1].ID == nil {
		t.Fatal("expected executable ids")
	}
	if got, want := blocks[0].ID.HeadingPath, []string{"Pocket Board", "First Executable Check"}; strings.Join(got, " / ") != strings.Join(want, " / ") {
		t.Fatalf("unexpected first heading path %#v", got)
	}
	if got, want := blocks[1].ID.HeadingPath, []string{"Pocket Board", "Verify Created Board"}; strings.Join(got, " / ") != strings.Join(want, " / ") {
		t.Fatalf("unexpected second heading path %#v", got)
	}
	if blocks[0].ID.Ordinal != 1 || blocks[1].ID.Ordinal != 2 {
		t.Fatalf("unexpected ordinals %d, %d", blocks[0].ID.Ordinal, blocks[1].ID.Ordinal)
	}
}

func TestParseDocumentRejectsUnsupportedReservedBlock(t *testing.T) {
	_, err := ParseDocument("bad.spec.md", "```expect\n${value} matches /x/\n```\n")
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "unsupported spec block") {
		t.Fatalf("unexpected error %v", err)
	}
}
