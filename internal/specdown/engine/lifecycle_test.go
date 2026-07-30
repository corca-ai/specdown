package engine

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/corca-ai/specdown/internal/specdown/config"
	"github.com/corca-ai/specdown/internal/specdown/core"
)

func TestRunFailedSetupHookFailsDocumentAndSkipsAffectedCase(t *testing.T) {
	source := strings.Join([]string{
		"# Lifecycle",
		"",
		"> setup",
		"```run:board",
		"fail-command",
		"```",
		"",
		"> teardown",
		"```run:board",
		"create-board cleanup",
		"```",
		"",
		"## Affected",
		"",
		"```run:board",
		"create-board should-not-run",
		"```",
		"",
	}, "\n")
	root := writeSpecFile(t, "lifecycle.spec.md", source)

	report, err := Run(root, helperAdapterConfig(), noopModelRunner{}, RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := report.Results[1].Status; got != core.StatusFailed {
		t.Fatalf("document status = %q, want %q", got, core.StatusFailed)
	}
	if got := report.Summary.CasesTotal; got != 1 {
		t.Fatalf("case total = %d, want 1", got)
	}
	if got := report.Summary.CasesSkipped; got != 1 {
		t.Fatalf("skipped cases = %d, want 1", got)
	}
	if got := report.Results[len(report.Results)-1].Cases[0].Status; got != core.StatusSkipped {
		t.Fatalf("affected case status = %q, want skipped", got)
	}
	result := report.Results[len(report.Results)-1]
	if len(result.LifecycleEvents) != 2 {
		t.Fatalf("lifecycle events = %+v, want failed setup and passed teardown", result.LifecycleEvents)
	}
	if result.LifecycleEvents[0].Phase != core.HookSetup || result.LifecycleEvents[0].Status != core.StatusFailed {
		t.Fatalf("setup event = %+v, want failed setup", result.LifecycleEvents[0])
	}
	if result.LifecycleEvents[1].Phase != core.HookTeardown || result.LifecycleEvents[1].Status != core.StatusPassed {
		t.Fatalf("teardown event = %+v, want passed teardown", result.LifecycleEvents[1])
	}
	if report.Summary.LifecycleFailed != 1 || report.Summary.LifecyclePassed != 1 {
		t.Fatalf("lifecycle summary = %+v, want one passed and one failed", report.Summary)
	}
}

func TestRunFailedTeardownHookFailsDocument(t *testing.T) {
	source := strings.Join([]string{
		"# Lifecycle",
		"",
		"> teardown",
		"```run:board",
		"fail-command",
		"```",
		"",
		"## Case",
		"",
		"```run:board",
		"create-board executed",
		"```",
		"",
	}, "\n")
	root := writeSpecFile(t, "teardown-failure.spec.md", source)

	report, err := Run(root, helperAdapterConfig(), noopModelRunner{}, RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	result := report.Results[len(report.Results)-1]
	if result.Status != core.StatusFailed {
		t.Fatalf("document status = %q, want failed", result.Status)
	}
	if len(result.Cases) != 1 || result.Cases[0].Status != core.StatusPassed {
		t.Fatalf("cases = %+v, want one passed case", result.Cases)
	}
	if len(result.LifecycleEvents) != 1 || result.LifecycleEvents[0].Status != core.StatusFailed {
		t.Fatalf("lifecycle events = %+v, want failed teardown", result.LifecycleEvents)
	}
}

func TestRunFailedSetupEachSkipsOnlyAffectedChildScopes(t *testing.T) {
	source := strings.Join([]string{
		"# Lifecycle",
		"",
		"## Group",
		"",
		"> setup:each",
		"```run:board",
		"fail-command",
		"```",
		"",
		"> teardown:each",
		"```run:board",
		`board "cleanup" should not exist`,
		"```",
		"",
		"### Child A",
		"",
		"```run:board",
		"create-board should-not-run-a",
		"```",
		"",
		"### Child B",
		"",
		"```run:board",
		"create-board should-not-run-b",
		"```",
		"",
		"## Outside",
		"",
		"```run:board",
		"create-board outside-ran",
		"```",
		"",
	}, "\n")
	root := writeSpecFile(t, "setup-each-failure.spec.md", source)

	report, err := Run(root, helperAdapterConfig(), noopModelRunner{}, RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	result := report.Results[len(report.Results)-1]
	if len(result.Cases) != 3 ||
		result.Cases[0].Status != core.StatusSkipped ||
		result.Cases[1].Status != core.StatusSkipped ||
		result.Cases[2].Status != core.StatusPassed ||
		result.Cases[2].ID.HeadingPath[len(result.Cases[2].ID.HeadingPath)-1] != "Outside" {
		t.Fatalf("cases = %+v, want two skipped children and one passed outside case", result.Cases)
	}
	if len(result.LifecycleEvents) != 4 {
		t.Fatalf("lifecycle events = %+v, want setup/teardown for each child", result.LifecycleEvents)
	}
	for i, event := range result.LifecycleEvents {
		wantPhase := core.HookSetup
		wantStatus := core.StatusFailed
		if i%2 == 1 {
			wantPhase = core.HookTeardown
			wantStatus = core.StatusPassed
		}
		if event.Phase != wantPhase || event.Status != wantStatus || !event.Each {
			t.Fatalf("event %d = %+v, want %s %s each", i, event, wantPhase, wantStatus)
		}
		wantChild := "Child A"
		if i >= 2 {
			wantChild = "Child B"
		}
		if got := event.HeadingPath[len(event.HeadingPath)-1]; got != wantChild {
			t.Fatalf("event %d target = %q, want %q", i, got, wantChild)
		}
	}
}

func TestRunExpectedFailureStillRunsTeardown(t *testing.T) {
	source := strings.Join([]string{
		"# Lifecycle",
		"",
		"> teardown",
		"```run:board",
		"create-board cleanup",
		"```",
		"",
		"## Expected failure",
		"",
		"```run:board !fail",
		"fail-command",
		"```",
		"",
	}, "\n")
	root := writeSpecFile(t, "expected-failure.spec.md", source)

	report, err := Run(root, helperAdapterConfig(), noopModelRunner{}, RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	result := report.Results[len(report.Results)-1]
	if result.Status != core.StatusPassed || len(result.Cases) != 1 || !result.Cases[0].ExpectFail {
		t.Fatalf("result = %+v, want a passing document with one expected failure", result)
	}
	if len(result.LifecycleEvents) != 1 || result.LifecycleEvents[0].Status != core.StatusPassed {
		t.Fatalf("lifecycle events = %+v, want passed teardown", result.LifecycleEvents)
	}
}

func TestRunUnexpectedFailureAtLimitStillRunsTeardown(t *testing.T) {
	source := strings.Join([]string{
		"# Lifecycle",
		"",
		"> teardown",
		"```run:board",
		"create-board cleanup",
		"```",
		"",
		"## Failure",
		"",
		"```run:board",
		"fail-command",
		"```",
		"",
	}, "\n")
	root := writeSpecFile(t, "failure-limit.spec.md", source)

	report, err := Run(root, helperAdapterConfig(), noopModelRunner{}, RunOptions{MaxFailures: 1})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	result := report.Results[len(report.Results)-1]
	if len(result.Cases) != 1 || result.Cases[0].Status != core.StatusFailed {
		t.Fatalf("cases = %+v, want one failed case", result.Cases)
	}
	if len(result.LifecycleEvents) != 1 ||
		result.LifecycleEvents[0].Phase != core.HookTeardown ||
		result.LifecycleEvents[0].Status != core.StatusPassed {
		t.Fatalf("lifecycle events = %+v, want passed teardown after failure limit", result.LifecycleEvents)
	}
}

func TestRunFailureLimitUnwindsScopeBeforeRemainingCase(t *testing.T) {
	source := strings.Join([]string{
		"# Lifecycle",
		"",
		"## Shared scope",
		"",
		"> teardown",
		"```run:board",
		"create-board cleanup",
		"```",
		"",
		"```run:board",
		"fail-command",
		"```",
		"",
		"```run:board",
		"create-board should-not-run",
		"```",
		"",
	}, "\n")
	root := writeSpecFile(t, "failure-limit-same-scope.spec.md", source)

	report, err := Run(root, helperAdapterConfig(), noopModelRunner{}, RunOptions{MaxFailures: 1})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	result := report.Results[len(report.Results)-1]
	if len(result.Cases) != 1 || result.Cases[0].Status != core.StatusFailed {
		t.Fatalf("cases = %+v, want only the first failed case", result.Cases)
	}
	if len(result.LifecycleEvents) != 1 ||
		result.LifecycleEvents[0].Phase != core.HookTeardown ||
		result.LifecycleEvents[0].Status != core.StatusPassed {
		t.Fatalf("lifecycle events = %+v, want teardown after terminal failure", result.LifecycleEvents)
	}
}

func TestRunParentSetupFailureSkipsNestedSetup(t *testing.T) {
	source := strings.Join([]string{
		"# Lifecycle",
		"",
		"## Blocked parent",
		"",
		"> setup",
		"```run:board",
		"fail-command",
		"```",
		"",
		"> teardown",
		"```run:board",
		"create-board parent-cleanup",
		"```",
		"",
		"### Nested child",
		"",
		"> setup",
		"```run:board",
		"create-board nested-setup-should-not-run",
		"```",
		"",
		"> teardown",
		"```run:board",
		"create-board nested-teardown-should-not-run",
		"```",
		"",
		"#### Case",
		"",
		"```run:board",
		"create-board nested-case-should-not-run",
		"```",
		"",
		"## Outside",
		"",
		"```run:board",
		`board "nested-setup-should-not-run" should not exist`,
		"```",
		"",
		"```run:board",
		`board "nested-teardown-should-not-run" should not exist`,
		"```",
		"",
		"```run:board",
		`board "parent-cleanup" should exist`,
		"```",
		"",
	}, "\n")
	root := writeSpecFile(t, "nested-setup.spec.md", source)

	report, err := Run(root, helperAdapterConfig(), noopModelRunner{}, RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	result := report.Results[len(report.Results)-1]
	if len(result.LifecycleEvents) != 2 {
		t.Fatalf("lifecycle events = %+v, want failed parent setup and parent teardown", result.LifecycleEvents)
	}
	if len(result.Cases) != 4 ||
		result.Cases[0].Status != core.StatusSkipped ||
		result.Cases[1].Status != core.StatusPassed ||
		result.Cases[2].Status != core.StatusPassed ||
		result.Cases[3].Status != core.StatusPassed {
		t.Fatalf("cases = %+v, want skipped nested case and passing outside checks", result.Cases)
	}
}

func TestRunCancellationDoesNotRepeatClosedEachTeardown(t *testing.T) {
	source := strings.Join([]string{
		"# Lifecycle",
		"",
		"> teardown:each",
		"```run:shell",
		"printf x >> cleanup.log",
		"```",
		"",
		"## Child A",
		"",
		"```run:shell",
		"true",
		"```",
		"",
		"## Child B",
		"",
		"```run:shell",
		"sleep 30",
		"```",
		"",
	}, "\n")
	root := writeSpecFile(t, "cancel-boundary.spec.md", source)
	ctx, cancel := context.WithCancel(context.Background())

	_, err := RunContext(ctx, root, config.Config{Entry: "specs/index.spec.md"}, noopModelRunner{}, RunOptions{
		Progress: func(event ProgressEvent) {
			if event.Kind == "case" && event.CaseNum == 1 {
				cancel()
			}
		},
	})
	if !contextCanceled(err) {
		t.Fatalf("run error = %v, want context cancellation", err)
	}
	body, readErr := os.ReadFile(filepath.Join(root, "cleanup.log"))
	if readErr != nil {
		t.Fatalf("read cleanup log: %v", readErr)
	}
	if got := string(body); got != "x" {
		t.Fatalf("cleanup log = %q, want one teardown execution", got)
	}
}

func TestRunCancellationDuringTeardownLetsCleanupFinish(t *testing.T) {
	source := strings.Join([]string{
		"# Lifecycle",
		"",
		"> teardown",
		"```run:shell",
		"printf started > teardown-started",
		"sleep 0.2",
		"printf finished > teardown-finished",
		"```",
		"",
		"```run:shell",
		"true",
		"```",
		"",
	}, "\n")
	root := writeSpecFile(t, "cancel-during-teardown.spec.md", source)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		started := filepath.Join(root, "teardown-started")
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(started); err == nil {
				cancel()
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		cancel()
	}()

	_, err := RunContext(ctx, root, config.Config{Entry: "specs/index.spec.md"}, noopModelRunner{}, RunOptions{})
	<-done
	if !contextCanceled(err) {
		t.Fatalf("run error = %v, want context cancellation", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "teardown-finished")); statErr != nil {
		t.Fatalf("finished teardown marker: %v", statErr)
	}
}

func TestRunTeardownHooksUnwindInnerToOuter(t *testing.T) {
	source := strings.Join([]string{
		"# Lifecycle",
		"",
		"> teardown",
		"```run:board",
		"create-board outer-cleanup",
		"```",
		"",
		"## Nested",
		"",
		"> teardown",
		"```run:board",
		"create-board inner-cleanup",
		"```",
		"",
		"```run:board",
		"create-board case",
		"```",
		"",
	}, "\n")
	root := writeSpecFile(t, "teardown-order.spec.md", source)

	report, err := Run(root, helperAdapterConfig(), noopModelRunner{}, RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	events := report.Results[len(report.Results)-1].LifecycleEvents
	if len(events) != 2 {
		t.Fatalf("lifecycle events = %+v, want inner and outer teardown", events)
	}
	if got := events[0].HeadingPath[len(events[0].HeadingPath)-1]; got != "Nested" {
		t.Fatalf("first teardown scope = %q, want Nested", got)
	}
	if got := events[1].HeadingPath[len(events[1].HeadingPath)-1]; got != "Lifecycle" {
		t.Fatalf("second teardown scope = %q, want Lifecycle", got)
	}
}

func TestRunContinuesOuterTeardownAfterInnerTeardownFailure(t *testing.T) {
	source := strings.Join([]string{
		"# Lifecycle",
		"",
		"> teardown",
		"```run:board",
		"create-board outer-cleanup",
		"```",
		"",
		"## Nested",
		"",
		"> teardown",
		"```run:board",
		"fail-command",
		"```",
		"",
		"```run:board",
		"create-board case",
		"```",
		"",
	}, "\n")
	root := writeSpecFile(t, "teardown-continue.spec.md", source)

	report, err := Run(root, helperAdapterConfig(), noopModelRunner{}, RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	result := report.Results[len(report.Results)-1]
	if result.Status != core.StatusFailed {
		t.Fatalf("document status = %q, want failed", result.Status)
	}
	events := result.LifecycleEvents
	if len(events) != 2 ||
		events[0].Status != core.StatusFailed ||
		events[1].Status != core.StatusPassed {
		t.Fatalf("lifecycle events = %+v, want failed inner then passed outer teardown", events)
	}
}

func TestRunSetupFailureDoesNotInvokeAlloyRunner(t *testing.T) {
	source := strings.Join([]string{
		"# Lifecycle",
		"",
		"> setup",
		"```run:shell",
		"exit 1",
		"```",
		"",
		"```alloy:model(sample)",
		"module sample",
		"sig Item {}",
		"assert exists { some Item }",
		"```",
		"",
		"> alloy:ref(sample#exists, scope=3)",
		"",
	}, "\n")
	root := writeSpecFile(t, "alloy-skipped.spec.md", source)
	runner := &lifecycleAlloyRunner{}

	report, err := Run(root, config.Config{Entry: "specs/index.spec.md"}, runner, RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if runner.calls != 0 {
		t.Fatalf("alloy runner calls = %d, want 0", runner.calls)
	}
	result := report.Results[len(report.Results)-1]
	if len(result.Cases) != 1 || result.Cases[0].Status != core.StatusSkipped {
		t.Fatalf("cases = %+v, want skipped Alloy check", result.Cases)
	}
}

func TestRunInvokesAlloyRunnerAfterSuccessfulSetup(t *testing.T) {
	source := strings.Join([]string{
		"# Lifecycle",
		"",
		"> setup",
		"```run:shell",
		"printf ready > setup-marker",
		"```",
		"",
		"```alloy:model(sample)",
		"module sample",
		"sig Item {}",
		"assert exists { some Item }",
		"```",
		"",
		"> alloy:ref(sample#exists, scope=3)",
		"",
	}, "\n")
	root := writeSpecFile(t, "alloy-after-setup.spec.md", source)
	runner := &lifecycleAlloyRunner{requiredMarker: filepath.Join(root, "setup-marker")}

	report, err := Run(root, config.Config{Entry: "specs/index.spec.md"}, runner, RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("alloy runner calls = %d, want 1", runner.calls)
	}
	result := report.Results[len(report.Results)-1]
	if len(result.Cases) != 1 || result.Cases[0].Status != core.StatusPassed {
		t.Fatalf("cases = %+v, want passed Alloy check", result.Cases)
	}
}

func TestRunUsesDistinctArtifactsForHookedAlloyChecks(t *testing.T) {
	source := strings.Join([]string{
		"# Lifecycle",
		"",
		"> setup",
		"```run:shell",
		"true",
		"```",
		"",
		"```alloy:model(sample)",
		"module sample",
		"sig Item {}",
		"assert first { some Item }",
		"assert second { no Item }",
		"```",
		"",
		"> alloy:ref(sample#first, scope=3)",
		"",
		"> alloy:ref(sample#second, scope=3)",
		"",
	}, "\n")
	root := writeSpecFile(t, "alloy-artifacts.spec.md", source)
	runner := &lifecycleAlloyRunner{}

	report, err := Run(root, config.Config{Entry: "specs/index.spec.md"}, runner, RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if report.Summary.CasesPassed != 2 {
		t.Fatalf("summary = %+v, want two passed Alloy checks", report.Summary)
	}
	if len(runner.artifactSuffixes) != 2 ||
		runner.artifactSuffixes[0] == "" ||
		runner.artifactSuffixes[0] == runner.artifactSuffixes[1] {
		t.Fatalf("artifact suffixes = %v, want two distinct suffixes", runner.artifactSuffixes)
	}
}

func TestRunCancellationStillRunsSectionTeardown(t *testing.T) {
	source := strings.Join([]string{
		"# Lifecycle",
		"",
		"> teardown",
		"```run:shell",
		"printf cleaned > section-cleanup-marker",
		"```",
		"",
		"```run:shell",
		"sleep 30",
		"```",
		"",
	}, "\n")
	root := writeSpecFile(t, "cancel-cleanup.spec.md", source)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	_, err := RunContext(ctx, root, config.Config{Entry: "specs/index.spec.md"}, noopModelRunner{}, RunOptions{})
	if !contextCanceled(err) {
		t.Fatalf("run error = %v, want context cancellation", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "section-cleanup-marker")); statErr != nil {
		t.Fatalf("section teardown marker: %v", statErr)
	}
}

func TestRunCancellationReusesHealthyStatefulSessionForTeardown(t *testing.T) {
	source := strings.Join([]string{
		"# Lifecycle",
		"",
		"> setup",
		"```run:board",
		"create-board stateful-cleanup",
		"```",
		"",
		"> teardown",
		"```run:board",
		`board "stateful-cleanup" should exist`,
		"```",
		"",
		"```run:shell",
		"sleep 30",
		"```",
		"",
	}, "\n")
	root := writeSpecFile(t, "stateful-cancel-cleanup.spec.md", source)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	report, err := RunContext(ctx, root, helperAdapterConfig(), noopModelRunner{}, RunOptions{})
	if !contextCanceled(err) {
		t.Fatalf("run error = %v, want context cancellation", err)
	}
	result := report.Results[len(report.Results)-1]
	if len(result.LifecycleEvents) != 2 ||
		result.LifecycleEvents[0].Status != core.StatusPassed ||
		result.LifecycleEvents[1].Status != core.StatusPassed {
		t.Fatalf("lifecycle events = %+v, want setup and stateful teardown to pass", result.LifecycleEvents)
	}
}

func TestRunTimedOutCaseUsesFreshSessionForTeardown(t *testing.T) {
	source := strings.Join([]string{
		"---",
		"timeout: 50",
		"---",
		"",
		"# Lifecycle",
		"",
		"> teardown",
		"```run:shell",
		"printf cleaned > timeout-cleanup-marker",
		"```",
		"",
		"```run:shell",
		"sleep 30",
		"```",
		"",
	}, "\n")
	root := writeSpecFile(t, "timeout-cleanup.spec.md", source)

	report, err := Run(root, config.Config{Entry: "specs/index.spec.md"}, noopModelRunner{}, RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	result := report.Results[len(report.Results)-1]
	if len(result.Cases) != 1 || result.Cases[0].Status != core.StatusFailed {
		t.Fatalf("cases = %+v, want timed-out failure", result.Cases)
	}
	if len(result.LifecycleEvents) != 1 || result.LifecycleEvents[0].Status != core.StatusPassed {
		t.Fatalf("lifecycle events = %+v, want successful teardown on a fresh session", result.LifecycleEvents)
	}
	if _, statErr := os.Stat(filepath.Join(root, "timeout-cleanup-marker")); statErr != nil {
		t.Fatalf("timeout cleanup marker: %v", statErr)
	}
}

func TestRunFailureLimitPreservesCanceledSiblingTeardownFailure(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}
	failing := strings.Join([]string{
		"# Limit trigger",
		"",
		"```run:shell",
		"while [ ! -f sibling-case-passed ]; do sleep 0.01; done",
		"sleep 0.1",
		"exit 1",
		"```",
		"",
	}, "\n")
	canceled := strings.Join([]string{
		"# Canceled sibling",
		"",
		"> setup",
		"```run:shell",
		"printf started > sibling-started",
		"```",
		"",
		"> teardown",
		"```run:shell",
		"exit 9",
		"```",
		"",
		"```run:shell",
		"printf passed > sibling-case-passed",
		"```",
		"",
		"```run:shell",
		"sleep 30",
		"```",
		"",
	}, "\n")
	firstPath := filepath.Join(specsDir, "limit.spec.md")
	secondPath := filepath.Join(specsDir, "sibling.spec.md")
	if err := os.WriteFile(firstPath, []byte(failing), 0o644); err != nil {
		t.Fatalf("write limit spec: %v", err)
	}
	if err := os.WriteFile(secondPath, []byte(canceled), 0o644); err != nil {
		t.Fatalf("write sibling spec: %v", err)
	}
	writeEntryFile(t, root, firstPath, secondPath)

	report, err := Run(root, config.Config{Entry: "specs/index.spec.md"}, noopModelRunner{}, RunOptions{
		Jobs:        2,
		MaxFailures: 1,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	var sibling *core.DocumentResult
	for i := range report.Results {
		if strings.HasSuffix(report.Results[i].Document.RelativeTo, "sibling.spec.md") {
			sibling = &report.Results[i]
			break
		}
	}
	if sibling == nil {
		t.Fatalf("results = %+v, want canceled sibling result", report.Results)
	}
	if len(sibling.LifecycleEvents) != 2 ||
		sibling.LifecycleEvents[1].Phase != core.HookTeardown ||
		sibling.LifecycleEvents[1].Status != core.StatusFailed {
		t.Fatalf("sibling lifecycle events = %+v, want failed cancellation teardown", sibling.LifecycleEvents)
	}
	if len(sibling.Cases) != 1 || sibling.Cases[0].Status != core.StatusPassed {
		t.Fatalf("sibling cases = %+v, want completed case preserved", sibling.Cases)
	}
	if report.Summary.CasesPassed != 1 {
		t.Fatalf("summary = %+v, want completed sibling case counted", report.Summary)
	}
	if report.Summary.LifecycleFailed != 1 {
		t.Fatalf("summary = %+v, want sibling lifecycle failure", report.Summary)
	}
}

func TestRunCanceledGlobalSetupStillRunsGlobalTeardown(t *testing.T) {
	root := writeSpecFile(t, "global-cancel.spec.md", "# Lifecycle\n\nProse.\n")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	cfg := config.Config{
		Entry:    "specs/index.spec.md",
		Setup:    "sleep 30",
		Teardown: "printf cleaned > global-cleanup-marker",
	}

	report, err := RunContext(ctx, root, cfg, noopModelRunner{}, RunOptions{})
	if !contextCanceled(err) {
		t.Fatalf("run error = %v, want context cancellation", err)
	}
	if len(report.LifecycleEvents) != 2 ||
		report.LifecycleEvents[1].Phase != core.HookTeardown ||
		report.LifecycleEvents[1].Status != core.StatusPassed {
		t.Fatalf("global lifecycle events = %+v, want canceled setup and passed teardown", report.LifecycleEvents)
	}
	if _, statErr := os.Stat(filepath.Join(root, "global-cleanup-marker")); statErr != nil {
		t.Fatalf("global teardown marker: %v", statErr)
	}
}

func TestRunGlobalTeardownFailureIsReturned(t *testing.T) {
	root := writeSpecFile(t, "global-teardown.spec.md", "# Lifecycle\n\nProse.\n")
	cfg := config.Config{
		Entry:    "specs/index.spec.md",
		Teardown: lifecycleFailureCommand(),
	}

	report, err := Run(root, cfg, noopModelRunner{}, RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(report.LifecycleEvents) != 1 {
		t.Fatalf("global lifecycle events = %+v, want one teardown", report.LifecycleEvents)
	}
	event := report.LifecycleEvents[0]
	if event.Scope != core.LifecycleScopeGlobal || event.Phase != core.HookTeardown || event.Status != core.StatusFailed {
		t.Fatalf("global teardown event = %+v, want failed global teardown", event)
	}
	if report.Summary.LifecycleFailed != 1 {
		t.Fatalf("summary = %+v, want one lifecycle failure", report.Summary)
	}
}

func TestRunGlobalSetupFailureReturnsSkippedReport(t *testing.T) {
	root := writeSpecFile(t, "global-setup.spec.md", "# Lifecycle\n\n```run:shell\nprintf should-not-run\n```\n")
	cfg := config.Config{
		Entry:    "specs/index.spec.md",
		Setup:    lifecycleFailureCommand(),
		Teardown: lifecycleSuccessCommand(),
	}

	report, err := Run(root, cfg, noopModelRunner{}, RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(report.LifecycleEvents) != 2 {
		t.Fatalf("global lifecycle events = %+v, want failed setup and passed teardown", report.LifecycleEvents)
	}
	event := report.LifecycleEvents[0]
	if event.Phase != core.HookSetup || event.Status != core.StatusFailed {
		t.Fatalf("global setup event = %+v, want failed global setup", event)
	}
	if report.Summary.LifecycleFailed != 1 {
		t.Fatalf("summary = %+v, want one lifecycle failure", report.Summary)
	}
	if report.Summary.CasesSkipped != 1 {
		t.Fatalf("summary = %+v, want one skipped case", report.Summary)
	}
	if report.Summary.SpecsSkipped != report.Summary.SpecsTotal {
		t.Fatalf("summary = %+v, want every discovered spec skipped", report.Summary)
	}
	result := report.Results[len(report.Results)-1]
	if result.Status != core.StatusSkipped || len(result.Cases) != 1 || result.Cases[0].Status != core.StatusSkipped {
		t.Fatalf("cases = %+v, want one skipped case", result.Cases)
	}
	if got := result.Cases[0].Message; got != globalSetupSkipMessage {
		t.Fatalf("skip message = %q, want %q", got, globalSetupSkipMessage)
	}
	if report.LifecycleEvents[1].Phase != core.HookTeardown || report.LifecycleEvents[1].Status != core.StatusPassed {
		t.Fatalf("global teardown event = %+v, want passed teardown", report.LifecycleEvents[1])
	}
}

func TestRunOnlyLifecycleFailureReturnsReportWithoutEngineError(t *testing.T) {
	root := writeSpecFile(t, "only-lifecycle.spec.md", "# Lifecycle\n\nProse.\n")
	cfg := config.Config{Setup: lifecycleFailureCommand()}

	report, err := Run(root, cfg, noopModelRunner{}, RunOptions{OnlySetup: true})
	if err != nil {
		t.Fatalf("run setup only: %v", err)
	}
	if report.Summary.LifecycleFailed != 1 ||
		len(report.LifecycleEvents) != 1 ||
		report.LifecycleEvents[0].Status != core.StatusFailed {
		t.Fatalf("report = %+v, want first-class setup failure", report)
	}
}

type lifecycleAlloyRunner struct {
	calls            int
	requiredMarker   string
	artifactSuffixes []string
}

func (r *lifecycleAlloyRunner) RunDocument(_ context.Context, plan core.DocumentPlan) ([]core.CaseResult, error) {
	r.calls++
	if len(plan.Cases) > 0 {
		r.artifactSuffixes = append(r.artifactSuffixes, plan.ArtifactSuffix)
	}
	if r.requiredMarker != "" {
		if _, err := os.Stat(r.requiredMarker); err != nil {
			return nil, err
		}
	}
	results := make([]core.CaseResult, 0, len(plan.Cases))
	for i := range plan.Cases {
		specCase := plan.Cases[i]
		if specCase.Kind != core.CaseKindAlloy {
			continue
		}
		results = append(results, core.CaseResult{
			ID:     specCase.ID,
			Kind:   core.CaseKindAlloy,
			Label:  specCase.DefaultLabel(),
			Status: core.StatusPassed,
			Alloy: &core.AlloyResultDetail{
				Model:     specCase.Alloy.Model,
				Assertion: specCase.Alloy.Assertion,
				Scope:     specCase.Alloy.Scope,
			},
		})
	}
	return results, nil
}

func contextCanceled(err error) bool {
	return err != nil && (strings.Contains(err.Error(), context.Canceled.Error()) ||
		strings.Contains(err.Error(), "signal: killed"))
}

func lifecycleFailureCommand() string {
	return strconv.Quote(os.Args[0]) + " -test.run=^TestLifecycleFailureProcess$ -- lifecycle-fail"
}

func lifecycleSuccessCommand() string {
	return strconv.Quote(os.Args[0]) + " -test.run=^TestLifecycleFailureProcess$ -- lifecycle-pass"
}

func TestLifecycleFailureProcess(t *testing.T) {
	if len(os.Args) > 0 && os.Args[len(os.Args)-1] == "lifecycle-fail" {
		os.Exit(7)
	}
}
