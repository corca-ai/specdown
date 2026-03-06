package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunExecutesBoardBlock(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "pocket-board.spec.md")

	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := "# Pocket Board\n\n## First Executable Check\n\n```run:board\ncreate-board \"demo\"\n```\n"
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	report, err := Run(filepath.Join(root, "specs"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if report.Summary.SpecsPassed != 1 {
		t.Fatalf("unexpected spec summary %+v", report.Summary)
	}
	if report.Summary.CasesPassed != 1 {
		t.Fatalf("unexpected case summary %+v", report.Summary)
	}

	if len(report.Results) != 1 || len(report.Results[0].Cases) != 1 {
		t.Fatalf("unexpected results %#v", report.Results)
	}

	caseResult := report.Results[0].Cases[0]
	if caseResult.Status != StatusPassed {
		t.Fatalf("unexpected case status %q", caseResult.Status)
	}
	if len(caseResult.Events) != 2 {
		t.Fatalf("unexpected events %#v", caseResult.Events)
	}
	if caseResult.Events[0].Type != EventCaseStarted || caseResult.Events[1].Type != EventCasePassed {
		t.Fatalf("unexpected event types %#v", caseResult.Events)
	}
}

func TestRunFailsWhenBoardCommandFails(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "pocket-board.spec.md")

	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := "# Pocket Board\n\n## Broken Check\n\n```run:board\nmove-card demo doing\n```\n"
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	report, err := Run(filepath.Join(root, "specs"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if report.Summary.SpecsFailed != 1 || report.Summary.CasesFailed != 1 {
		t.Fatalf("unexpected summary %+v", report.Summary)
	}

	caseResult := report.Results[0].Cases[0]
	if caseResult.Status != StatusFailed {
		t.Fatalf("unexpected case status %q", caseResult.Status)
	}
	if caseResult.Message == "" {
		t.Fatal("expected failure message")
	}
	if caseResult.Events[len(caseResult.Events)-1].Type != EventCaseFailed {
		t.Fatalf("unexpected events %#v", caseResult.Events)
	}
}

func TestRunAggregatesPassAndFailCasesInOneDocument(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "pocket-board.spec.md")

	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := "# Pocket Board\n\n## Create Board\n\n```run:board\ncreate-board \"demo\"\n```\n\n## Duplicate Board\n\n```run:board\ncreate-board \"demo\"\n```\n"
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	report, err := Run(filepath.Join(root, "specs"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if report.Summary.SpecsFailed != 1 {
		t.Fatalf("unexpected spec summary %+v", report.Summary)
	}
	if report.Summary.CasesPassed != 1 || report.Summary.CasesFailed != 1 {
		t.Fatalf("unexpected case summary %+v", report.Summary)
	}
	if len(report.Results) != 1 || len(report.Results[0].Cases) != 2 {
		t.Fatalf("unexpected results %#v", report.Results)
	}
	if report.Results[0].Cases[0].Status != StatusPassed {
		t.Fatalf("unexpected first case status %q", report.Results[0].Cases[0].Status)
	}
	if report.Results[0].Cases[1].Status != StatusFailed {
		t.Fatalf("unexpected second case status %q", report.Results[0].Cases[1].Status)
	}
}
