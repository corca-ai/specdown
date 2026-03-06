package core

import (
	"strings"
	"testing"
)

func TestParseDocumentBuildsHeadingPathAndExecutableIDs(t *testing.T) {
	doc, err := ParseDocument("pocket-board.spec.md", strings.Join([]string{
		"# Pocket Board",
		"",
		"소개 문단.",
		"",
		"## 변수 흐름",
		"",
		"```run:board -> $boardName",
		"create-board",
		"```",
		"",
		"### 생성한 보드 확인",
		"",
		"```verify:board",
		"board \"${boardName}\" should exist",
		"```",
		"",
		"### 표 기반 확인",
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

	if doc.Title != "Pocket Board" {
		t.Fatalf("unexpected title %q", doc.Title)
	}

	var blocks []CodeBlockNode
	var tables []TableNode
	var headings []HeadingNode
	for _, node := range doc.Nodes {
		switch current := node.(type) {
		case HeadingNode:
			headings = append(headings, current)
		case CodeBlockNode:
			blocks = append(blocks, current)
		case TableNode:
			tables = append(tables, current)
		}
	}

	if len(blocks) != 2 {
		t.Fatalf("expected 2 code blocks, got %d", len(blocks))
	}
	if len(headings) != 4 {
		t.Fatalf("expected 4 headings, got %d", len(headings))
	}
	if got := headings[2].HeadingPath; len(got) != 3 || got[0] != "Pocket Board" || got[2] != "생성한 보드 확인" {
		t.Fatalf("unexpected heading path %#v", got)
	}
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	if tables[0].Fixture != "board-exists" {
		t.Fatalf("unexpected fixture %q", tables[0].Fixture)
	}
	if len(tables[0].Rows) != 2 {
		t.Fatalf("unexpected row count %d", len(tables[0].Rows))
	}
	if tables[0].Rows[0].ID == nil || tables[0].Rows[1].ID == nil {
		t.Fatal("expected executable table row ids")
	}
	if blocks[0].ID.Ordinal != 1 || blocks[1].ID.Ordinal != 2 || tables[0].Rows[0].ID.Ordinal != 3 || tables[0].Rows[1].ID.Ordinal != 4 {
		t.Fatalf("unexpected ordinals %d %d %d %d", blocks[0].ID.Ordinal, blocks[1].ID.Ordinal, tables[0].Rows[0].ID.Ordinal, tables[0].Rows[1].ID.Ordinal)
	}
}

func TestParseDocumentRejectsFixtureDirectiveWithoutTable(t *testing.T) {
	_, err := ParseDocument("bad.spec.md", strings.Join([]string{
		"# Bad",
		"",
		"<!-- fixture:board-exists -->",
		"",
		"not a table",
		"",
	}, "\n"))
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "must be followed by a table") {
		t.Fatalf("unexpected error %v", err)
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

func TestParseDocumentSupportsAlloyModelBlocksAndReferences(t *testing.T) {
	doc, err := ParseDocument("pocket-board.spec.md", strings.Join([]string{
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

	var (
		models []AlloyModelNode
		refs   []AlloyRefNode
	)
	for _, node := range doc.Nodes {
		switch current := node.(type) {
		case AlloyModelNode:
			models = append(models, current)
		case AlloyRefNode:
			refs = append(refs, current)
		}
	}

	if len(models) != 2 {
		t.Fatalf("expected 2 alloy model blocks, got %d", len(models))
	}
	if models[0].Model != "board" || models[1].Model != "board" {
		t.Fatalf("unexpected models %#v", models)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 alloy ref, got %d", len(refs))
	}
	if refs[0].ID == nil || refs[0].ID.Ordinal != 1 {
		t.Fatalf("unexpected alloy ref id %#v", refs[0].ID)
	}
	if refs[0].Assertion != "cardExists" || refs[0].Scope != "5" {
		t.Fatalf("unexpected alloy ref %#v", refs[0])
	}
}

func TestParseDocumentRejectsInvalidAlloyReferenceDirective(t *testing.T) {
	_, err := ParseDocument("bad.spec.md", strings.Join([]string{
		"# Bad",
		"",
		"<!-- alloy:ref(board#cardExists) -->",
		"",
	}, "\n"))
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "invalid alloy reference directive") {
		t.Fatalf("unexpected error %v", err)
	}
}
