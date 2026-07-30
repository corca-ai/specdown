package engine

import (
	"time"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

// assembleReport is the single owner of the top-level report contract.
func assembleReport(title string, results []core.DocumentResult, summary core.Summary) core.Report {
	return core.Report{
		SchemaVersion: core.ReportSchemaVersion,
		Title:         title,
		GeneratedAt:   time.Now(),
		Results:       results,
		Summary:       summary,
	}
}

func assembleExecutionReport(title string, results []core.DocumentResult) core.Report {
	summary := core.Summary{SpecsTotal: len(results)}
	for i := range results {
		accumulateSummary(&summary, results[i])
	}
	return assembleReport(title, results, summary)
}

func skippedRunReport(title string, docs []core.Document, opts RunOptions, message string) (core.Report, error) {
	plan, err := core.CompileDocuments(docs)
	if err != nil {
		return core.Report{}, err
	}
	if opts.Filter != "" {
		plan = filterPlan(plan, opts.Filter)
	}

	results := make([]core.DocumentResult, 0, len(plan.Documents))
	summary := core.Summary{SpecsTotal: len(plan.Documents)}
	for i := range plan.Documents {
		cases := make([]core.CaseResult, 0, len(plan.Documents[i].Cases))
		for j := range plan.Documents[i].Cases {
			cases = append(cases, skippedCaseResult(plan.Documents[i].Cases[j], message))
		}
		result := core.DocumentResult{
			Document: plan.Documents[i].Document,
			Status:   core.StatusSkipped,
			Cases:    cases,
		}
		results = append(results, result)
		accumulateSummary(&summary, result)
	}

	return assembleReport(title, results, summary), nil
}

func dryRunReport(title string, plan core.Plan) core.Report {
	results := make([]core.DocumentResult, 0, len(plan.Documents))
	summary := core.Summary{SpecsTotal: len(plan.Documents)}

	for i := range plan.Documents {
		doc := &plan.Documents[i]
		cases := make([]core.CaseResult, 0, len(doc.Cases))
		for j := range doc.Cases {
			c := &doc.Cases[j]
			cr := core.CaseResult{
				ID:    c.ID,
				Kind:  c.Kind,
				Label: dryRunLabel(*c),
			}
			switch c.Kind {
			case core.CaseKindCode:
				cr.Code = &core.CodeResultDetail{
					Block: c.Code.Block.Descriptor(),
				}
			case core.CaseKindTableRow:
				cr.Table = &core.TableResultDetail{
					Check:     c.TableRow.Check,
					Columns:   append([]string(nil), c.TableRow.Columns...),
					RowNumber: c.TableRow.RowNumber,
				}
			case core.CaseKindAlloy:
				cr.Alloy = &core.AlloyResultDetail{
					Model:     c.Alloy.Model,
					Assertion: c.Alloy.Assertion,
					Scope:     c.Alloy.Scope,
				}
			}
			cases = append(cases, cr)
		}
		results = append(results, core.DocumentResult{
			Document: doc.Document,
			Cases:    cases,
		})
		summary.CasesTotal += len(doc.Cases)
	}

	return assembleReport(title, results, summary)
}

func dryRunLabel(c core.CaseSpec) string {
	if c.Kind == core.CaseKindAlloy {
		return c.DefaultLabel()
	}
	if len(c.ID.HeadingPath) == 0 {
		return c.DisplayKind()
	}
	return c.DisplayKind() + " @ " + c.ID.HeadingPath[len(c.ID.HeadingPath)-1]
}

func lifecycleOnlyReport(events []core.LifecycleEvent) core.Report {
	report := assembleReport("", nil, core.Summary{})
	attachGlobalLifecycle(&report, events)
	return report
}

func attachGlobalLifecycle(report *core.Report, events []core.LifecycleEvent) {
	ensureReportContract(report)
	report.LifecycleEvents = append(report.LifecycleEvents, events...)
	accumulateLifecycleSummary(&report.Summary, events)
}

func ensureReportContract(report *core.Report) {
	if report.SchemaVersion != 0 {
		return
	}
	report.SchemaVersion = core.ReportSchemaVersion
	report.GeneratedAt = time.Now()
}

func accumulateSummary(summary *core.Summary, result core.DocumentResult) {
	switch result.Status {
	case core.StatusPassed:
		summary.SpecsPassed++
	case core.StatusSkipped:
		summary.SpecsSkipped++
	default:
		summary.SpecsFailed++
	}

	summary.CasesTotal += len(result.Cases)
	for i := range result.Cases {
		switch {
		case result.Cases[i].Status == core.StatusPassed:
			summary.CasesPassed++
		case result.Cases[i].Status == core.StatusSkipped:
			summary.CasesSkipped++
		case result.Cases[i].ExpectFail:
			summary.CasesExpectedFail++
		default:
			summary.CasesFailed++
		}
	}
	accumulateLifecycleSummary(summary, result.LifecycleEvents)
}

func accumulateLifecycleSummary(summary *core.Summary, events []core.LifecycleEvent) {
	summary.LifecycleTotal += len(events)
	for i := range events {
		if events[i].Status == core.StatusFailed {
			summary.LifecycleFailed++
		} else {
			summary.LifecyclePassed++
		}
	}
}
