package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/corca-ai/specdown/internal/specdown/adapterhost"
	"github.com/corca-ai/specdown/internal/specdown/alloy"
	"github.com/corca-ai/specdown/internal/specdown/config"
	"github.com/corca-ai/specdown/internal/specdown/core"
	"github.com/corca-ai/specdown/internal/specdown/subprocess"
	"github.com/corca-ai/specdown/internal/specdown/trace"
)

// adapterEntry holds an adapter config for registry lookups.
type adapterEntry struct {
	Config config.AdapterConfig
}

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

type adapterRegistry struct {
	blocks map[string]adapterEntry
	checks map[string]adapterEntry
}

// executionContext carries shared state through the document execution call chain.
type executionContext struct {
	ctx            context.Context
	cancel         context.CancelFunc
	registry       adapterRegistry
	host           adapterhost.Host
	alloyRunner    core.ModelRunner
	defaultTimeout int
	progress       ProgressFunc
	maxFailures    int
	failures       *atomic.Int32
	casesTotal     int
	caseCounter    *atomic.Int32
	limitHit       *atomic.Bool
	limitReached   chan struct{}
	limitOnce      *sync.Once
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

//nolint:gocognit // top-level orchestration with setup/teardown/trace phases
func Run(baseDir string, cfg config.Config, modelRunner core.ModelRunner, opts RunOptions) (core.Report, error) {
	return RunContext(context.Background(), baseDir, cfg, modelRunner, opts)
}

//nolint:gocognit // top-level orchestration with setup/teardown/trace phases
func RunContext(ctx context.Context, baseDir string, cfg config.Config, modelRunner core.ModelRunner, opts RunOptions) (core.Report, error) {
	if opts.OnlySetup || opts.OnlyTeardown {
		return runOnlyLifecycle(ctx, baseDir, cfg, opts)
	}

	var globalEvents []core.LifecycleEvent
	if cfg.Setup != "" && !opts.NoSetup {
		event, setupErr := runGlobalLifecycle(ctx, baseDir, core.HookSetup, cfg.Setup)
		globalEvents = append(globalEvents, event)
		if event.Status == core.StatusFailed {
			if ctx.Err() != nil {
				if cfg.Teardown != "" && !opts.NoTeardown {
					cleanupCtx, cancel := lifecycleCleanupContext(ctx, cfg.EffectiveDefaultTimeout())
					teardownEvent, _ := runGlobalLifecycle(cleanupCtx, baseDir, core.HookTeardown, cfg.Teardown)
					cancel()
					globalEvents = append(globalEvents, teardownEvent)
				}
				return lifecycleOnlyReport(globalEvents), fmt.Errorf("setup command failed: %w", setupErr)
			}
			title, docs, reportErr := core.DiscoverFromEntry(baseDir, cfg.Entry, cfg.IgnorePrefixes)
			if cfg.Teardown != "" && !opts.NoTeardown {
				cleanupCtx, cancel := lifecycleCleanupContext(ctx, cfg.EffectiveDefaultTimeout())
				teardownEvent, _ := runGlobalLifecycle(cleanupCtx, baseDir, core.HookTeardown, cfg.Teardown)
				cancel()
				globalEvents = append(globalEvents, teardownEvent)
			}
			if reportErr != nil {
				report := lifecycleOnlyReport(globalEvents)
				return report, errors.Join(fmt.Errorf("setup command failed: %w", setupErr), reportErr)
			}
			report, reportErr := skippedRunReport(title, docs, opts, globalSetupSkipMessage)
			if reportErr != nil {
				fallback := lifecycleOnlyReport(globalEvents)
				return fallback, errors.Join(fmt.Errorf("setup command failed: %w", setupErr), reportErr)
			}
			report.LifecycleEvents = append(report.LifecycleEvents, globalEvents...)
			accumulateLifecycleSummary(&report.Summary, globalEvents)
			return report, nil
		}
	}

	title, docs, err := core.DiscoverFromEntry(baseDir, cfg.Entry, cfg.IgnorePrefixes)
	var report core.Report
	if err == nil {
		host := adapterhost.Host{BaseDir: baseDir}
		defaultTimeout := cfg.EffectiveDefaultTimeout()
		progress := opts.Progress
		if progress == nil {
			progress = func(ProgressEvent) {}
		}
		report, err = runWithDocs(ctx, title, docs, cfg, host, modelRunner, opts, defaultTimeout, progress)
	}

	// Run trace validation when trace is configured
	if err == nil && cfg.Trace != nil {
		graph, traceErrs := trace.Validate(baseDir, cfg.Trace)
		report.TraceErrors = make([]string, 0, len(traceErrs))
		for _, e := range traceErrs {
			report.TraceErrors = append(report.TraceErrors, e.Error())
		}
		if len(traceErrs) > 0 {
			report.Summary.TraceErrorCount = len(traceErrs)
		}
		report.TraceGraph = buildTraceGraphData(graph)
	}

	if cfg.Teardown != "" && !opts.NoTeardown {
		teardownCtx, cancel := lifecycleCleanupContext(ctx, cfg.EffectiveDefaultTimeout())
		teardownEvent, _ := runGlobalLifecycle(teardownCtx, baseDir, core.HookTeardown, cfg.Teardown)
		cancel()
		globalEvents = append(globalEvents, teardownEvent)
	}
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	if report.SchemaVersion == 0 {
		report.SchemaVersion = core.ReportSchemaVersion
		report.GeneratedAt = time.Now()
	}
	report.LifecycleEvents = append(report.LifecycleEvents, globalEvents...)
	accumulateLifecycleSummary(&report.Summary, globalEvents)

	return report, err
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

func lifecycleOnlyReport(events []core.LifecycleEvent) core.Report {
	report := core.Report{
		SchemaVersion:   core.ReportSchemaVersion,
		GeneratedAt:     time.Now(),
		LifecycleEvents: append([]core.LifecycleEvent(nil), events...),
	}
	accumulateLifecycleSummary(&report.Summary, events)
	return report
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

	return core.Report{
		SchemaVersion: core.ReportSchemaVersion,
		Title:         title,
		GeneratedAt:   time.Now(),
		Results:       results,
		Summary:       summary,
	}, nil
}

// ModelExplorer runs Alloy models and returns instance-level results.
type ModelExplorer interface {
	ExploreDocument(ctx context.Context, plan core.DocumentPlan, opts alloy.ExploreOptions) ([]alloy.ExploreModelResult, error)
}

// ModelDumper can write model artifacts without running verification.
type ModelDumper interface {
	DumpModels(plan core.DocumentPlan) ([]string, error)
}

// ExploreModels runs Alloy models from all discovered documents and returns
// per-model results grouped by document path.
func ExploreModels(baseDir string, cfg config.Config, explorer ModelExplorer, filter string, opts alloy.ExploreOptions) (map[string][]alloy.ExploreModelResult, error) {
	return ExploreModelsContext(context.Background(), baseDir, cfg, explorer, filter, opts)
}

func ExploreModelsContext(ctx context.Context, baseDir string, cfg config.Config, explorer ModelExplorer, filter string, opts alloy.ExploreOptions) (map[string][]alloy.ExploreModelResult, error) {
	_, docs, err := core.DiscoverFromEntry(baseDir, cfg.Entry, cfg.IgnorePrefixes)
	if err != nil {
		return nil, err
	}

	plan, err := core.CompileDocuments(docs)
	if err != nil {
		return nil, err
	}

	if filter != "" {
		plan = filterPlanByDoc(plan, filter)
	}

	results := make(map[string][]alloy.ExploreModelResult)
	for i := range plan.Documents {
		docPath := plan.Documents[i].Document.RelativeTo
		explored, err := explorer.ExploreDocument(ctx, plan.Documents[i], opts)
		if err != nil {
			return nil, err
		}
		if len(explored) > 0 {
			results[docPath] = explored
		}
	}
	return results, nil
}

// filterPlanByDoc keeps only documents whose RelativeTo path contains the filter substring.
func filterPlanByDoc(plan core.Plan, filter string) core.Plan {
	var filtered []core.DocumentPlan
	for i := range plan.Documents {
		if strings.Contains(plan.Documents[i].Document.RelativeTo, filter) {
			filtered = append(filtered, plan.Documents[i])
		}
	}
	return core.Plan{Documents: filtered}
}

func DumpModels(baseDir string, cfg config.Config, dumper ModelDumper) ([]string, error) {
	_, docs, err := core.DiscoverFromEntry(baseDir, cfg.Entry, cfg.IgnorePrefixes)
	if err != nil {
		return nil, err
	}

	plan, err := core.CompileDocuments(docs)
	if err != nil {
		return nil, err
	}

	var paths []string
	for i := range plan.Documents {
		dumped, err := dumper.DumpModels(plan.Documents[i])
		if err != nil {
			return nil, err
		}
		paths = append(paths, dumped...)
	}
	return paths, nil
}

func runWithDocs(ctx context.Context, title string, docs []core.Document, cfg config.Config, host adapterhost.Host, alloyRunner core.ModelRunner, opts RunOptions, defaultTimeout int, progress ProgressFunc) (core.Report, error) {
	plan, err := core.CompileDocuments(docs)
	if err != nil {
		return core.Report{}, err
	}

	if opts.Filter != "" {
		plan = filterPlan(plan, opts.Filter)
	}

	if opts.DryRun {
		report := dryRunReport(plan)
		report.Title = title
		return report, nil
	}

	registry, err := buildRegistry(cfg.Adapters)
	if err != nil {
		return core.Report{}, err
	}

	jobs := opts.Jobs
	if jobs < 1 {
		jobs = 1
	}

	var casesTotal int
	for i := range plan.Documents {
		casesTotal += len(plan.Documents[i].Cases)
	}

	var failures atomic.Int32
	var caseCounter atomic.Int32
	var limitHit atomic.Bool
	limitReached := make(chan struct{})
	var limitOnce sync.Once
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	ec := &executionContext{
		ctx:            runCtx,
		cancel:         cancel,
		registry:       registry,
		host:           host,
		alloyRunner:    alloyRunner,
		defaultTimeout: defaultTimeout,
		progress:       progress,
		maxFailures:    opts.MaxFailures,
		failures:       &failures,
		casesTotal:     casesTotal,
		caseCounter:    &caseCounter,
		limitHit:       &limitHit,
		limitReached:   limitReached,
		limitOnce:      &limitOnce,
	}
	results, err := ec.executeDocuments(plan.Documents, jobs)

	// Filter out unexecuted documents (zero-value entries from early stop).
	var executed []core.DocumentResult
	for i := range results {
		if results[i].Document.RelativeTo != "" || len(results[i].Cases) > 0 {
			executed = append(executed, results[i])
		}
	}

	summary := core.Summary{SpecsTotal: len(executed)}
	for i := range executed {
		accumulateSummary(&summary, executed[i])
	}
	results = executed

	report := core.Report{
		SchemaVersion: core.ReportSchemaVersion,
		Title:         title,
		GeneratedAt:   time.Now(),
		Results:       results,
		Summary:       summary,
	}
	if errors.Is(err, errMaxFailures) {
		return report, nil
	}
	return report, err
}

func (ec *executionContext) executeDocuments(documents []core.DocumentPlan, jobs int) ([]core.DocumentResult, error) {
	results := make([]core.DocumentResult, len(documents))
	if len(documents) == 0 {
		return results, nil
	}

	if jobs > len(documents) {
		jobs = len(documents)
	}
	tasks := make(chan int)
	var wg sync.WaitGroup
	var errOnce sync.Once
	var firstErr error

	recordError := func(err error) {
		errOnce.Do(func() {
			firstErr = err
			ec.cancel()
		})
	}

	for range jobs {
		wg.Add(1)
		go ec.runDocumentWorker(documents, results, tasks, recordError, &wg)
	}

	ec.sendDocumentTasks(tasks, len(documents))
	wg.Wait()

	switch {
	case ec.limitHit.Load():
		return results, errMaxFailures
	case firstErr != nil:
		return results, firstErr
	case ec.ctx.Err() != nil:
		return results, ec.ctx.Err()
	default:
		return results, nil
	}
}

func (ec *executionContext) runDocumentWorker(
	documents []core.DocumentPlan,
	results []core.DocumentResult,
	tasks <-chan int,
	recordError func(error),
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	for {
		select {
		case <-ec.ctx.Done():
			return
		case <-ec.limitReached:
			return
		case i, ok := <-tasks:
			if !ok || ec.ctx.Err() != nil || ec.limitHit.Load() {
				return
			}
			result, err := ec.runDocument(documents[i])
			results[i] = result
			if err != nil {
				recordError(err)
				return
			}
		}
	}
}

func (ec *executionContext) sendDocumentTasks(tasks chan<- int, count int) {
	defer close(tasks)
	for i := range count {
		select {
		case tasks <- i:
		case <-ec.ctx.Done():
			return
		case <-ec.limitReached:
			return
		}
	}
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

func dryRunReport(plan core.Plan) core.Report {
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

	return core.Report{
		SchemaVersion: core.ReportSchemaVersion,
		GeneratedAt:   time.Now(),
		Results:       results,
		Summary:       summary,
	}
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

func buildRegistry(adapters []config.AdapterConfig) (adapterRegistry, error) {
	registry := adapterRegistry{
		blocks: make(map[string]adapterEntry),
		checks: make(map[string]adapterEntry),
	}
	for _, adapter := range adapters {
		entry := adapterEntry{Config: adapter}
		for _, block := range adapter.Blocks {
			if previous, exists := registry.blocks[block]; exists {
				return adapterRegistry{}, fmt.Errorf("block %q is declared by both adapter %q and %q", block, previous.Config.Name, adapter.Name)
			}
			registry.blocks[block] = entry
		}
		for _, check := range adapter.Checks {
			if previous, exists := registry.checks[check]; exists {
				return adapterRegistry{}, fmt.Errorf("check %q is declared by both adapter %q and %q", check, previous.Config.Name, adapter.Name)
			}
			registry.checks[check] = entry
		}
	}

	// Auto-register built-in shell adapter for unclaimed shell blocks.
	builtinEntry := adapterEntry{Config: config.AdapterConfig{
		Name:         "__builtin_shell",
		BuiltinShell: true,
	}}
	for _, block := range []string{"run:shell"} {
		if _, exists := registry.blocks[block]; !exists {
			registry.blocks[block] = builtinEntry
		}
	}

	// Auto-register built-in jq check adapter for unclaimed jq checks.
	if _, exists := registry.checks["jq"]; !exists {
		registry.checks["jq"] = adapterEntry{Config: config.AdapterConfig{
			Name:      "__builtin_jq",
			BuiltinJQ: true,
		}}
	}

	return registry, nil
}

func (r adapterRegistry) adapterFor(specCase core.CaseSpec) (adapterEntry, error) {
	switch specCase.Kind {
	case core.CaseKindCode:
		desc := specCase.Code.Block.Descriptor()
		entry, ok := r.blocks[desc]
		if !ok {
			return adapterEntry{}, fmt.Errorf("no adapter supports block %q in %s\nhint: declare this block in an adapter's \"blocks\" list in specdown.json", desc, specCase.ID.Key())
		}
		return entry, nil
	case core.CaseKindTableRow:
		check := specCase.TableRow.Check
		entry, ok := r.checks[check]
		if !ok {
			return adapterEntry{}, fmt.Errorf("no adapter supports check %q in %s\nhint: declare this check in an adapter's \"checks\" list in specdown.json", check, specCase.ID.Key())
		}
		return entry, nil
	default:
		return adapterEntry{}, fmt.Errorf("unsupported case kind %q", specCase.Kind)
	}
}

//nolint:gocognit // coordinates sessions, Alloy scheduling, cleanup, and result assembly
func (ec *executionContext) runDocument(plan core.DocumentPlan) (core.DocumentResult, error) {
	if err := ec.ctx.Err(); err != nil {
		return core.DocumentResult{}, err
	}
	ec.progress(ProgressEvent{Kind: "spec", Spec: plan.Document.RelativeTo})

	if len(plan.Cases) == 0 {
		return core.DocumentResult{
			Document: plan.Document,
			Status:   core.StatusPassed,
		}, nil
	}

	host := ec.host
	if wd := plan.Document.Frontmatter.Workdir; wd != "" {
		resolved := filepath.Join(ec.host.BaseDir, wd)
		if err := os.MkdirAll(resolved, 0o755); err != nil {
			return core.DocumentResult{}, fmt.Errorf("create workdir %q: %w", wd, err)
		}
		host = adapterhost.Host{BaseDir: resolved}
	}
	sm := newSessionManager(context.WithoutCancel(ec.ctx), host)
	cleanupSessions := newSessionManager(context.WithoutCancel(ec.ctx), host)

	// Documents without hooks can verify Alloy checks in one batch. Documents
	// with hooks defer each Alloy check until its setup scope has succeeded.
	var precomputed map[string]core.CaseResult
	lazyAlloy := len(plan.Hooks) > 0
	if !lazyAlloy {
		modelResults, modelErr := ec.alloyRunner.RunDocument(ec.ctx, plan)
		if modelErr != nil {
			return core.DocumentResult{}, modelErr
		}
		precomputed = indexResultsByKey(modelResults)
	}

	cases, lifecycleEvents, err := ec.runDocumentCases(plan, sm, cleanupSessions, precomputed, lazyAlloy)
	hitLimit := errors.Is(err, errMaxFailures)
	result := core.DocumentResult{
		Document:        plan.Document,
		Status:          documentStatus(cases, lifecycleEvents),
		Cases:           cases,
		LifecycleEvents: lifecycleEvents,
	}
	if err != nil && !hitLimit && result.Status == core.StatusPassed {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			result.Status = core.StatusSkipped
		} else {
			result.Status = core.StatusFailed
		}
	}

	if closeErr := sm.CloseAll(); closeErr != nil {
		if err == nil && !hitLimit {
			return result, closeErr
		}
		fmt.Fprintf(os.Stderr, "warning: closing adapter sessions: %v\n", closeErr)
	}
	if closeErr := cleanupSessions.CloseAll(); closeErr != nil {
		if err == nil && !hitLimit {
			return result, closeErr
		}
		fmt.Fprintf(os.Stderr, "warning: closing cleanup adapter sessions: %v\n", closeErr)
	}

	if hitLimit {
		return result, errMaxFailures
	}
	return result, err
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

func (ec *executionContext) runDocumentCases(
	plan core.DocumentPlan,
	sm *sessionManager,
	cleanupSessions *sessionManager,
	precomputed map[string]core.CaseResult,
	lazyAlloy bool,
) ([]core.CaseResult, []core.LifecycleEvent, error) {
	timeout := plan.Document.Frontmatter.Timeout
	if timeout == 0 {
		timeout = ec.defaultTimeout
	}
	ctx := &caseRunContext{
		ctx:             ec.ctx,
		registry:        ec.registry,
		sessions:        sm,
		cleanupSessions: cleanupSessions,
		bindings:        newBindingsManager(),
		timeoutMs:       timeout,
		hooks:           plan.Hooks,
		results:         make([]core.CaseResult, 0, len(plan.Cases)),
		lifecycleEvents: make([]core.LifecycleEvent, 0, len(plan.Hooks)),
		activeTeardowns: make(map[string]struct{}),
		spec:            plan.Document.RelativeTo,
		progress:        ec.progress,
		maxFailures:     ec.maxFailures,
		failures:        ec.failures,
		casesTotal:      ec.casesTotal,
		caseCounter:     ec.caseCounter,
		precomputed:     precomputed,
		alloyRunner:     ec.alloyRunner,
		document:        plan.Document,
		alloyModels:     plan.AlloyModels,
		lazyAlloy:       lazyAlloy,
		limitHit:        ec.limitHit,
		limitReached:    ec.limitReached,
		limitOnce:       ec.limitOnce,
	}

	for i := range plan.Cases {
		if err := ec.ctx.Err(); err != nil {
			ctx.runTeardownsAfterCancellation(ctx.prevPath)
			return ctx.results, ctx.lifecycleEvents, err
		}
		nextPath := peekNextPath(plan.Cases, i)
		if err := ctx.processCase(plan.Cases[i], nextPath); err != nil {
			if errors.Is(err, errMaxFailures) {
				return ctx.results, ctx.lifecycleEvents, err
			}
			return ctx.results, ctx.lifecycleEvents, err
		}
	}
	if err := ec.ctx.Err(); err != nil {
		ctx.runTeardownsAfterCancellation(ctx.prevPath)
		return ctx.results, ctx.lifecycleEvents, err
	}
	return ctx.results, ctx.lifecycleEvents, nil
}

type caseRunContext struct {
	ctx             context.Context
	registry        adapterRegistry
	sessions        *sessionManager
	cleanupSessions *sessionManager
	bindings        *bindingsManager
	timeoutMs       int
	hooks           []core.HookSpec
	results         []core.CaseResult
	lifecycleEvents []core.LifecycleEvent
	blockedScopes   []core.HeadingPath
	activeTeardowns map[string]struct{}
	prevPath        core.HeadingPath
	spec            string
	progress        ProgressFunc
	maxFailures     int
	failures        *atomic.Int32
	casesTotal      int
	caseCounter     *atomic.Int32
	precomputed     map[string]core.CaseResult
	alloyRunner     core.ModelRunner
	document        core.Document
	alloyModels     []core.AlloyModelSpec
	lazyAlloy       bool
	limitHit        *atomic.Bool
	limitReached    chan struct{}
	limitOnce       *sync.Once
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
		var caseNum int
		if c.caseCounter != nil {
			caseNum = int(c.caseCounter.Add(1))
		}
		c.progress(ProgressEvent{Kind: "case", Spec: c.spec, Case: &result, CaseNum: caseNum, CasesTotal: c.casesTotal})
	}
	if result.Status == core.StatusFailed && !result.ExpectFail &&
		c.maxFailures > 0 && c.failures != nil {
		if int(c.failures.Add(1)) >= c.maxFailures {
			if c.limitHit != nil {
				c.limitHit.Store(true)
			}
			if c.limitOnce != nil && c.limitReached != nil {
				c.limitOnce.Do(func() {
					close(c.limitReached)
				})
			}
			return errMaxFailures
		}
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
//
//nolint:gocognit // setup and teardown have symmetric validation and report paths
func runOnlyLifecycle(ctx context.Context, baseDir string, cfg config.Config, opts RunOptions) (core.Report, error) {
	var events []core.LifecycleEvent
	if opts.OnlySetup {
		if cfg.Setup == "" {
			return core.Report{}, fmt.Errorf("no setup command configured in specdown.json")
		}
		event, setupErr := runGlobalLifecycle(ctx, baseDir, core.HookSetup, cfg.Setup)
		events = append(events, event)
		if event.Status == core.StatusFailed {
			report := lifecycleOnlyReport(events)
			if ctx.Err() != nil {
				return report, fmt.Errorf("setup command failed: %w", setupErr)
			}
			return report, nil
		}
	}
	if opts.OnlyTeardown {
		if cfg.Teardown == "" {
			return core.Report{}, fmt.Errorf("no teardown command configured in specdown.json")
		}
		cleanupCtx, cancel := lifecycleCleanupContext(ctx, cfg.EffectiveDefaultTimeout())
		event, teardownErr := runGlobalLifecycle(cleanupCtx, baseDir, core.HookTeardown, cfg.Teardown)
		cancel()
		events = append(events, event)
		if event.Status == core.StatusFailed {
			report := lifecycleOnlyReport(events)
			if ctx.Err() != nil {
				return report, fmt.Errorf("teardown command failed: %w", teardownErr)
			}
			return report, nil
		}
		if ctx.Err() != nil {
			return lifecycleOnlyReport(events), ctx.Err()
		}
	}
	return lifecycleOnlyReport(events), nil
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
