package engine

import (
	"context"
	"errors"

	"github.com/corca-ai/specdown/internal/specdown/adapterhost"
	"github.com/corca-ai/specdown/internal/specdown/config"
	"github.com/corca-ai/specdown/internal/specdown/core"
)

// documentExecutor carries immutable dependencies shared by document runs.
type documentExecutor struct {
	registry       adapterRegistry
	host           adapterhost.Host
	alloyRunner    core.ModelRunner
	defaultTimeout int
	progress       *progressTracker
	budget         *failureBudget
}

func runWithDocs(ctx context.Context, title string, docs []core.Document, cfg config.Config, host adapterhost.Host, alloyRunner core.ModelRunner, opts RunOptions, defaultTimeout int, progress ProgressFunc) (core.Report, error) {
	plan, err := compileRunPlan(docs, opts.Filter)
	if err != nil {
		return core.Report{}, err
	}
	return executeRunPlan(ctx, title, plan, cfg, host, alloyRunner, opts, defaultTimeout, progress)
}

func compileRunPlan(documents []core.Document, filter string) (core.Plan, error) {
	plan, err := core.CompileDocuments(documents)
	if err != nil {
		return core.Plan{}, err
	}
	if filter != "" {
		plan = filterPlan(plan, filter)
	}
	return plan, nil
}

func executeRunPlan(
	ctx context.Context,
	title string,
	plan core.Plan,
	cfg config.Config,
	host adapterhost.Host,
	alloyRunner core.ModelRunner,
	opts RunOptions,
	defaultTimeout int,
	progress ProgressFunc,
) (core.Report, error) {
	if opts.DryRun {
		return dryRunReport(title, plan), nil
	}

	registry, err := buildRegistry(cfg.Adapters)
	if err != nil {
		return core.Report{}, err
	}

	budget := newFailureBudget(opts.MaxFailures)
	executor := &documentExecutor{
		registry:       registry,
		host:           host,
		alloyRunner:    alloyRunner,
		defaultTimeout: defaultTimeout,
		progress:       newProgressTracker(progress, plan.Documents),
		budget:         budget,
	}
	scheduler := newDocumentScheduler(ctx, budget, executor.runDocument)
	defer scheduler.close()
	results, err := scheduler.execute(plan.Documents, opts.Jobs)

	// Filter out unexecuted documents (zero-value entries from early stop).
	var executed []core.DocumentResult
	for i := range results {
		if results[i].Document.RelativeTo != "" || len(results[i].Cases) > 0 {
			executed = append(executed, results[i])
		}
	}

	report := assembleExecutionReport(title, executed)
	if errors.Is(err, errMaxFailures) {
		return report, nil
	}
	return report, err
}

func filterPlan(plan core.Plan, filter string) core.Plan {
	f := parseFilter(filter)
	var filtered []core.DocumentPlan
	for i := range plan.Documents {
		var cases []core.CaseSpec
		for j := range plan.Documents[i].Cases {
			if f.matches(plan.Documents[i].Cases[j]) {
				cases = append(cases, plan.Documents[i].Cases[j])
			}
		}
		if len(cases) > 0 {
			filtered = append(filtered, core.DocumentPlan{
				Document:       plan.Documents[i].Document,
				Cases:          cases,
				Hooks:          plan.Documents[i].Hooks,
				AlloyModels:    plan.Documents[i].AlloyModels,
				ArtifactSuffix: plan.Documents[i].ArtifactSuffix,
			})
		}
	}
	return core.Plan{Documents: filtered}
}
