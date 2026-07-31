package cli

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/corca-ai/specdown/internal/specdown/core"
	"github.com/corca-ai/specdown/internal/specdown/engine"
)

func (c command) printBindings(report core.Report) {
	for i := range report.Results {
		for j := range report.Results[i].Cases {
			result := report.Results[i].Cases[j]
			if len(result.VisibleBindings) == 0 {
				continue
			}
			path := strings.Join(result.ID.HeadingPath, " > ")
			kind := caseKindLabel(result)
			var pairs []string
			for _, binding := range result.VisibleBindings {
				pairs = append(pairs, fmt.Sprintf("$%s=%v", binding.Name, binding.Value))
			}
			writeFormat(c.stderr, "  BIND  %s  [%s]  %s\n", path, kind, strings.Join(pairs, ", "))
		}
	}
}

func (c command) printTraceErrors(report core.Report) {
	for _, traceError := range report.TraceErrors {
		writeFormat(c.stderr, "trace: %s\n", traceError)
	}
}

func (c command) printWarnings(report core.Report) {
	for i := range report.Results {
		for _, warning := range report.Results[i].Document.Warnings {
			writeFormat(c.stderr, "warning: %s\n", warning)
		}
	}
}

func (c command) printLifecycleFailures(report core.Report) {
	for i := range report.LifecycleEvents {
		c.printLifecycleFailure(report.LifecycleEvents[i])
	}
	for i := range report.Results {
		for j := range report.Results[i].LifecycleEvents {
			c.printLifecycleFailure(report.Results[i].LifecycleEvents[j])
		}
	}
}

func (c command) printLifecycleFailure(event core.LifecycleEvent) {
	if event.Status != core.StatusFailed {
		return
	}
	label := string(event.Scope) + " " + string(event.Phase)
	if event.Each {
		label += ":each"
	}
	location := ""
	if event.File != "" {
		location = "  " + event.File
	}
	if len(event.HeadingPath) > 0 {
		location += "  " + strings.Join(event.HeadingPath, " > ")
	}
	writeFormat(c.stderr, "  FAIL  lifecycle %s%s  (%dms)\n", label, location, event.DurationMs)
	if event.Message != "" {
		for _, line := range strings.Split(event.Message, "\n") {
			writeFormat(c.stderr, "        %s\n", line)
		}
	}
}

func (c command) stdoutProgress() engine.ProgressFunc {
	var mu sync.Mutex
	return func(event engine.ProgressEvent) {
		mu.Lock()
		defer mu.Unlock()
		switch event.Kind {
		case "spec":
			writeFormat(c.stdout, "spec: %s\n", event.Spec)
		case "case":
			c.printCaseResult(*event.Case, event.CaseNum, event.CasesTotal)
		}
	}
}

func caseTag(status core.Status, expectFail bool) string {
	if status == core.StatusSkipped {
		return "SKIP"
	}
	if status == core.StatusFailed {
		if expectFail {
			return "XFAIL"
		}
		return "FAIL"
	}
	return "PASS"
}

func caseKindLabel(result core.CaseResult) string {
	switch {
	case result.Code != nil:
		return result.Code.Block
	case result.Table != nil:
		return result.Table.Check
	case result.Alloy != nil:
		return "alloy:" + result.Alloy.Model + "#" + result.Alloy.Assertion
	default:
		return "expect"
	}
}

func printIndented(writer io.Writer, prefix, value string) {
	lines := strings.Split(value, "\n")
	writeFormat(writer, "%s%s\n", prefix, lines[0])
	indent := strings.Repeat(" ", len(prefix))
	for _, line := range lines[1:] {
		writeFormat(writer, "%s%s\n", indent, line)
	}
}

func (c command) printCaseResult(result core.CaseResult, caseNum, casesTotal int) {
	tag := caseTag(result.Status, result.ExpectFail)
	kind := caseKindLabel(result)
	label := ""
	if result.Table != nil && result.Table.RowNumber > 0 {
		label = fmt.Sprintf(" row %d", result.Table.RowNumber)
	}
	counter := ""
	if casesTotal > 0 {
		counter = fmt.Sprintf("[%d/%d] ", caseNum, casesTotal)
	}
	writeFormat(c.stdout, "  %s%s  %s  [%s]%s  (%dms)\n", counter, tag, strings.Join(result.ID.HeadingPath, " > "), kind, label, result.DurationMs)

	if result.Status == core.StatusFailed {
		c.printFailureDetail(result)
	}
}

func (c command) printFailureDetail(result core.CaseResult) {
	if result.ID.Line > 0 {
		writeFormat(c.stdout, "        %s:%d\n", result.ID.File, result.ID.Line)
	}
	if result.Code != nil && result.Code.ExitCode != nil {
		writeFormat(c.stdout, "        exit code %d\n", *result.Code.ExitCode)
	}
	if result.Message != "" {
		printIndented(c.stdout, "        ", result.Message)
	}
	if result.Code != nil && result.Code.Stderr != "" && result.Code.Stderr != result.Message {
		printIndented(c.stdout, "        stderr: ", result.Code.Stderr)
	}
	if result.Expected != "" {
		writeFormat(c.stdout, "        expected: %s\n", result.Expected)
	}
	if result.Actual != "" {
		writeFormat(c.stdout, "        actual:   %s\n", result.Actual)
	}
	c.printCodeDetail(result)
	c.printFailureBindings(result)
	c.printTableDetail(result)
	c.printFailedDoctestSteps(result)
}

func (c command) printCodeDetail(result core.CaseResult) {
	if result.Code == nil || result.Code.RenderedSource == "" || len(result.Code.Steps) > 0 {
		return
	}
	writeLine(c.stdout, "        source:")
	for _, line := range strings.Split(result.Code.RenderedSource, "\n") {
		writeFormat(c.stdout, "          %s\n", line)
	}
}

func (c command) printFailureBindings(result core.CaseResult) {
	if len(result.VisibleBindings) == 0 {
		return
	}
	var pairs []string
	for _, binding := range result.VisibleBindings {
		pairs = append(pairs, fmt.Sprintf("$%s=%v", binding.Name, binding.Value))
	}
	writeFormat(c.stdout, "        bindings: %s\n", strings.Join(pairs, ", "))
}

func (c command) printTableDetail(result core.CaseResult) {
	if result.Table == nil || len(result.Table.Columns) == 0 || len(result.Table.RenderedCells) == 0 {
		return
	}
	var pairs []string
	for columnIndex, column := range result.Table.Columns {
		if columnIndex < len(result.Table.RenderedCells) {
			pairs = append(pairs, fmt.Sprintf("%s=%s", column, result.Table.RenderedCells[columnIndex]))
		}
	}
	writeFormat(c.stdout, "        row: %s\n", strings.Join(pairs, ", "))
}

func (c command) printFailedDoctestSteps(result core.CaseResult) {
	if result.Code == nil {
		return
	}
	for _, step := range result.Code.Steps {
		if step.Status != core.StatusFailed {
			continue
		}
		writeFormat(c.stdout, "        $ %s\n", step.Command)
		writeFormat(c.stdout, "        expected: %s\n", step.Expected)
		writeFormat(c.stdout, "        actual:   %s\n", step.Actual)
	}
}

func (c command) printDryRun(report core.Report) {
	for i := range report.Results {
		writeFormat(c.stdout, "spec: %s\n", report.Results[i].Document.RelativeTo)
		for j := range report.Results[i].Cases {
			result := report.Results[i].Cases[j]
			if result.Alloy != nil {
				writeFormat(c.stdout, "  alloy: %s [%s#%s, scope=%s]\n", strings.Join(result.ID.HeadingPath, " > "), result.Alloy.Model, result.Alloy.Assertion, result.Alloy.Scope)
				continue
			}
			kind := caseKindLabel(result)
			writeFormat(c.stdout, "  case: %s [%s]\n", strings.Join(result.ID.HeadingPath, " > "), kind)
		}
	}
	c.printDryRunSummary(report)
}

func (c command) printDryRunSummary(report core.Report) {
	writeFormat(c.stdout, "\ntotal: %d spec(s), %d case(s)\n",
		report.Summary.SpecsTotal, report.Summary.CasesTotal)
}
