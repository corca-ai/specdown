package engine

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/corca-ai/specdown/internal/specdown/adapterhost"
	"github.com/corca-ai/specdown/internal/specdown/adapterprotocol"
	"github.com/corca-ai/specdown/internal/specdown/config"
	"github.com/corca-ai/specdown/internal/specdown/core"
)

func TestRunSupportsBoardAndCardLifecycleFixtures(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "pocket-board.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := strings.Join([]string{
		"# Pocket Board",
		"",
		"## Board Creation",
		"",
		"```run:board -> $boardName",
		"create-board",
		"```",
		"",
		"### A created board must exist immediately",
		"",
		"```run:board",
		"board \"${boardName}\" should exist",
		"```",
		"",
		"### A board that was not created must not exist",
		"",
		"```run:board",
		"board \"${boardName}-archive\" should not exist",
		"```",
		"",
		"### Card Lifecycle",
		"",
		"```run:board -> $cardId",
		"create-card \"${boardName}\" \"write spec\"",
		"```",
		"",
		"#### A created card must belong to a board",
		"",
		"> fixture:card-exists",
		"| board | card | exists |",
		"| --- | --- | --- |",
		"| ${boardName} | ${cardId} | yes |",
		"",
		"#### New cards must be in todo",
		"",
		"> fixture:card-column",
		"| board | card | column |",
		"| --- | --- | --- |",
		"| ${boardName} | ${cardId} | todo |",
		"",
		"#### Cards can be moved to another column",
		"",
		"```run:board",
		"move-card \"${boardName}\" \"${cardId}\" doing",
		"```",
		"",
		"##### A moved card must be in doing",
		"",
		"> fixture:card-column",
		"| board | card | column |",
		"| --- | --- | --- |",
		"| ${boardName} | ${cardId} | doing |",
		"",
	}, "\n")
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	writeEntryFile(t, root, specPath)

	report, err := Run(root, helperAdapterConfig(), RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if report.Summary.SpecsPassed != 1 || report.Summary.CasesFailed != 0 {
		t.Fatalf("unexpected summary %+v", report.Summary)
	}
	if report.Summary.CasesPassed != 8 {
		t.Fatalf("unexpected summary %+v", report.Summary)
	}
	if got := report.Results[0].Cases[0].Bindings; len(got) != 1 || got[0].Name != "boardName" || got[0].Value != "board-1" {
		t.Fatalf("unexpected board binding %#v", got)
	}
	if got := report.Results[0].Cases[3].Bindings; len(got) != 1 || got[0].Name != "cardId" || got[0].Value != "card-1" {
		t.Fatalf("unexpected card binding %#v", got)
	}
	if got := report.Results[0].Cases[4].RenderedCells; len(got) != 3 || got[0] != "board-1" || got[1] != "card-1" || got[2] != "yes" {
		t.Fatalf("unexpected card exists row %#v", got)
	}
	if got := report.Results[0].Cases[7].RenderedCells; len(got) != 3 || got[2] != "doing" {
		t.Fatalf("unexpected moved card row %#v", got)
	}
}

func TestRunFailsWhenCardColumnFixtureMismatches(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "pocket-board.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := strings.Join([]string{
		"# Pocket Board",
		"",
		"## Board Creation",
		"",
		"```run:board -> $boardName",
		"create-board",
		"```",
		"",
		"### Card Lifecycle",
		"",
		"```run:board -> $cardId",
		"create-card \"${boardName}\" \"write spec\"",
		"```",
		"",
		"#### Card Column Check",
		"",
		"> fixture:card-column",
		"| board | card | column |",
		"| --- | --- | --- |",
		"| ${boardName} | ${cardId} | doing |",
		"",
	}, "\n")
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	writeEntryFile(t, root, specPath)

	report, err := Run(root, helperAdapterConfig(), RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if report.Summary.CasesFailed != 1 {
		t.Fatalf("unexpected summary %+v", report.Summary)
	}
	failedCase := report.Results[0].Cases[2]
	if failedCase.Message != "column mismatch for card \"card-1\" in board \"board-1\"" {
		t.Fatalf("unexpected failure message %q", failedCase.Message)
	}
	if failedCase.Expected != "doing" {
		t.Fatalf("unexpected expected %q", failedCase.Expected)
	}
	if failedCase.Actual != "todo" {
		t.Fatalf("unexpected actual %q", failedCase.Actual)
	}
}

func TestRunFailsWhenRuntimeBindingWasNotProducedForFixtureRow(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "pocket-board.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := strings.Join([]string{
		"# Pocket Board",
		"",
		"## Board Creation",
		"",
		"```run:board -> $boardName",
		"create-board board-1",
		"```",
		"",
		"### Board Existence Rules",
		"",
		"> fixture:board-exists",
		"| board | exists |",
		"| --- | --- |",
		"| ${boardName} | yes |",
		"",
	}, "\n")
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	writeEntryFile(t, root, specPath)

	report, err := Run(root, helperNoBindingConfig(), RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if report.Summary.CasesFailed != 1 {
		t.Fatalf("unexpected summary %+v", report.Summary)
	}
	if got := report.Results[0].Cases[1].Message; got != `missing runtime binding for "boardName"` {
		t.Fatalf("unexpected message %q", got)
	}
}

func TestRunFailsWhenNoAdapterSupportsFixture(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "pocket-board.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := strings.Join([]string{
		"# Pocket Board",
		"",
		"## Table Check",
		"",
		"> fixture:unknown",
		"| board | exists |",
		"| --- | --- |",
		"| demo | yes |",
		"",
	}, "\n")
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	writeEntryFile(t, root, specPath)

	_, err := Run(root, helperAdapterConfig(), RunOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no adapter supports fixture") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestRunTracksAlloyChecksAlongsideAdapterCases(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "pocket-board.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := strings.Join([]string{
		"# Pocket Board",
		"",
		"## Board Creation",
		"",
		"```run:board -> $boardName",
		"create-board",
		"```",
		"",
		"## Formal Rules",
		"",
		"```alloy:model(board)",
		"module board",
		"",
		"sig Card {}",
		"assert cardShape { some Card }",
		"```",
		"",
		"> alloy:ref(board#cardShape, scope=5)",
		"",
	}, "\n")
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	writeEntryFile(t, root, specPath)

	title, docs, err := core.DiscoverFromEntry(root, "specs/index.spec.md", nil)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	report, err := runWithDocs(title, docs, config.Config{
		Entry:    "specs/index.spec.md",
		Adapters: helperAdapterConfig().Adapters,
	}, adapterhost.Host{BaseDir: root}, fakeAlloyRunner{
		results: map[string]core.AlloyCheckResult{
			"specs/pocket-board.spec.md|Pocket Board|Formal Rules|2": {
				ID: core.SpecID{
					File:        "specs/pocket-board.spec.md",
					HeadingPath: []string{"Pocket Board", "Formal Rules"},
					Ordinal:     2,
				},
				Model:     "board",
				Assertion: "cardShape",
				Scope:     "5",
				Label:     "alloy:ref(board#cardShape, scope=5) @ Formal Rules",
				Status:    core.StatusPassed,
			},
		},
	}, RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if report.Summary.SpecsPassed != 1 || report.Summary.CasesPassed != 1 || report.Summary.AlloyChecksPassed != 1 {
		t.Fatalf("unexpected summary %+v", report.Summary)
	}
	if got := report.Results[0].AlloyChecks[0].Label; got != "alloy:ref(board#cardShape, scope=5) @ Formal Rules" {
		t.Fatalf("unexpected alloy label %q", got)
	}
}

func TestFilterPlanKeepsMatchingCases(t *testing.T) {
	plan := core.Plan{
		Documents: []core.DocumentPlan{
			{
				Document: core.Document{RelativeTo: "a.spec.md"},
				Cases: []core.CaseSpec{
					{ID: core.SpecID{HeadingPath: []string{"Board", "Create"}}},
					{ID: core.SpecID{HeadingPath: []string{"Board", "Delete"}}},
				},
			},
		},
	}
	filtered := filterPlan(plan, "Create")
	if len(filtered.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(filtered.Documents))
	}
	if len(filtered.Documents[0].Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(filtered.Documents[0].Cases))
	}
	if filtered.Documents[0].Cases[0].ID.HeadingPath[1] != "Create" {
		t.Fatalf("unexpected case %v", filtered.Documents[0].Cases[0].ID.HeadingPath)
	}
}

func TestFilterPlanDropsDocumentsWithNoCases(t *testing.T) {
	plan := core.Plan{
		Documents: []core.DocumentPlan{
			{
				Document: core.Document{RelativeTo: "a.spec.md"},
				Cases:    []core.CaseSpec{{ID: core.SpecID{HeadingPath: []string{"X"}}}},
			},
		},
	}
	filtered := filterPlan(plan, "nonexistent-pattern")
	if len(filtered.Documents) != 0 {
		t.Fatalf("expected 0 documents, got %d", len(filtered.Documents))
	}
}

func TestDryRunReportHasZeroStatuses(t *testing.T) {
	plan := core.Plan{
		Documents: []core.DocumentPlan{
			{
				Document: core.Document{RelativeTo: "a.spec.md"},
				Cases: []core.CaseSpec{
					{ID: core.SpecID{HeadingPath: []string{"A"}}, Kind: core.CaseKindCode, Block: core.BlockSpec{Raw: "run:shell", Kind: core.BlockKindRun, Target: "shell"}},
				},
				AlloyChecks: []core.AlloyCheckSpec{
					{ID: core.SpecID{HeadingPath: []string{"A"}}, Model: "m", Assertion: "a", Scope: "5"},
				},
			},
		},
	}
	report := dryRunReport(plan)
	if report.Summary.CasesTotal != 1 || report.Summary.AlloyChecksTotal != 1 {
		t.Fatalf("unexpected totals %+v", report.Summary)
	}
	if report.Summary.CasesPassed != 0 || report.Summary.CasesFailed != 0 {
		t.Fatalf("dry-run should have zero pass/fail %+v", report.Summary)
	}
	if report.Results[0].Cases[0].Status != "" {
		t.Fatalf("dry-run case should have empty status, got %q", report.Results[0].Cases[0].Status)
	}
}

func TestRenderTemplateEscapesBackslashDollar(t *testing.T) {
	bindings := []core.Binding{{Name: "x", Value: "42"}}
	got, err := renderTemplate(`\${x} and ${x}`, bindings)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got != "${x} and 42" {
		t.Fatalf("expected '${x} and 42', got %q", got)
	}
}

func TestRenderTemplateReturnsErrorForUnresolved(t *testing.T) {
	_, err := renderTemplate("${missing}", nil)
	if err == nil {
		t.Fatal("expected error for unresolved variable")
	}
}

func TestBindingReachableAncestorAndSibling(t *testing.T) {
	// Ancestor: binding at ["A"] visible from ["A", "B"]
	if !bindingReachable([]string{"A"}, []string{"A", "B"}) {
		t.Fatal("ancestor should be reachable")
	}
	// Sibling: binding at ["A"] visible from ["B"] (same depth, same parent = root)
	if !bindingReachable([]string{"A"}, []string{"B"}) {
		t.Fatal("sibling should be reachable")
	}
	// Child binding NOT visible from parent
	if bindingReachable([]string{"A", "B"}, []string{"A"}) {
		t.Fatal("child should not be reachable from parent")
	}
	// Unrelated deeper path
	if bindingReachable([]string{"A", "B"}, []string{"C", "D"}) {
		t.Fatal("unrelated deep path should not be reachable")
	}
}

func TestRunWithFrontmatterTimeout(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "timeout.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Frontmatter with 100ms timeout — the helper adapter is fast enough
	source := "---\ntimeout: 100\n---\n\n# T\n\n## Run\n\n```run:board -> $b\ncreate-board\n```\n"
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	writeEntryFile(t, root, specPath)
	report, err := Run(root, helperAdapterConfig(), RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if report.Summary.CasesPassed != 1 {
		t.Fatalf("expected 1 passed case, got %+v", report.Summary)
	}
}

func TestRunDryRunSkipsExecution(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "dry.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := "# Dry\n\n## Test\n\n```run:board -> $b\ncreate-board\n```\n"
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	writeEntryFile(t, root, specPath)
	// DryRun should not launch any adapter — even with no adapter config
	report, err := Run(root, config.Config{Entry: "specs/index.spec.md"}, RunOptions{DryRun: true})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if report.Summary.CasesTotal != 1 {
		t.Fatalf("expected 1 case total, got %d", report.Summary.CasesTotal)
	}
	if report.Summary.CasesPassed != 0 {
		t.Fatalf("dry-run should not mark cases as passed")
	}
}

func TestRunWithFilterOnlyRunsMatchingCases(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "filter.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := strings.Join([]string{
		"# Filter",
		"",
		"## Alpha",
		"",
		"```run:board -> $a",
		"create-board",
		"```",
		"",
		"## Beta",
		"",
		"```run:board -> $b",
		"create-board",
		"```",
		"",
	}, "\n")
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	writeEntryFile(t, root, specPath)
	report, err := Run(root, helperAdapterConfig(), RunOptions{Filter: "Alpha"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if report.Summary.CasesTotal != 1 {
		t.Fatalf("expected 1 case, got %d", report.Summary.CasesTotal)
	}
	if report.Summary.CasesPassed != 1 {
		t.Fatalf("expected 1 passed, got %+v", report.Summary)
	}
}

func TestRunExecutesSetupEachHooksAtSectionBoundaries(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "hook.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := strings.Join([]string{
		"# Hook Test",
		"",
		"## Group",
		"",
		"> setup:each",
		"```run:board",
		"create-board",
		"```",
		"",
		"### Scenario A",
		"",
		"```run:board -> $a",
		"create-board",
		"```",
		"",
		"### Scenario B",
		"",
		"```run:board -> $b",
		"create-board",
		"```",
		"",
	}, "\n")
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	writeEntryFile(t, root, specPath)

	report, err := Run(root, helperAdapterConfig(), RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// 2 cases (Scenario A and B), setup:each hook runs before each but isn't counted as a case
	if report.Summary.CasesPassed != 2 {
		t.Fatalf("expected 2 passed cases, got %+v", report.Summary)
	}
	if report.Summary.CasesFailed != 0 {
		t.Fatalf("expected 0 failed, got %+v", report.Summary)
	}
}

func TestRunExecutesSetupOnceHook(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "hook-once.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := strings.Join([]string{
		"# Hook Once",
		"",
		"> setup",
		"```run:board",
		"create-board",
		"```",
		"",
		"## Section A",
		"",
		"```run:board -> $a",
		"create-board",
		"```",
		"",
		"## Section B",
		"",
		"```run:board -> $b",
		"create-board",
		"```",
		"",
	}, "\n")
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	writeEntryFile(t, root, specPath)

	report, err := Run(root, helperAdapterConfig(), RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if report.Summary.CasesPassed != 2 {
		t.Fatalf("expected 2 passed cases, got %+v", report.Summary)
	}
}

func TestRunFixtureCallWithParams(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "fixture-call.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := strings.Join([]string{
		"# Fixture Call",
		"",
		"## Setup",
		"",
		"```run:board -> $boardName",
		"create-board",
		"```",
		"",
		"## Check",
		"",
		"> fixture:board-exists(board=board-1, exists=yes)",
		"",
	}, "\n")
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	writeEntryFile(t, root, specPath)

	report, err := Run(root, helperAdapterConfig(), RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if report.Summary.CasesPassed != 2 {
		t.Fatalf("expected 2 passed cases, got %+v", report.Summary)
	}
}

func writeEntryFile(t *testing.T, root string, specFiles ...string) {
	t.Helper()
	specsDir := filepath.Join(root, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}
	var lines []string
	lines = append(lines, "# Test Specs\n")
	for _, f := range specFiles {
		name := filepath.Base(f)
		lines = append(lines, "- ["+name+"]("+name+")")
	}
	entry := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(specsDir, "index.spec.md"), []byte(entry), 0o644); err != nil {
		t.Fatalf("write entry: %v", err)
	}
}

func helperAdapterConfig() config.Config {
	executable, err := os.Executable()
	if err != nil {
		panic(err)
	}

	return config.Config{
		Entry: "specs/index.spec.md",
		Adapters: []config.AdapterConfig{
			{
				Name:     "helper-board",
				Command:  []string{executable, "-test.run=TestHelperAdapterProcess", "--", "board"},
				Blocks:   []string{"run:board"},
				Fixtures: []string{"board-exists", "card-exists", "card-column"},
			},
		},
	}
}

func helperNoBindingConfig() config.Config {
	executable, err := os.Executable()
	if err != nil {
		panic(err)
	}

	return config.Config{
		Entry: "specs/index.spec.md",
		Adapters: []config.AdapterConfig{
			{
				Name:     "helper-board",
				Command:  []string{executable, "-test.run=TestHelperAdapterProcess", "--", "board-no-bindings"},
				Blocks:   []string{"run:board"},
				Fixtures: []string{"board-exists", "card-exists", "card-column"},
			},
		},
	}
}

func TestHelperAdapterProcess(t *testing.T) {
	if len(os.Args) < 2 {
		return
	}

	mode := os.Args[len(os.Args)-1]
	if mode != "board" && mode != "board-no-bindings" {
		return
	}

	state := helperState{
		boards:      make(map[string]*helperBoard),
		nextBoardID: 1,
		nextCardID:  1,
		emitBinding: mode == "board",
	}

	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var request adapterprotocol.Request
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			os.Exit(2)
		}

		switch request.Type {
		case "setup", "teardown":
			// lifecycle hooks — no-op for helper adapter
		case "runCase":
			if request.Case == nil {
				os.Exit(3)
			}
			runHelperCase(encoder, &state, request.ID, *request.Case)
		default:
			os.Exit(4)
		}
	}
	os.Exit(0)
}

type helperState struct {
	boards      map[string]*helperBoard
	nextBoardID int
	nextCardID  int
	emitBinding bool
}

type helperBoard struct {
	cards map[string]*helperCard
}

type helperCard struct {
	column string
}

func runHelperCase(encoder *json.Encoder, state *helperState, seqID int, item adapterprotocol.Case) {
	bindings, err := executeHelperCase(state, item)
	if err != nil {
		resp := adapterprotocol.Response{
			ID:      seqID,
			Type:    "failed",
			Message: err.Error(),
		}
		var hf *helperFailure
		if errors.As(err, &hf) {
			resp.Message = hf.message
			resp.Expected = hf.expected
			resp.Actual = hf.actual
			resp.Label = hf.label
		}
		_ = encoder.Encode(resp)
		return
	}

	_ = encoder.Encode(adapterprotocol.Response{
		ID:       seqID,
		Type:     "passed",
		Bindings: bindings,
	})
}

func executeHelperCase(state *helperState, item adapterprotocol.Case) ([]adapterprotocol.Binding, error) {
	if item.Kind == "tableRow" {
		return executeHelperFixtureRow(state, item)
	}

	parts, err := parseHelperCommand(item.Source)
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		return nil, &helperError{message: "empty command"}
	}

	switch parts[0] {
	case "create-board":
		return executeHelperCreateBoard(state, item, parts)
	case "create-card":
		return executeHelperCreateCard(state, item, parts)
	case "move-card":
		return executeHelperMoveCard(state, parts)
	case "board":
		return executeHelperVerifyBoard(state, item.Source)
	default:
		return nil, &helperError{message: "unsupported case " + item.Block}
	}
}

func executeHelperCreateBoard(state *helperState, item adapterprotocol.Case, parts []string) ([]adapterprotocol.Binding, error) {
	var name string
	switch len(parts) {
	case 1:
		name = "board-" + strconv.Itoa(state.nextBoardID)
		state.nextBoardID++
	case 2:
		name = parts[1]
	default:
		return nil, &helperError{message: "unsupported board command " + strconvQuote(item.Source)}
	}
	if _, exists := state.boards[name]; exists {
		return nil, &helperError{message: "board " + strconvQuote(name) + " already exists"}
	}
	state.boards[name] = &helperBoard{cards: make(map[string]*helperCard)}
	if !state.emitBinding || len(item.CaptureNames) == 0 {
		return nil, nil
	}
	return []adapterprotocol.Binding{{Name: item.CaptureNames[0], Value: name}}, nil
}

func executeHelperCreateCard(state *helperState, item adapterprotocol.Case, parts []string) ([]adapterprotocol.Binding, error) {
	if len(parts) != 3 {
		return nil, &helperError{message: "unsupported board command " + strconvQuote(item.Source)}
	}
	board, err := helperBoardFor(state, parts[1])
	if err != nil {
		return nil, err
	}
	cardID := "card-" + strconv.Itoa(state.nextCardID)
	state.nextCardID++
	board.cards[cardID] = &helperCard{column: "todo"}
	if !state.emitBinding || len(item.CaptureNames) == 0 {
		return nil, nil
	}
	return []adapterprotocol.Binding{{Name: item.CaptureNames[0], Value: cardID}}, nil
}

func executeHelperMoveCard(state *helperState, parts []string) ([]adapterprotocol.Binding, error) {
	if len(parts) != 4 {
		return nil, &helperError{message: "unsupported board command"}
	}
	board, err := helperBoardFor(state, parts[1])
	if err != nil {
		return nil, err
	}
	card := board.cards[parts[2]]
	if card == nil {
		return nil, helperCardExistsFailure(parts[1], parts[2], true)
	}
	card.column = parts[3]
	return nil, nil
}

func executeHelperVerifyBoard(state *helperState, source string) ([]adapterprotocol.Binding, error) {
	name, shouldExist, err := parseHelperVerifySource(source)
	if err != nil {
		return nil, err
	}
	_, exists := state.boards[name]
	if shouldExist == exists {
		return nil, nil
	}
	return nil, helperBoardExistsFailure(name, shouldExist)
}

func executeHelperFixtureRow(state *helperState, item adapterprotocol.Case) ([]adapterprotocol.Binding, error) {
	if len(item.Columns) != len(item.Cells) {
		return nil, &helperError{message: "fixture row shape mismatch"}
	}

	values := make(map[string]string, len(item.Columns)+len(item.FixtureParams))
	for index, column := range item.Columns {
		values[column] = item.Cells[index]
	}
	// Fixture call params override column values
	for k, v := range item.FixtureParams {
		values[k] = v
	}

	switch item.Fixture {
	case "board-exists":
		return helperFixtureBoardExists(state, values)
	case "card-exists":
		return helperFixtureCardExists(state, values)
	case "card-column":
		return helperFixtureCardColumn(state, values)
	default:
		return nil, &helperError{message: "unsupported fixture " + strconvQuote(item.Fixture)}
	}
}

func helperFixtureBoardExists(state *helperState, values map[string]string) ([]adapterprotocol.Binding, error) {
	name := values["board"]
	shouldExist := parseHelperExists(values["exists"])
	_, exists := state.boards[name]
	if shouldExist == exists {
		return nil, nil
	}
	return nil, helperBoardExistsFailure(name, shouldExist)
}

func helperFixtureCardExists(state *helperState, values map[string]string) ([]adapterprotocol.Binding, error) {
	boardName := values["board"]
	cardName := values["card"]
	shouldExist := parseHelperExists(values["exists"])
	board, err := helperBoardFor(state, boardName)
	if err != nil {
		return nil, err
	}
	_, exists := board.cards[cardName]
	if shouldExist == exists {
		return nil, nil
	}
	return nil, helperCardExistsFailure(boardName, cardName, shouldExist)
}

func helperFixtureCardColumn(state *helperState, values map[string]string) ([]adapterprotocol.Binding, error) {
	boardName := values["board"]
	cardName := values["card"]
	expectedColumn := values["column"]
	board, err := helperBoardFor(state, boardName)
	if err != nil {
		return nil, err
	}
	card := board.cards[cardName]
	if card == nil {
		return nil, helperCardExistsFailure(boardName, cardName, true)
	}
	if card.column == expectedColumn {
		return nil, nil
	}
	return nil, &helperFailure{
		message:  "column mismatch for card " + strconvQuote(cardName) + " in board " + strconvQuote(boardName),
		expected: expectedColumn,
		actual:   card.column,
	}
}

type helperError struct {
	message string
}

func (e *helperError) Error() string {
	return e.message
}

type helperFailure struct {
	message  string
	expected string
	actual   string
	label    string
}

func (e *helperFailure) Error() string {
	return e.message
}

func helperBoardExistsFailure(name string, shouldExist bool) *helperError {
	if shouldExist {
		return &helperError{
			message: "expected board " + strconvQuote(name) + " to exist; actual boards: [\"board-1\"]",
		}
	}
	return &helperError{
		message: "expected board " + strconvQuote(name) + " not to exist; actual boards: [\"board-1\"]",
	}
}

func helperCardExistsFailure(boardName string, cardName string, shouldExist bool) *helperError {
	if shouldExist {
		return &helperError{
			message: "expected card " + strconvQuote(cardName) + " to exist in board " + strconvQuote(boardName) + "; actual cards: [\"card-1\"]",
		}
	}
	return &helperError{
		message: "expected card " + strconvQuote(cardName) + " not to exist in board " + strconvQuote(boardName) + "; actual cards: [\"card-1\"]",
	}
}

func helperBoardFor(state *helperState, boardName string) (*helperBoard, error) {
	board := state.boards[boardName]
	if board == nil {
		return nil, helperBoardExistsFailure(boardName, true)
	}
	return board, nil
}

func strconvQuote(value string) string {
	return strconv.Quote(value)
}

func parseHelperCommand(source string) ([]string, error) {
	var (
		parts   []string
		current strings.Builder
		inQuote bool
	)
	for _, r := range source {
		switch r {
		case '"':
			inQuote = !inQuote
		case ' ', '\t':
			if inQuote {
				current.WriteRune(r)
				continue
			}
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if inQuote {
		return nil, &helperError{message: "invalid command source " + strconvQuote(source)}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts, nil
}

func parseHelperVerifySource(source string) (string, bool, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(source, "board"))
	if strings.HasSuffix(trimmed, "should exist") {
		namePart := strings.TrimSpace(strings.TrimSuffix(trimmed, "should exist"))
		name, err := parseHelperCommandArg(namePart)
		return name, true, err
	}
	if strings.HasSuffix(trimmed, "should not exist") {
		namePart := strings.TrimSpace(strings.TrimSuffix(trimmed, "should not exist"))
		name, err := parseHelperCommandArg(namePart)
		return name, false, err
	}
	return "", false, &helperError{message: "invalid verify source " + strconvQuote(source)}
}

func parseHelperCommandArg(input string) (string, error) {
	input = strings.TrimSpace(input)
	value, err := strconv.Unquote(input)
	if err == nil {
		return value, nil
	}
	if strings.ContainsAny(input, " \t") {
		return "", &helperError{message: "invalid command source " + strconvQuote(input)}
	}
	return input, nil
}

func parseHelperExists(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes", "true":
		return true
	default:
		return false
	}
}

type fakeAlloyRunner struct {
	results map[string]core.AlloyCheckResult
}

func (f fakeAlloyRunner) RunDocument(plan core.DocumentPlan) ([]core.AlloyCheckResult, error) {
	results := make([]core.AlloyCheckResult, 0, len(plan.AlloyChecks))
	for _, check := range plan.AlloyChecks {
		result, ok := f.results[check.ID.Key()]
		if !ok {
			result = core.AlloyCheckResult{
				ID:        check.ID,
				Model:     check.Model,
				Assertion: check.Assertion,
				Scope:     check.Scope,
				Label:     "alloy:ref(" + check.Model + "#" + check.Assertion + ", scope=" + check.Scope + ")",
				Status:    core.StatusPassed,
			}
		}
		results = append(results, result)
	}
	return results, nil
}
