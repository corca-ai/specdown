package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/corca-ai/specdown/internal/specdown/config"
	"github.com/corca-ai/specdown/internal/specdown/core"
	"github.com/corca-ai/specdown/internal/specdown/subprocess"
	"github.com/corca-ai/specdown/internal/specdown/trace"
)

// ProgressEvent describes a streaming progress notification.
type ProgressEvent struct {
	// Kind is "spec" when a document starts, "case" when a case finishes.
	Kind string
	// Spec is the document-relative path (set for both kinds).
	Spec string
	// Case is set when Kind == "case".
	Case *core.CaseResult
	// CaseNum is the 1-based index of the current case (set for "case" events).
	CaseNum int
	// CasesTotal is the total number of cases in the run.
	CasesTotal int
}

// ProgressFunc is called during execution to stream progress.
// It must be safe to call from multiple goroutines when Jobs > 1.
type ProgressFunc func(ProgressEvent)

// errMaxFailures is a sentinel returned when the failure limit is reached.
var errMaxFailures = errors.New("maximum failure count reached")

type RunOptions struct {
	Filter       string
	Jobs         int
	DryRun       bool
	Progress     ProgressFunc
	MaxFailures  int // 0 means unlimited
	NoSetup      bool
	NoTeardown   bool
	OnlySetup    bool
	OnlyTeardown bool
}

const globalSetupSkipMessage = "not executed because the global setup command failed"
const sectionSetupSkipMessage = "not executed because a section setup hook failed"

func runShellCommand(ctx context.Context, baseDir, command string) error {
	shell, flag := "sh", "-c"
	if runtime.GOOS == "windows" {
		shell, flag = "cmd", "/C"
	}
	cmd := subprocess.CommandContext(ctx, shell, flag, command)
	cmd.Dir = baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func Run(baseDir string, cfg config.Config, modelRunner core.ModelRunner, opts RunOptions) (core.Report, error) {
	return RunContext(context.Background(), baseDir, cfg, modelRunner, opts)
}

func RunContext(ctx context.Context, baseDir string, cfg config.Config, modelRunner core.ModelRunner, opts RunOptions) (core.Report, error) {
	opts = normalizeRunOptions(opts)
	if opts.OnlySetup || opts.OnlyTeardown {
		return runOnlyLifecycle(ctx, baseDir, cfg, opts)
	}
	phases := newRunPhases(ctx, baseDir, cfg, modelRunner, opts)
	if setupErr := phases.setupPhase(); setupErr != nil {
		return phases.finishFailedSetup(setupErr)
	}
	phases.discoveryCompilePhase()
	phases.executionPhase()
	phases.traceValidationPhase()
	phases.teardownPhase()
	return phases.finish()
}

func normalizeRunOptions(options RunOptions) RunOptions {
	if options.Jobs < 1 {
		options.Jobs = 1
	}
	if options.MaxFailures < 1 {
		options.MaxFailures = 0
	}
	return options
}

func lifecycleCleanupContext(ctx context.Context, timeoutMs int) (context.Context, context.CancelFunc) {
	if timeoutMs <= 0 {
		timeoutMs = config.DefaultTimeoutMsec
	}
	return context.WithTimeout(context.WithoutCancel(ctx), time.Duration(timeoutMs)*time.Millisecond)
}

func runGlobalLifecycle(ctx context.Context, baseDir string, phase core.HookKind, command string) (core.LifecycleEvent, error) {
	startedAt := time.Now()
	event := core.LifecycleEvent{
		Scope:  core.LifecycleScopeGlobal,
		Phase:  phase,
		Status: core.StatusPassed,
	}
	err := runShellCommand(ctx, baseDir, command)
	if err != nil {
		event.Status = core.StatusFailed
		event.Message = err.Error()
	}
	event.DurationMs = int(time.Since(startedAt).Milliseconds())
	return event, err
}

// indexResultsByKey builds a lookup map from model runner results.
func indexResultsByKey(results []core.CaseResult) map[string]core.CaseResult {
	m := make(map[string]core.CaseResult, len(results))
	for i := range results {
		m[results[i].ID.Key()] = results[i]
	}
	return m
}

// documentStatus derives the overall document status from case and lifecycle results.
func documentStatus(cases []core.CaseResult, lifecycleEvents ...[]core.LifecycleEvent) core.Status {
	allSkipped := len(cases) > 0
	for i := range cases {
		if cases[i].Status == core.StatusFailed && !cases[i].ExpectFail {
			return core.StatusFailed
		}
		if cases[i].Status != core.StatusSkipped {
			allSkipped = false
		}
	}
	for _, events := range lifecycleEvents {
		for i := range events {
			if events[i].Status == core.StatusFailed {
				return core.StatusFailed
			}
		}
	}
	if allSkipped {
		return core.StatusSkipped
	}
	return core.StatusPassed
}

func (executor *documentExecutor) runDocumentCases(
	ctx context.Context,
	plan core.DocumentPlan,
	sm *sessionManager,
	cleanupSessions *sessionManager,
	precomputed map[string]core.CaseResult,
	lazyAlloy bool,
) ([]core.CaseResult, []core.LifecycleEvent, error) {
	timeout := plan.Document.Frontmatter.Timeout
	if timeout == 0 {
		timeout = executor.defaultTimeout
	}
	caseRun := &caseRunContext{
		caseRunEnvironment: caseRunEnvironment{
			ctx:             ctx,
			registry:        executor.registry,
			sessions:        sm,
			cleanupSessions: cleanupSessions,
			timeoutMs:       timeout,
			spec:            plan.Document.RelativeTo,
			progress:        executor.progress,
			budget:          executor.budget,
			precomputed:     precomputed,
			alloyRunner:     executor.alloyRunner,
			document:        plan.Document,
			alloyModels:     plan.AlloyModels,
			lazyAlloy:       lazyAlloy,
		},
		bindings:        newBindingsManager(),
		hooks:           plan.Hooks,
		results:         make([]core.CaseResult, 0, len(plan.Cases)),
		lifecycleEvents: make([]core.LifecycleEvent, 0, len(plan.Hooks)),
		activeTeardowns: make(map[string]struct{}),
	}

	for i := range plan.Cases {
		if err := ctx.Err(); err != nil {
			caseRun.runTeardownsAfterCancellation(caseRun.prevPath)
			return caseRun.results, caseRun.lifecycleEvents, err
		}
		nextPath := peekNextPath(plan.Cases, i)
		if err := caseRun.processCase(plan.Cases[i], nextPath); err != nil {
			if errors.Is(err, errMaxFailures) {
				return caseRun.results, caseRun.lifecycleEvents, err
			}
			return caseRun.results, caseRun.lifecycleEvents, err
		}
	}
	if err := ctx.Err(); err != nil {
		caseRun.runTeardownsAfterCancellation(caseRun.prevPath)
		return caseRun.results, caseRun.lifecycleEvents, err
	}
	return caseRun.results, caseRun.lifecycleEvents, nil
}

type caseRunEnvironment struct {
	ctx             context.Context
	registry        adapterRegistry
	sessions        *sessionManager
	cleanupSessions *sessionManager
	timeoutMs       int
	spec            string
	progress        *progressTracker
	budget          *failureBudget
	precomputed     map[string]core.CaseResult
	alloyRunner     core.ModelRunner
	document        core.Document
	alloyModels     []core.AlloyModelSpec
	lazyAlloy       bool
}

type caseRunContext struct {
	caseRunEnvironment
	bindings        *bindingsManager
	hooks           []core.HookSpec
	results         []core.CaseResult
	lifecycleEvents []core.LifecycleEvent
	blockedScopes   []core.HeadingPath
	activeTeardowns map[string]struct{}
	prevPath        core.HeadingPath
}

// processCase handles a single case: hooks, execution, result recording.
func (c *caseRunContext) processCase(specCase core.CaseSpec, nextPath core.HeadingPath) error {
	currPath := specCase.ID.HeadingPath
	prevPath := c.prevPath
	if !c.scopeBlocked(currPath) {
		if failedHook := c.runHooksMatching(core.HookSetup, currPath, func(h core.HookSpec) bool {
			return shouldRunHook(h, prevPath, currPath)
		}); failedHook != nil {
			c.blockedScopes = append(c.blockedScopes, hookExecutionScope(*failedHook, currPath))
		}
	}
	c.activateTeardowns(prevPath, currPath)

	var caseErr error
	if c.scopeBlocked(currPath) {
		caseErr = c.recordResult(skippedCaseResult(specCase, sectionSetupSkipMessage), specCase.ID.HeadingPath)
	} else {
		var result core.CaseResult
		if specCase.Kind == core.CaseKindAlloy {
			var err error
			result, err = c.runAlloyCase(specCase)
			if err != nil {
				caseErr = err
			}
		} else {
			var err error
			result, err = runSingleCase(c.ctx, specCase, c.registry, c.sessions, c.bindings.VisibleAt(specCase.ID.HeadingPath), c.timeoutMs)
			if err != nil {
				caseErr = err
			}
		}
		if caseErr == nil {
			caseErr = c.recordResult(result, specCase.ID.HeadingPath)
		}
	}

	teardownNextPath := nextPath
	if caseErr != nil {
		teardownNextPath = nil
	}
	c.runTeardowns(currPath, teardownNextPath)

	c.prevPath = currPath
	return caseErr
}

func (c *caseRunContext) runAlloyCase(specCase core.CaseSpec) (core.CaseResult, error) {
	results := c.precomputed
	if c.lazyAlloy {
		modelResults, err := c.alloyRunner.RunDocument(c.ctx, core.DocumentPlan{
			Document:       c.document,
			Cases:          []core.CaseSpec{specCase},
			AlloyModels:    c.alloyModels,
			ArtifactSuffix: fmt.Sprintf("case-%d", specCase.ID.Ordinal),
		})
		if err != nil {
			return core.CaseResult{}, err
		}
		results = indexResultsByKey(modelResults)
	}
	if result, ok := results[specCase.ID.Key()]; ok {
		return result, nil
	}
	return core.CaseResult{
		ID:      specCase.ID,
		Kind:    core.CaseKindAlloy,
		Label:   specCase.DefaultLabel(),
		Status:  core.StatusFailed,
		Message: "missing model verification result for " + specCase.ID.Key(),
		Alloy: &core.AlloyResultDetail{
			Model:     specCase.Alloy.Model,
			Assertion: specCase.Alloy.Assertion,
			Scope:     specCase.Alloy.Scope,
		},
	}, nil
}

func (c *caseRunContext) runTeardowns(currPath, nextPath core.HeadingPath) {
	hookCtx, cancel := lifecycleCleanupContext(c.ctx, c.timeoutMs)
	defer cancel()
	var sessions sessionProvider = cleanupSessionProvider{
		primary:  c.sessions,
		fallback: c.cleanupSessions,
	}
	c.runHooksMatchingWith(hookCtx, sessions, core.HookTeardown, currPath, func(h core.HookSpec) bool {
		return shouldRunTeardownHook(h, currPath, nextPath)
	})
}

func (c *caseRunContext) runTeardownsAfterCancellation(currPath core.HeadingPath) {
	if len(currPath) == 0 {
		return
	}
	cleanupCtx, cancel := lifecycleCleanupContext(c.ctx, c.timeoutMs)
	defer cancel()
	sessions := cleanupSessionProvider{
		primary:  c.sessions,
		fallback: c.cleanupSessions,
	}
	c.runHooksMatchingWith(cleanupCtx, sessions, core.HookTeardown, currPath, func(h core.HookSpec) bool {
		return shouldRunTeardownHook(h, currPath, nil)
	})
}

func (c *caseRunContext) activateTeardowns(prevPath, currPath core.HeadingPath) {
	for i := range c.hooks {
		hook := c.hooks[i]
		if hook.Kind != core.HookTeardown || !shouldRunHook(hook, prevPath, currPath) {
			continue
		}
		scope := hookExecutionScope(hook, currPath)
		if c.scopeStrictlyBelowBlocked(scope) {
			continue
		}
		c.activeTeardowns[teardownActivationKey(i, scope)] = struct{}{}
	}
}

func (c *caseRunContext) scopeStrictlyBelowBlocked(scope core.HeadingPath) bool {
	for i := range c.blockedScopes {
		blocked := c.blockedScopes[i]
		if len(blocked) < len(scope) && blocked.IsPrefix(scope) {
			return true
		}
	}
	return false
}

func teardownActivationKey(hookIndex int, scope core.HeadingPath) string {
	return fmt.Sprintf("%d|%s", hookIndex, scope.Key())
}

func skippedCaseResult(specCase core.CaseSpec, message string) core.CaseResult {
	result := core.CaseResult{
		ID:      specCase.ID,
		Kind:    specCase.Kind,
		Status:  core.StatusSkipped,
		Label:   specCase.DefaultLabel(),
		Message: message,
		Events: []core.Event{{
			Type:    core.EventCaseSkipped,
			ID:      specCase.ID,
			Label:   specCase.DefaultLabel(),
			Message: message,
		}},
	}
	switch specCase.Kind {
	case core.CaseKindCode:
		result.Code = &core.CodeResultDetail{
			Block:    specCase.Code.Block.Descriptor(),
			Template: specCase.Code.Template,
		}
	case core.CaseKindTableRow:
		result.Table = &core.TableResultDetail{
			Check:         specCase.TableRow.Check,
			Columns:       append([]string(nil), specCase.TableRow.Columns...),
			TemplateCells: append([]string(nil), specCase.TableRow.Cells...),
			RowNumber:     specCase.TableRow.RowNumber,
		}
	case core.CaseKindAlloy:
		result.Alloy = &core.AlloyResultDetail{
			Model:     specCase.Alloy.Model,
			Assertion: specCase.Alloy.Assertion,
			Scope:     specCase.Alloy.Scope,
		}
	}
	return result
}

func (c *caseRunContext) scopeBlocked(path core.HeadingPath) bool {
	for i := range c.blockedScopes {
		if c.blockedScopes[i].IsPrefix(path) {
			return true
		}
	}
	return false
}

// recordResult appends a case result, records bindings, emits progress,
// and returns errMaxFailures when the failure limit is reached.
func (c *caseRunContext) recordResult(result core.CaseResult, path core.HeadingPath) error {
	c.results = append(c.results, result)
	if result.Status != core.StatusFailed {
		c.bindings.Add(result.Bindings, path)
	}
	if c.progress != nil {
		c.progress.caseFinished(c.spec, result)
	}
	if result.Status == core.StatusFailed && !result.ExpectFail &&
		c.budget.recordUnexpectedFailure() {
		return errMaxFailures
	}
	return nil
}

// peekNextPath returns the heading path of the next case, or nil if at the end.
func peekNextPath(cases []core.CaseSpec, current int) core.HeadingPath {
	if current+1 < len(cases) {
		return cases[current+1].ID.HeadingPath
	}
	return nil
}

func buildTraceGraphData(g trace.Graph) *core.TraceGraphData {
	docs := make([]core.TraceDocument, len(g.Documents))
	for i, d := range g.Documents {
		docs[i] = core.TraceDocument{Path: d.Path, Type: d.Type}
	}
	edges := make([]core.TraceEdge, len(g.DirectEdges))
	for i, e := range g.DirectEdges {
		edges[i] = core.TraceEdge{Source: e.Source, Target: e.Target, EdgeName: e.EdgeName}
	}
	transitive := make([]core.TraceEdge, len(g.TransitiveEdges))
	for i, e := range g.TransitiveEdges {
		transitive[i] = core.TraceEdge{Source: e.Source, Target: e.Target, EdgeName: e.EdgeName}
	}
	return &core.TraceGraphData{
		Documents:       docs,
		Edges:           edges,
		TransitiveEdges: transitive,
	}
}

// runOnlyLifecycle runs only setup and/or teardown commands without executing specs.
func runOnlyLifecycle(ctx context.Context, baseDir string, cfg config.Config, opts RunOptions) (core.Report, error) {
	run := &lifecycleOnlyRun{ctx: ctx, baseDir: baseDir, config: cfg}
	if opts.OnlySetup {
		terminal, report, err := run.setupPhase()
		if terminal {
			return report, err
		}
	}
	if opts.OnlyTeardown {
		return run.teardownPhase()
	}
	return lifecycleOnlyReport(run.events), nil
}
