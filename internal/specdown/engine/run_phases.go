package engine

import (
	"context"
	"errors"
	"fmt"

	"github.com/corca-ai/specdown/internal/specdown/adapterhost"
	"github.com/corca-ai/specdown/internal/specdown/config"
	"github.com/corca-ai/specdown/internal/specdown/core"
	"github.com/corca-ai/specdown/internal/specdown/trace"
)

type runPhases struct {
	ctx         context.Context
	baseDir     string
	config      config.Config
	modelRunner core.ModelRunner
	options     RunOptions

	globalEvents []core.LifecycleEvent
	title        string
	documents    []core.Document
	plan         core.Plan
	report       core.Report
	runErr       error
}

type lifecycleOnlyRun struct {
	ctx     context.Context
	baseDir string
	config  config.Config
	events  []core.LifecycleEvent
}

func newRunPhases(
	ctx context.Context,
	baseDir string,
	cfg config.Config,
	modelRunner core.ModelRunner,
	options RunOptions,
) *runPhases {
	return &runPhases{
		ctx:         ctx,
		baseDir:     baseDir,
		config:      cfg,
		modelRunner: modelRunner,
		options:     options,
	}
}

func (phases *runPhases) setupPhase() error {
	if phases.config.Setup == "" || phases.options.NoSetup {
		return nil
	}
	event, err := runGlobalLifecycle(
		phases.ctx,
		phases.baseDir,
		core.HookSetup,
		phases.config.Setup,
	)
	phases.globalEvents = append(phases.globalEvents, event)
	if event.Status == core.StatusFailed {
		return err
	}
	return nil
}

func (phases *runPhases) discoveryCompilePhase() {
	phases.title, phases.documents, phases.runErr = core.DiscoverFromEntry(
		phases.baseDir,
		phases.config.Entry,
		phases.config.IgnorePrefixes,
	)
	if phases.runErr != nil {
		return
	}
	phases.plan, phases.runErr = compileRunPlan(phases.documents, phases.options.Filter)
}

func (phases *runPhases) executionPhase() {
	if phases.runErr != nil {
		return
	}
	phases.report, phases.runErr = executeRunPlan(
		phases.ctx,
		phases.title,
		phases.plan,
		phases.config,
		adapterhost.Host{BaseDir: phases.baseDir},
		phases.modelRunner,
		phases.options,
		phases.config.EffectiveDefaultTimeout(),
		phases.options.Progress,
	)
}

func (phases *runPhases) traceValidationPhase() {
	if phases.runErr != nil || phases.config.Trace == nil {
		return
	}
	graph, traceErrors := trace.Validate(phases.baseDir, phases.config.Trace)
	phases.report.TraceErrors = make([]string, 0, len(traceErrors))
	for _, traceErr := range traceErrors {
		phases.report.TraceErrors = append(phases.report.TraceErrors, traceErr.Error())
	}
	phases.report.Summary.TraceErrorCount = len(traceErrors)
	phases.report.TraceGraph = buildTraceGraphData(graph)
}

func (phases *runPhases) teardownPhase() {
	if phases.config.Teardown == "" || phases.options.NoTeardown {
		return
	}
	cleanupCtx, cancel := lifecycleCleanupContext(
		phases.ctx,
		phases.config.EffectiveDefaultTimeout(),
	)
	event, _ := runGlobalLifecycle(
		cleanupCtx,
		phases.baseDir,
		core.HookTeardown,
		phases.config.Teardown,
	)
	cancel()
	phases.globalEvents = append(phases.globalEvents, event)
}

func (phases *runPhases) finish() (core.Report, error) {
	if phases.runErr == nil && phases.ctx.Err() != nil {
		phases.runErr = phases.ctx.Err()
	}
	attachGlobalLifecycle(&phases.report, phases.globalEvents)
	return phases.report, phases.runErr
}

func (phases *runPhases) finishFailedSetup(setupErr error) (core.Report, error) {
	if phases.ctx.Err() != nil {
		phases.teardownPhase()
		return lifecycleOnlyReport(phases.globalEvents), fmt.Errorf(
			"setup command failed: %w",
			setupErr,
		)
	}

	title, documents, discoveryErr := core.DiscoverFromEntry(
		phases.baseDir,
		phases.config.Entry,
		phases.config.IgnorePrefixes,
	)
	phases.teardownPhase()
	if discoveryErr != nil {
		return lifecycleOnlyReport(phases.globalEvents), errors.Join(
			fmt.Errorf("setup command failed: %w", setupErr),
			discoveryErr,
		)
	}
	report, reportErr := skippedRunReport(
		title,
		documents,
		phases.options,
		globalSetupSkipMessage,
	)
	if reportErr != nil {
		return lifecycleOnlyReport(phases.globalEvents), errors.Join(
			fmt.Errorf("setup command failed: %w", setupErr),
			reportErr,
		)
	}
	attachGlobalLifecycle(&report, phases.globalEvents)
	return report, nil
}

func (run *lifecycleOnlyRun) setupPhase() (terminal bool, report core.Report, err error) {
	if run.config.Setup == "" {
		return true, core.Report{}, fmt.Errorf("no setup command configured in specdown.json")
	}
	event, setupErr := runGlobalLifecycle(
		run.ctx,
		run.baseDir,
		core.HookSetup,
		run.config.Setup,
	)
	run.events = append(run.events, event)
	if event.Status != core.StatusFailed {
		return false, core.Report{}, nil
	}
	report = lifecycleOnlyReport(run.events)
	if run.ctx.Err() != nil {
		return true, report, fmt.Errorf("setup command failed: %w", setupErr)
	}
	return true, report, nil
}

func (run *lifecycleOnlyRun) teardownPhase() (core.Report, error) {
	if run.config.Teardown == "" {
		return core.Report{}, fmt.Errorf("no teardown command configured in specdown.json")
	}
	cleanupCtx, cancel := lifecycleCleanupContext(
		run.ctx,
		run.config.EffectiveDefaultTimeout(),
	)
	event, teardownErr := runGlobalLifecycle(
		cleanupCtx,
		run.baseDir,
		core.HookTeardown,
		run.config.Teardown,
	)
	cancel()
	run.events = append(run.events, event)
	report := lifecycleOnlyReport(run.events)
	if event.Status == core.StatusFailed && run.ctx.Err() != nil {
		return report, fmt.Errorf("teardown command failed: %w", teardownErr)
	}
	if run.ctx.Err() != nil {
		return report, run.ctx.Err()
	}
	return report, nil
}
