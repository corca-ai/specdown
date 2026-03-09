package core

import (
	"strings"
	"testing"
)

func TestParseDocumentBuildsHeadingPathAndExecutableIDs(t *testing.T) {
	doc, err := ParseDocument("pocket-board.spec.md", strings.Join([]string{
		"# Pocket Board",
		"",
		"Introduction paragraph.",
		"",
		"## Variable Flow",
		"",
		"```run:board -> $boardName",
		"create-board",
		"```",
		"",
		"### Verify Created Board",
		"",
		"```run:board",
		"board \"${boardName}\" should exist",
		"```",
		"",
		"### Table Check",
		"",
		"> fixture:board-exists",
		"| board | exists |",
		"| --- | --- |",
		"| ${boardName} | yes |",
		"| ${boardName}-archive | yes |",
		"",
	}, "\n"), nil)

	if err != nil {
		t.Fatalf("parse document: %v", err)
	}

	blocks, tables, headings := classifyNodes(doc)

	t.Run("title", func(t *testing.T) {
		if doc.Title != "Pocket Board" {
			t.Fatalf("unexpected title %q", doc.Title)
		}
	})
	t.Run("headings", func(t *testing.T) {
		if len(headings) != 4 {
			t.Fatalf("expected 4 headings, got %d", len(headings))
		}
		if got := headings[2].HeadingPath; len(got) != 3 || got[0] != "Pocket Board" || got[2] != "Verify Created Board" {
			t.Fatalf("unexpected heading path %#v", got)
		}
	})
	t.Run("code_blocks", func(t *testing.T) {
		if len(blocks) != 2 {
			t.Fatalf("expected 2 code blocks, got %d", len(blocks))
		}
	})
	t.Run("tables", func(t *testing.T) {
		assertTableShape(t, tables)
	})
	t.Run("ordinals", func(t *testing.T) {
		assertOrdinals(t, blocks, tables)
	})
}

func classifyNodes(doc Document) ([]CodeBlockNode, []TableNode, []HeadingNode) {
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
	return blocks, tables, headings
}

func assertTableShape(t *testing.T, tables []TableNode) {
	t.Helper()
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
}

func assertOrdinals(t *testing.T, blocks []CodeBlockNode, tables []TableNode) {
	t.Helper()
	if blocks[0].ID.Ordinal != 1 || blocks[1].ID.Ordinal != 2 || tables[0].Rows[0].ID.Ordinal != 3 || tables[0].Rows[1].ID.Ordinal != 4 {
		t.Fatalf("unexpected ordinals %d %d %d %d", blocks[0].ID.Ordinal, blocks[1].ID.Ordinal, tables[0].Rows[0].ID.Ordinal, tables[0].Rows[1].ID.Ordinal)
	}
}

func TestParseDocumentRejectsFixtureDirectiveWithoutTable(t *testing.T) {
	_, err := ParseDocument("bad.spec.md", strings.Join([]string{
		"# Bad",
		"",
		"> fixture:board-exists",
		"",
		"not a table",
		"",
	}, "\n"), nil)

	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "must be followed by a table") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestParseDocumentTreatsExpectAsPlainCodeBlock(t *testing.T) {
	doc, err := ParseDocument("test.spec.md", "# T\n\n```expect\n${value} matches /x/\n```\n", nil)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	found := false
	for _, node := range doc.Nodes {
		if cb, ok := node.(CodeBlockNode); ok && cb.Block.Raw == "expect" {
			if cb.Block.Executable() {
				t.Fatal("expect block should not be executable")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("expected to find an expect code block")
	}
}

func TestParseDocumentSupportsAlloyModelBlocksAndReferences(t *testing.T) {
	doc, err := ParseDocument("pocket-board.spec.md", strings.Join([]string{
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

func TestParseDocumentExtractsFrontmatterTimeout(t *testing.T) {
	doc, err := ParseDocument("test.spec.md", "---\ntimeout: 3000\n---\n\n# Title\n\nBody.\n", nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Frontmatter.Timeout != 3000 {
		t.Fatalf("expected timeout 3000, got %d", doc.Frontmatter.Timeout)
	}
	if doc.Title != "Title" {
		t.Fatalf("expected title 'Title', got %q", doc.Title)
	}
}

func TestParseDocumentDefaultsFrontmatterWhenAbsent(t *testing.T) {
	doc, err := ParseDocument("test.spec.md", "# Title\n\nBody.\n", nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Frontmatter.Timeout != 0 {
		t.Fatalf("expected timeout 0, got %d", doc.Frontmatter.Timeout)
	}
}

func TestParseDocumentParsesFixtureParams(t *testing.T) {
	doc, err := ParseDocument("test.spec.md", strings.Join([]string{
		"# Test",
		"",
		"> fixture:write-permission(user=alan)",
		"| path | write |",
		"| --- | --- |",
		"| /tmp | yes |",
		"",
	}, "\n"), nil)

	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	var tables []TableNode
	for _, node := range doc.Nodes {
		if tbl, ok := node.(TableNode); ok {
			tables = append(tables, tbl)
		}
	}
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	if tables[0].Fixture != "write-permission" {
		t.Fatalf("unexpected fixture %q", tables[0].Fixture)
	}
	if tables[0].FixtureParams == nil {
		t.Fatal("expected fixture params")
	}
	if got := tables[0].FixtureParams["user"]; got != "alan" {
		t.Fatalf("expected param user=alan, got %q", got)
	}
}

func TestParseDocumentParsesFixtureMultipleParams(t *testing.T) {
	doc, err := ParseDocument("test.spec.md", strings.Join([]string{
		"# Test",
		"",
		"> fixture:editor-op(type=lexical, mode=rich)",
		"| input | output |",
		"| --- | --- |",
		"| a | b |",
		"",
	}, "\n"), nil)

	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	var tables []TableNode
	for _, node := range doc.Nodes {
		if tbl, ok := node.(TableNode); ok {
			tables = append(tables, tbl)
		}
	}
	if tables[0].FixtureParams["type"] != "lexical" || tables[0].FixtureParams["mode"] != "rich" {
		t.Fatalf("unexpected params %#v", tables[0].FixtureParams)
	}
}

func TestParseDocumentFixtureWithoutParamsHasNilParams(t *testing.T) {
	doc, err := ParseDocument("test.spec.md", strings.Join([]string{
		"# Test",
		"",
		"> fixture:board-exists",
		"| board | exists |",
		"| --- | --- |",
		"| x | yes |",
		"",
	}, "\n"), nil)

	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	var tables []TableNode
	for _, node := range doc.Nodes {
		if tbl, ok := node.(TableNode); ok {
			tables = append(tables, tbl)
		}
	}
	if tables[0].FixtureParams != nil {
		t.Fatalf("expected nil params for parameterless fixture, got %#v", tables[0].FixtureParams)
	}
}

func TestParseTableCellsWithEscapedPipe(t *testing.T) {
	doc, err := ParseDocument("test.spec.md", strings.Join([]string{
		"# Test",
		"",
		`> fixture:check`,
		`| input | expected |`,
		`| --- | --- |`,
		`| a\|b | a\|b |`,
		"",
	}, "\n"), nil)

	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	var tables []TableNode
	for _, node := range doc.Nodes {
		if tbl, ok := node.(TableNode); ok {
			tables = append(tables, tbl)
		}
	}
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	if len(tables[0].Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(tables[0].Rows))
	}
	// Raw cells preserve the escape sequence; UnescapeCell resolves it
	if got := tables[0].Rows[0].Cells[0]; got != `a\|b` {
		t.Fatalf("expected raw cell %q, got %q", `a\|b`, got)
	}
}

func TestUnescapeCell(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`hello`, "hello"},
		{`a\nb`, "a\nb"},
		{`a\|b`, "a|b"},
		{`a\\b`, `a\b`},
		{`\n\|\\\n`, "\n|\\\n"},
		{`no escapes`, "no escapes"},
		{`trailing\`, `trailing\`},
	}
	for _, tt := range tests {
		got := UnescapeCell(tt.input)
		if got != tt.want {
			t.Errorf("UnescapeCell(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseDocumentAllowsFixtureWithParamsAndNoTable(t *testing.T) {
	doc, err := ParseDocument("test.spec.md", strings.Join([]string{
		"# Test",
		"",
		"Some prose.",
		"> fixture:check-user(field=plan, expected=STANDARD)",
		"",
		"More prose.",
		"",
	}, "\n"), nil)

	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	var calls []FixtureCallNode
	for _, node := range doc.Nodes {
		if fc, ok := node.(FixtureCallNode); ok {
			calls = append(calls, fc)
		}
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 fixture call, got %d", len(calls))
	}
	if calls[0].Fixture != "check-user" {
		t.Fatalf("unexpected fixture %q", calls[0].Fixture)
	}
	if calls[0].FixtureParams["field"] != "plan" || calls[0].FixtureParams["expected"] != "STANDARD" {
		t.Fatalf("unexpected params %#v", calls[0].FixtureParams)
	}
	if calls[0].ID == nil || calls[0].ID.Ordinal != 1 {
		t.Fatalf("unexpected ID %#v", calls[0].ID)
	}
}

func TestParseDocumentRejectsFixtureWithoutParamsAndNoTable(t *testing.T) {
	_, err := ParseDocument("bad.spec.md", strings.Join([]string{
		"# Bad",
		"",
		"> fixture:board-exists",
		"",
		"not a table",
		"",
	}, "\n"), nil)

	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "must be followed by a table") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestParseDocumentParsesHookDirectives(t *testing.T) {
	doc, err := ParseDocument("test.spec.md", strings.Join([]string{
		"# Test",
		"",
		"## Group",
		"",
		"> setup:each",
		"```run:api",
		"login u0",
		"```",
		"",
		"> teardown:each",
		"```run:api",
		"reset u0",
		"```",
		"",
		"### Scenario A",
		"",
		"Prose here.",
		"",
	}, "\n"), nil)

	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	var hooks []HookNode
	for _, node := range doc.Nodes {
		if h, ok := node.(HookNode); ok {
			hooks = append(hooks, h)
		}
	}
	if len(hooks) != 2 {
		t.Fatalf("expected 2 hooks, got %d", len(hooks))
	}
	if hooks[0].Hook != HookSetup || !hooks[0].Each {
		t.Fatalf("expected setup:each, got %v each=%v", hooks[0].Hook, hooks[0].Each)
	}
	if hooks[0].Block.Kind != BlockKindRun || hooks[0].Block.Target != "api" {
		t.Fatalf("unexpected block %#v", hooks[0].Block)
	}
	if hooks[0].Source != "login u0" {
		t.Fatalf("unexpected source %q", hooks[0].Source)
	}
	if hooks[1].Hook != HookTeardown || !hooks[1].Each {
		t.Fatalf("expected teardown:each, got %v each=%v", hooks[1].Hook, hooks[1].Each)
	}
}

func TestParseDocumentParsesNonEachHook(t *testing.T) {
	doc, err := ParseDocument("test.spec.md", strings.Join([]string{
		"# Test",
		"",
		"> setup",
		"```run:shell",
		"init-db",
		"```",
		"",
	}, "\n"), nil)

	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	var hooks []HookNode
	for _, node := range doc.Nodes {
		if h, ok := node.(HookNode); ok {
			hooks = append(hooks, h)
		}
	}
	if len(hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks))
	}
	if hooks[0].Hook != HookSetup || hooks[0].Each {
		t.Fatalf("expected setup (non-each), got %v each=%v", hooks[0].Hook, hooks[0].Each)
	}
}

func TestParseDocumentRejectsHookWithoutCodeBlock(t *testing.T) {
	_, err := ParseDocument("bad.spec.md", strings.Join([]string{
		"# Bad",
		"",
		"> setup:each",
		"",
		"Just prose.",
		"",
	}, "\n"), nil)

	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "must be followed by a code block") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestParseDocumentRejectsInvalidAlloyReferenceDirective(t *testing.T) {
	_, err := ParseDocument("bad.spec.md", strings.Join([]string{
		"# Bad",
		"",
		"> alloy:ref(board#cardExists)",
		"",
	}, "\n"), nil)

	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "invalid alloy reference directive") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestParseDocumentWarnsOnUnknownBlockPrefix(t *testing.T) {
	doc, err := ParseDocument("test.spec.md", strings.Join([]string{
		"# Test",
		"",
		"```verify:shell",
		"echo hello",
		"```",
		"",
		"```test:webapp",
		"some test",
		"```",
		"",
	}, "\n"), nil)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(doc.Warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d: %v", len(doc.Warnings), doc.Warnings)
	}
	if !strings.Contains(doc.Warnings[0], "verify") {
		t.Fatalf("expected warning about verify prefix, got %q", doc.Warnings[0])
	}
	if !strings.Contains(doc.Warnings[1], "test") {
		t.Fatalf("expected warning about test prefix, got %q", doc.Warnings[1])
	}
}

func TestParseDocumentSuppressesWarningForIgnoredPrefix(t *testing.T) {
	doc, err := ParseDocument("test.spec.md", strings.Join([]string{
		"# Test",
		"",
		"```verify:shell",
		"echo hello",
		"```",
		"",
		"```test:webapp",
		"some test",
		"```",
		"",
	}, "\n"), []string{"verify", "test"})
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(doc.Warnings) != 0 {
		t.Fatalf("expected 0 warnings with ignore list, got %d: %v", len(doc.Warnings), doc.Warnings)
	}
}

func TestParseDocumentNoWarningForPlainInfoStrings(t *testing.T) {
	doc, err := ParseDocument("test.spec.md", strings.Join([]string{
		"# Test",
		"",
		"```json",
		`{"key": "value"}`,
		"```",
		"",
		"```go",
		"package main",
		"```",
		"",
	}, "\n"), nil)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(doc.Warnings) != 0 {
		t.Fatalf("expected 0 warnings for plain blocks, got %d: %v", len(doc.Warnings), doc.Warnings)
	}
}
