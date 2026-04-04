package engine

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/corca-ai/specdown/internal/specdown/adapterhost"
	"github.com/corca-ai/specdown/internal/specdown/config"
	"github.com/corca-ai/specdown/internal/specdown/core"
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
	Filter      string
	Jobs        int
	DryRun      bool
	Progress    ProgressFunc
	MaxFailures int // 0 means unlimited
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
	registry       adapterRegistry
	host           adapterhost.Host
	alloyRunner    core.ModelRunner
	defaultTimeout int
	progress       ProgressFunc
	maxFailures    int
	failures       *atomic.Int32
	casesTotal     int
	caseCounter    *atomic.Int32
}

func runShellCommand(baseDir, command string) error {
	shell, flag := "sh", "-c"
	if runtime.GOOS == "windows" {
		shell, flag = "cmd", "/C"
	}
	cmd := exec.Command(shell, flag, command)
	cmd.Dir = baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

//nolint:gocognit // top-level orchestration with setup/teardown/trace phases
func Run(baseDir string, cfg config.Config, modelRunner core.ModelRunner, opts RunOptions) (core.Report, error) {
	if opts.OnlySetup || opts.OnlyTeardown {
		return runOnlyLifecycle(baseDir, cfg, opts)
	}

	if cfg.Setup != "" && !opts.NoSetup {
		if err := runShellCommand(baseDir, cfg.Setup); err != nil {
			return core.Report{}, fmt.Errorf("setup command failed: %w", err)
		}
	}
	if cfg.Teardown != "" && !opts.NoTeardown {
		defer func() {
			if terr := runShellCommand(baseDir, cfg.Teardown); terr != nil {
				fmt.Fprintf(os.Stderr, "warning: teardown command failed: %v\n", terr)
			}
		}()
	}

	title, docs, err := core.DiscoverFromEntry(baseDir, cfg.Entry, cfg.IgnorePrefixes)
	if err != nil {
		return core.Report{}, err
	}
	host := adapterhost.Host{BaseDir: baseDir}
	defaultTimeout := cfg.EffectiveDefaultTimeout()
	progress := opts.Progress
	if progress == nil {
		progress = func(ProgressEvent) {}
	}
	report, err := runWithDocs(title, docs, cfg, host, modelRunner, opts, defaultTimeout, progress)
	if err != nil {
		return core.Report{}, err
	}

	// Run trace validation when trace is configured
	if cfg.Trace != nil {
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

	return report, nil
}

// ModelDumper can write model artifacts without running verification.
type ModelDumper interface {
	DumpModels(plan core.DocumentPlan) ([]string, error)
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

func runWithDocs(title string, docs []core.Document, cfg config.Config, host adapterhost.Host, alloyRunner core.ModelRunner, opts RunOptions, defaultTimeout int, progress ProgressFunc) (core.Report, error) {
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
	ec := &executionContext{
		registry:       registry,
		host:           host,
		alloyRunner:    alloyRunner,
		defaultTimeout: defaultTimeout,
		progress:       progress,
		maxFailures:    opts.MaxFailures,
		failures:       &failures,
		casesTotal:     casesTotal,
		caseCounter:    &caseCounter,
	}
	results, err := ec.executeDocuments(plan.Documents, jobs)
	hitLimit := errors.Is(err, errMaxFailures)
	if err != nil && !hitLimit {
		return core.Report{}, err
	}

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

	return core.Report{
		SchemaVersion: 2,
		Title:         title,
		GeneratedAt:   time.Now(),
		Results:       results,
		Summary:       summary,
	}, nil
}

func (ec *executionContext) executeDocuments(documents []core.DocumentPlan, jobs int) ([]core.DocumentResult, error) {
	results := make([]core.DocumentResult, len(documents))
	if jobs == 1 {
		for i := range documents {
			result, err := ec.runDocument(documents[i])
			if errors.Is(err, errMaxFailures) {
				results[i] = result
				return results, err
			}
			if err != nil {
				return nil, err
			}
			results[i] = result
		}
		return results, nil
	}

	errs := make([]error, len(documents))
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup
	for i := range documents {
		wg.Add(1)
		go func(i int, dp core.DocumentPlan) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			result, err := ec.runDocument(dp)
			results[i] = result
			errs[i] = err
		}(i, documents[i])
	}
	wg.Wait()
	for _, err := range errs {
		if errors.Is(err, errMaxFailures) {
			return results, err
		}
		if err != nil {
			return nil, err
		}
	}
	return results, nil
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
				Document:    plan.Documents[i].Document,
				Cases:       cases,
				Hooks:       plan.Documents[i].Hooks,
				AlloyModels: plan.Documents[i].AlloyModels,
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
		SchemaVersion: 2,
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

func (ec *executionContext) runDocument(plan core.DocumentPlan) (core.DocumentResult, error) {
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
	sm := newSessionManager(host)

	// Pre-compute model verification results via ModelRunner before the case loop.
	modelResults, modelErr := ec.alloyRunner.RunDocument(plan)
	if modelErr != nil {
		return core.DocumentResult{}, modelErr
	}
	precomputed := indexResultsByKey(modelResults)

	cases, err := ec.runDocumentCases(plan, sm, precomputed)
	hitLimit := errors.Is(err, errMaxFailures)
	if err != nil && !hitLimit {
		if closeErr := sm.CloseAll(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: closing adapter sessions: %v\n", closeErr)
		}
		return core.DocumentResult{}, err
	}

	if closeErr := sm.CloseAll(); closeErr != nil {
		if !hitLimit {
			return core.DocumentResult{}, closeErr
		}
		fmt.Fprintf(os.Stderr, "warning: closing adapter sessions: %v\n", closeErr)
	}

	result := core.DocumentResult{
		Document: plan.Document,
		Status:   documentStatus(cases),
		Cases:    cases,
	}
	if hitLimit {
		return result, errMaxFailures
	}
	return result, nil
}

// indexResultsByKey builds a lookup map from model runner results.
func indexResultsByKey(results []core.CaseResult) map[string]core.CaseResult {
	m := make(map[string]core.CaseResult, len(results))
	for i := range results {
		m[results[i].ID.Key()] = results[i]
	}
	return m
}

// documentStatus derives the overall document status from case results.
func documentStatus(cases []core.CaseResult) core.Status {
	for i := range cases {
		if cases[i].Status == core.StatusFailed && !cases[i].ExpectFail {
			return core.StatusFailed
		}
	}
	return core.StatusPassed
}

func (ec *executionContext) runDocumentCases(plan core.DocumentPlan, sm *sessionManager, precomputed map[string]core.CaseResult) ([]core.CaseResult, error) {
	timeout := plan.Document.Frontmatter.Timeout
	if timeout == 0 {
		timeout = ec.defaultTimeout
	}
	ctx := &caseRunContext{
		registry:    ec.registry,
		sessions:    sm,
		bindings:    newBindingsManager(),
		timeoutMs:   timeout,
		hooks:       plan.Hooks,
		results:     make([]core.CaseResult, 0, len(plan.Cases)),
		spec:        plan.Document.RelativeTo,
		progress:    ec.progress,
		maxFailures: ec.maxFailures,
		failures:    ec.failures,
		casesTotal:  ec.casesTotal,
		caseCounter: ec.caseCounter,
		precomputed: precomputed,
	}

	for i := range plan.Cases {
		nextPath := peekNextPath(plan.Cases, i)
		if err := ctx.processCase(plan.Cases[i], nextPath); err != nil {
			if errors.Is(err, errMaxFailures) {
				return ctx.results, err
			}
			return nil, err
		}
	}
	return ctx.results, nil
}

type caseRunContext struct {
	registry    adapterRegistry
	sessions    *sessionManager
	bindings    *bindingsManager
	timeoutMs   int
	hooks       []core.HookSpec
	results     []core.CaseResult
	prevPath    core.HeadingPath
	spec        string
	progress    ProgressFunc
	maxFailures int
	failures    *atomic.Int32
	casesTotal  int
	caseCounter *atomic.Int32
	precomputed map[string]core.CaseResult
}

// processCase handles a single case: hooks, execution, result recording.
func (c *caseRunContext) processCase(specCase core.CaseSpec, nextPath core.HeadingPath) error {
	currPath := specCase.ID.HeadingPath

	if specCase.Kind == core.CaseKindAlloy {
		result, ok := c.precomputed[specCase.ID.Key()]
		if !ok {
			result = core.CaseResult{
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
			}
		}
		if err := c.recordResult(result, specCase.ID.HeadingPath); err != nil {
			return err
		}
		c.prevPath = currPath
		return nil
	}

	prevPath := c.prevPath
	c.runHooksMatching(core.HookSetup, func(h core.HookSpec) bool {
		return shouldRunHook(h, prevPath, currPath)
	})

	result, err := runSingleCase(specCase, c.registry, c.sessions, c.bindings.VisibleAt(specCase.ID.HeadingPath), c.timeoutMs)
	if err != nil {
		return err
	}

	if err := c.recordResult(result, specCase.ID.HeadingPath); err != nil {
		return err
	}

	c.runHooksMatching(core.HookTeardown, func(h core.HookSpec) bool {
		return shouldRunTeardownHook(h, currPath, nextPath)
	})

	c.prevPath = currPath
	return nil
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
func runOnlyLifecycle(baseDir string, cfg config.Config, opts RunOptions) (core.Report, error) {
	if opts.OnlySetup {
		if cfg.Setup == "" {
			return core.Report{}, fmt.Errorf("no setup command configured in specdown.json")
		}
		if err := runShellCommand(baseDir, cfg.Setup); err != nil {
			return core.Report{}, fmt.Errorf("setup command failed: %w", err)
		}
	}
	if opts.OnlyTeardown {
		if cfg.Teardown == "" {
			return core.Report{}, fmt.Errorf("no teardown command configured in specdown.json")
		}
		if err := runShellCommand(baseDir, cfg.Teardown); err != nil {
			return core.Report{}, fmt.Errorf("teardown command failed: %w", err)
		}
	}
	return core.Report{GeneratedAt: time.Now()}, nil
}

func accumulateSummary(summary *core.Summary, result core.DocumentResult) {
	if result.Status == core.StatusPassed {
		summary.SpecsPassed++
	} else {
		summary.SpecsFailed++
	}

	summary.CasesTotal += len(result.Cases)
	for i := range result.Cases {
		switch {
		case result.Cases[i].Status == core.StatusPassed:
			summary.CasesPassed++
		case result.Cases[i].ExpectFail:
			summary.CasesExpectedFail++
		default:
			summary.CasesFailed++
		}
	}
}
