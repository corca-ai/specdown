package engine

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/corca-ai/specdown/internal/specdown/adapterhost"
	"github.com/corca-ai/specdown/internal/specdown/adapterprotocol"
	"github.com/corca-ai/specdown/internal/specdown/alloy"
	"github.com/corca-ai/specdown/internal/specdown/config"
	"github.com/corca-ai/specdown/internal/specdown/core"
	"github.com/corca-ai/specdown/internal/specdown/shelladapter"
	"github.com/corca-ai/specdown/internal/specdown/trace"
)

// adapterEntry holds an adapter config for registry lookups.
type adapterEntry struct {
	Config config.AdapterConfig
}

type RunOptions struct {
	Filter string
	Jobs   int
	DryRun bool
}

type adapterRegistry struct {
	blocks   map[string]adapterEntry
	checks map[string]adapterEntry
}

func Run(baseDir string, cfg config.Config, opts RunOptions) (core.Report, error) {
	title, docs, err := core.DiscoverFromEntry(baseDir, cfg.Entry, cfg.IgnorePrefixes)
	if err != nil {
		return core.Report{}, err
	}
	host := adapterhost.Host{BaseDir: baseDir}
	report, err := runWithDocs(title, docs, cfg, host, alloy.Runner{BaseDir: baseDir}, opts)
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

func DumpAlloyModels(baseDir string, cfg config.Config) ([]string, error) {
	_, docs, err := core.DiscoverFromEntry(baseDir, cfg.Entry, cfg.IgnorePrefixes)
	if err != nil {
		return nil, err
	}

	plan, err := core.CompileDocuments(docs)
	if err != nil {
		return nil, err
	}

	runner := alloy.Runner{BaseDir: baseDir}
	var paths []string
	for _, docPlan := range plan.Documents {
		dumped, err := runner.DumpModels(docPlan)
		if err != nil {
			return nil, err
		}
		paths = append(paths, dumped...)
	}
	return paths, nil
}

func runWithDocs(title string, docs []core.Document, cfg config.Config, host adapterhost.Host, alloyRunner alloy.DocumentRunner, opts RunOptions) (core.Report, error) {
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

	results, err := executeDocuments(plan.Documents, jobs, registry, host, alloyRunner)
	if err != nil {
		return core.Report{}, err
	}

	summary := core.Summary{SpecsTotal: len(plan.Documents)}
	for _, result := range results {
		accumulateSummary(&summary, result)
	}

	return core.Report{
		Title:       title,
		GeneratedAt: time.Now(),
		Results:     results,
		Summary:     summary,
	}, nil
}

func executeDocuments(documents []core.DocumentPlan, jobs int, registry adapterRegistry, host adapterhost.Host, alloyRunner alloy.DocumentRunner) ([]core.DocumentResult, error) {
	results := make([]core.DocumentResult, len(documents))
	if jobs == 1 {
		for i, docPlan := range documents {
			result, err := runDocument(docPlan, registry, host, alloyRunner)
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
	for i, docPlan := range documents {
		wg.Add(1)
		go func(i int, dp core.DocumentPlan) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			result, err := runDocument(dp, registry, host, alloyRunner)
			results[i] = result
			errs[i] = err
		}(i, docPlan)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

func filterPlan(plan core.Plan, filter string) core.Plan {
	f := parseFilter(filter)
	var filtered []core.DocumentPlan
	for _, doc := range plan.Documents {
		var cases []core.CaseSpec
		for _, c := range doc.Cases {
			if f.matches(c) {
				cases = append(cases, c)
			}
		}
		if len(cases) > 0 {
			filtered = append(filtered, core.DocumentPlan{
				Document:    doc.Document,
				Cases:       cases,
				AlloyModels: doc.AlloyModels,
			})
		}
	}
	return core.Plan{Documents: filtered}
}

func dryRunReport(plan core.Plan) core.Report {
	results := make([]core.DocumentResult, 0, len(plan.Documents))
	summary := core.Summary{SpecsTotal: len(plan.Documents)}

	for _, doc := range plan.Documents {
		cases := make([]core.CaseResult, 0, len(doc.Cases))
		for _, c := range doc.Cases {
			cr := core.CaseResult{
				ID:        c.ID,
				Kind:      c.Kind,
				Block:     c.Block.Descriptor(),
				Check:     c.Check,
				Label:     dryRunLabel(c),
				Columns:   append([]string(nil), c.Columns...),
				RowNumber: c.RowNumber,
			}
			if c.Kind == core.CaseKindAlloy {
				cr.Model = c.Model
				cr.Assertion = c.Assertion
				cr.Scope = c.Scope
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
		GeneratedAt: time.Now(),
		Results:     results,
		Summary:     summary,
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
		blocks:   make(map[string]adapterEntry),
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

	return registry, nil
}

func (r adapterRegistry) adapterFor(specCase core.CaseSpec) (adapterEntry, error) {
	switch specCase.Kind {
	case core.CaseKindCode:
		entry, ok := r.blocks[specCase.Block.Descriptor()]
		if !ok {
			return adapterEntry{}, fmt.Errorf("no adapter supports block %q in %s", specCase.Block.Descriptor(), specCase.ID.Key())
		}
		return entry, nil
	case core.CaseKindTableRow:
		entry, ok := r.checks[specCase.Check]
		if !ok {
			return adapterEntry{}, fmt.Errorf("no adapter supports check %q in %s", specCase.Check, specCase.ID.Key())
		}
		return entry, nil
	default:
		return adapterEntry{}, fmt.Errorf("unsupported case kind %q", specCase.Kind)
	}
}

func runDocument(plan core.DocumentPlan, registry adapterRegistry, host adapterhost.Host, alloyRunner alloy.DocumentRunner) (core.DocumentResult, error) {
	if len(plan.Cases) == 0 {
		return core.DocumentResult{
			Document: plan.Document,
			Status:   core.StatusPassed,
		}, nil
	}

	sm := newSessionManager(host)
	defer func() { _ = sm.CloseAll() }()

	cases, status, err := runDocumentCases(plan, registry, sm)
	if err != nil {
		return core.DocumentResult{}, err
	}

	if err := sm.CloseAll(); err != nil {
		return core.DocumentResult{}, err
	}

	alloyResults, err := alloyRunner.RunDocument(plan)
	if err != nil {
		return core.DocumentResult{}, err
	}

	cases, failed := mergeAlloyResults(cases, alloyResults)
	if failed {
		status = core.StatusFailed
	}

	return core.DocumentResult{
		Document: plan.Document,
		Status:   status,
		Cases:    cases,
	}, nil
}

// mergeAlloyResults replaces placeholder alloy cases with actual results
// and reports whether any alloy check failed.
func mergeAlloyResults(cases []core.CaseResult, alloyResults []core.CaseResult) ([]core.CaseResult, bool) {
	if len(alloyResults) == 0 {
		return cases, false
	}
	alloyByKey := make(map[string]core.CaseResult, len(alloyResults))
	for _, r := range alloyResults {
		alloyByKey[r.ID.Key()] = r
	}
	for i, c := range cases {
		if c.Kind == core.CaseKindAlloy {
			if ar, ok := alloyByKey[c.ID.Key()]; ok {
				cases[i] = ar
			}
		}
	}
	failed := false
	for _, r := range alloyResults {
		if r.Status == core.StatusFailed {
			failed = true
			break
		}
	}
	return cases, failed
}

func runDocumentCases(plan core.DocumentPlan, registry adapterRegistry, sm *sessionManager) ([]core.CaseResult, core.Status, error) {
	ctx := &caseRunContext{
		registry:  registry,
		sessions:  sm,
		bindings:  newBindingsManager(),
		timeoutMs: plan.Document.Frontmatter.Timeout,
		hooks:     plan.Hooks,
	}
	cases := make([]core.CaseResult, 0, len(plan.Cases))
	status := core.StatusPassed

	var prevPath core.HeadingPath
	for i, specCase := range plan.Cases {
		currPath := specCase.ID.HeadingPath

		// Alloy cases are handled by the alloy runner; produce placeholder results.
		if specCase.Kind == core.CaseKindAlloy {
			cases = append(cases, core.CaseResult{
				ID:        specCase.ID,
				Kind:      core.CaseKindAlloy,
				Model:     specCase.Model,
				Assertion: specCase.Assertion,
				Scope:     specCase.Scope,
				Label:     specCase.DefaultLabel(),
			})
			prevPath = currPath
			continue
		}

		if failed := ctx.runSetupHooks(prevPath, currPath); failed {
			status = core.StatusFailed
		}

		result, err := runSingleCase(specCase, ctx.registry, ctx.sessions, ctx.bindings.VisibleAt(specCase.ID.HeadingPath), ctx.timeoutMs)
		if err != nil {
			return nil, "", err
		}
		cases = append(cases, result)
		if result.Status == core.StatusFailed && !result.ExpectFail {
			status = core.StatusFailed
		} else if result.Status != core.StatusFailed {
			ctx.bindings.Add(result.Bindings, specCase.ID.HeadingPath)
		}

		var nextPath core.HeadingPath
		if i+1 < len(plan.Cases) {
			nextPath = plan.Cases[i+1].ID.HeadingPath
		}
		if failed := ctx.runTeardownHooks(currPath, nextPath); failed {
			status = core.StatusFailed
		}

		prevPath = currPath
	}
	return cases, status, nil
}

type caseRunContext struct {
	registry  adapterRegistry
	sessions  *sessionManager
	bindings  *bindingsManager
	timeoutMs int
	hooks     []core.HookSpec
}

func (c *caseRunContext) runSetupHooks(prevPath, currPath core.HeadingPath) bool {
	failed := false
	for _, hook := range c.hooks {
		if hook.Kind != core.HookSetup || !shouldRunHook(hook, prevPath, currPath) {
			continue
		}
		visible := c.bindings.VisibleAt(hook.HeadingPath)
		if err := runHook(hook, c.registry, c.sessions, visible, c.timeoutMs); err != nil {
			failed = true
		}
	}
	return failed
}

func (c *caseRunContext) runTeardownHooks(currPath, nextPath core.HeadingPath) bool {
	failed := false
	for _, hook := range c.hooks {
		if hook.Kind != core.HookTeardown || !shouldRunTeardownHook(hook, currPath, nextPath) {
			continue
		}
		visible := c.bindings.VisibleAt(hook.HeadingPath)
		if err := runHook(hook, c.registry, c.sessions, visible, c.timeoutMs); err != nil {
			failed = true
		}
	}
	return failed
}

func shouldRunHook(hook core.HookSpec, prevPath, currPath core.HeadingPath) bool {
	if !hook.HeadingPath.IsPrefix(currPath) {
		return false
	}
	if !hook.Each {
		return !hook.HeadingPath.IsPrefix(prevPath)
	}
	depth := len(hook.HeadingPath)
	if len(currPath) <= depth {
		return false
	}
	if !hook.HeadingPath.IsPrefix(prevPath) || len(prevPath) <= depth {
		return true
	}
	return currPath[depth] != prevPath[depth]
}

func shouldRunTeardownHook(hook core.HookSpec, currPath, nextPath core.HeadingPath) bool {
	if !hook.HeadingPath.IsPrefix(currPath) {
		return false
	}
	if !hook.Each {
		return !hook.HeadingPath.IsPrefix(nextPath)
	}
	depth := len(hook.HeadingPath)
	if len(currPath) <= depth {
		return false
	}
	if !hook.HeadingPath.IsPrefix(nextPath) || len(nextPath) <= depth {
		return true
	}
	return currPath[depth] != nextPath[depth]
}

func runHook(hook core.HookSpec, registry adapterRegistry, sm *sessionManager, visible []core.Binding, timeoutMs int) error {
	synthetic := core.CaseSpec{
		ID: core.SpecID{
			File:        "_hook",
			HeadingPath: hook.HeadingPath,
		},
		Kind:     core.CaseKindCode,
		Block:    hook.Block,
		Template: hook.Source,
	}

	adapter, err := registry.adapterFor(synthetic)
	if err != nil {
		return err
	}

	prepared, err := prepareCase(synthetic, visible)
	if err != nil {
		return err
	}

	session, err := sm.For(adapter.Config)
	if err != nil {
		return err
	}

	resp, err := session.Exec(synthetic.ID.Ordinal, prepared.Template, timeoutMs)
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf("%s hook failed: %s", hook.Kind, resp.Error)
	}
	return nil
}

func runSingleCase(specCase core.CaseSpec, registry adapterRegistry, sm *sessionManager, visible []core.Binding, timeoutMs int) (core.CaseResult, error) {
	start := time.Now()

	if specCase.Kind == core.CaseKindInlineExpect {
		prepared, err := prepareCase(specCase, visible)
		if err != nil {
			return variableFailure(specCase, err), nil
		}
		result := runInlineExpect(prepared, visible)
		if specCase.ExpectFail {
			result = applyExpectFail(result)
		}
		result.DurationMs = int(time.Since(start).Milliseconds())
		return result, nil
	}

	adapter, err := registry.adapterFor(specCase)
	if err != nil {
		return core.CaseResult{}, err
	}

	prepared, err := prepareCase(specCase, visible)
	if err != nil {
		return variableFailure(specCase, err), nil
	}

	session, err := sm.For(adapter.Config)
	if err != nil {
		return core.CaseResult{}, err
	}

	var result core.CaseResult
	switch specCase.Kind {
	case core.CaseKindCode:
		result, err = runCodeCase(specCase, prepared, session, timeoutMs)
	case core.CaseKindTableRow:
		result, err = runTableRowCase(specCase, prepared, session, timeoutMs)
	default:
		return core.CaseResult{}, fmt.Errorf("unsupported case kind %q", specCase.Kind)
	}
	if err != nil {
		return result, err
	}
	result.VisibleBindings = visible

	if specCase.Block.ExpectFail {
		result = applyExpectFail(result)
	}

	result.DurationMs = int(time.Since(start).Milliseconds())
	return result, nil
}

func runCodeCase(specCase core.CaseSpec, prepared core.CaseSpec, session *adapterhost.Session, timeoutMs int) (core.CaseResult, error) {
	result := core.CaseResult{
		ID:             specCase.ID,
		Kind:           specCase.Kind,
		Block:          specCase.Block.Descriptor(),
		Label:          specCase.DefaultLabel(),
		Template:       specCase.Template,
		RenderedSource: prepared.Template,
	}

	result.Events = append(result.Events, core.Event{
		Type:  core.EventCaseStarted,
		ID:    result.ID,
		Label: result.Label,
	})

	if shelladapter.IsDoctestContent(prepared.Template) {
		return runDoctestCase(specCase, prepared, session, result, timeoutMs)
	}

	resp, err := session.Exec(specCase.ID.Ordinal, prepared.Template, timeoutMs)
	if err != nil {
		return result, err
	}

	if resp.Error != "" {
		result.Status = core.StatusFailed
		result.Message = resp.Error
		result.Events = append(result.Events, core.Event{
			Type:    core.EventCaseFailed,
			ID:      result.ID,
			Label:   result.Label,
			Message: resp.Error,
		})
		return result, nil
	}

	result.Status = core.StatusPassed

	// Extract captures from output
	if resp.HasOutput && len(specCase.Block.CaptureNames) > 0 {
		result.Bindings = captureBindings(resp.Output, specCase.Block.CaptureNames)
	}

	result.Events = append(result.Events, core.Event{
		Type:     core.EventCasePassed,
		ID:       result.ID,
		Label:    result.Label,
		Bindings: result.Bindings,
	})

	return result, nil
}

func runDoctestCase(specCase core.CaseSpec, prepared core.CaseSpec, session *adapterhost.Session, result core.CaseResult, timeoutMs int) (core.CaseResult, error) {
	steps := shelladapter.ParseDoctestSource(prepared.Template)
	result.Status = core.StatusPassed

	for _, step := range steps {
		resp, err := session.Exec(specCase.ID.Ordinal, step.Command, timeoutMs)
		if err != nil {
			return result, err
		}

		actual, stepStatus := evalDoctestStep(resp, step.Expected)
		result.Steps = append(result.Steps, core.DoctestStep{
			Command:  step.Command,
			Expected: step.Expected,
			Actual:   actual,
			Status:   stepStatus,
		})

		if stepStatus == core.StatusFailed {
			result.Status = core.StatusFailed
		}
	}

	if result.Status == core.StatusFailed {
		result.Events = append(result.Events, core.Event{
			Type:  core.EventCaseFailed,
			ID:    result.ID,
			Label: result.Label,
		})
	} else {
		result.Events = append(result.Events, core.Event{
			Type:  core.EventCasePassed,
			ID:    result.ID,
			Label: result.Label,
		})
	}

	return result, nil
}

func evalDoctestStep(resp adapterprotocol.ExecResponse, expected string) (string, core.Status) {
	switch {
	case resp.Error != "":
		if expected == "" || !shelladapter.MatchWithWildcard(resp.Error, expected) {
			return resp.Error, core.StatusFailed
		}
		return resp.Error, core.StatusPassed
	case resp.HasOutput:
		actual := shelladapter.ExecResponseToString(resp.Output)
		if expected != "" && !shelladapter.MatchWithWildcard(actual, expected) {
			return actual, core.StatusFailed
		}
		return actual, core.StatusPassed
	default:
		if expected != "" {
			return "", core.StatusFailed
		}
		return "", core.StatusPassed
	}
}

func runTableRowCase(specCase core.CaseSpec, prepared core.CaseSpec, session *adapterhost.Session, timeoutMs int) (core.CaseResult, error) {
	result := core.CaseResult{
		ID:            specCase.ID,
		Kind:          specCase.Kind,
		Check:         specCase.Check,
		Label:         specCase.DefaultLabel(),
		Columns:       append([]string(nil), specCase.Columns...),
		TemplateCells: append([]string(nil), specCase.Cells...),
		RenderedCells: append([]string(nil), prepared.Cells...),
		RowNumber:     specCase.RowNumber,
	}

	result.Events = append(result.Events, core.Event{
		Type:  core.EventCaseStarted,
		ID:    result.ID,
		Label: result.Label,
	})

	resp, err := session.Assert(specCase.ID.Ordinal, prepared.Check, prepared.CheckParams, prepared.Columns, prepared.Cells, timeoutMs)
	if err != nil {
		return result, err
	}

	switch resp.Type {
	case "passed":
		result.Status = core.StatusPassed
		if resp.Actual != "" {
			result.Actual = resp.Actual
		}
		result.Events = append(result.Events, core.Event{
			Type:  core.EventCasePassed,
			ID:    result.ID,
			Label: result.Label,
		})
	case "failed":
		result.Status = core.StatusFailed
		result.Message = resp.Message
		result.Expected = resp.Expected
		result.Actual = resp.Actual
		if resp.Label != "" {
			result.Label = resp.Label
		}
		result.Events = append(result.Events, core.Event{
			Type:     core.EventCaseFailed,
			ID:       result.ID,
			Label:    result.Label,
			Message:  resp.Message,
			Expected: resp.Expected,
			Actual:   resp.Actual,
		})
	default:
		return result, fmt.Errorf("unexpected assert response type %q", resp.Type)
	}

	return result, nil
}

func captureBindings(rawOutput json.RawMessage, captureNames []string) []core.Binding {
	// Try to parse as string first
	var strValue string
	if err := json.Unmarshal(rawOutput, &strValue); err == nil {
		// String output — split by newlines for captures
		lines := strings.Split(strValue, "\n")
		var bindings []core.Binding
		for i, name := range captureNames {
			var value any = ""
			if i < len(lines) {
				value = lines[i]
			}
			bindings = append(bindings, core.Binding{Name: name, Value: value})
		}
		return bindings
	}

	// Non-string output (object, array, number, etc.) — store as structured value
	if len(captureNames) == 1 {
		var parsed interface{}
		if err := json.Unmarshal(rawOutput, &parsed); err == nil {
			return []core.Binding{{Name: captureNames[0], Value: parsed}}
		}
	}

	// Fallback: store raw JSON string
	var bindings []core.Binding
	for i, name := range captureNames {
		if i == 0 {
			bindings = append(bindings, core.Binding{Name: name, Value: string(rawOutput)})
		} else {
			bindings = append(bindings, core.Binding{Name: name, Value: ""})
		}
	}
	return bindings
}

func runInlineExpect(prepared core.CaseSpec, visible []core.Binding) core.CaseResult {
	result := core.CaseResult{
		ID:              prepared.ID,
		Kind:            prepared.Kind,
		Label:           prepared.DefaultLabel(),
		Expected:        prepared.ExpectValue,
		Actual:          prepared.Template,
		VisibleBindings: visible,
	}

	result.Events = append(result.Events, core.Event{
		Type:  core.EventCaseStarted,
		ID:    result.ID,
		Label: result.Label,
	})

	if prepared.Template == prepared.ExpectValue {
		result.Status = core.StatusPassed
		result.Events = append(result.Events, core.Event{
			Type:  core.EventCasePassed,
			ID:    result.ID,
			Label: result.Label,
		})
	} else {
		result.Status = core.StatusFailed
		result.Message = fmt.Sprintf("expected %q, got %q", prepared.ExpectValue, prepared.Template)
		result.Events = append(result.Events, core.Event{
			Type:     core.EventCaseFailed,
			ID:       result.ID,
			Label:    result.Label,
			Message:  result.Message,
			Expected: result.Expected,
			Actual:   result.Actual,
		})
	}
	return result
}

func applyExpectFail(result core.CaseResult) core.CaseResult {
	result.ExpectFail = true
	if result.Status == core.StatusPassed {
		// Unexpected success — this is a real failure
		result.ExpectFail = false
		result.Status = core.StatusFailed
		result.Message = "expected failure but case passed"
		result.Events = []core.Event{
			{Type: core.EventCaseStarted, ID: result.ID, Label: result.Label},
			{Type: core.EventCaseFailed, ID: result.ID, Label: result.Label, Message: result.Message},
		}
	}
	// When failed: keep status as failed, keep all failure details, just mark ExpectFail
	return result
}


var variablePattern = regexp.MustCompile(`(\\?)\$\{([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\}`)

func prepareCase(specCase core.CaseSpec, bindings []core.Binding) (core.CaseSpec, error) {
	prepared := specCase
	switch specCase.Kind {
	case core.CaseKindCode:
		rendered, err := renderTemplate(specCase.Template, bindings)
		if err != nil {
			return core.CaseSpec{}, err
		}
		prepared.Template = rendered
		return prepared, nil
	case core.CaseKindInlineExpect:
		rendered, err := renderTemplate(specCase.Template, bindings)
		if err != nil {
			return core.CaseSpec{}, err
		}
		prepared.Template = rendered
		renderedExpect, err := renderTemplate(specCase.ExpectValue, bindings)
		if err != nil {
			return core.CaseSpec{}, err
		}
		prepared.ExpectValue = renderedExpect
		return prepared, nil
	case core.CaseKindTableRow:
		rendered := make([]string, 0, len(specCase.Cells))
		for _, cell := range specCase.Cells {
			value, err := renderTemplate(cell, bindings)
			if err != nil {
				return core.CaseSpec{}, err
			}
			rendered = append(rendered, core.UnescapeCell(value))
		}
		prepared.Cells = rendered
		return prepared, nil
	default:
		return core.CaseSpec{}, fmt.Errorf("unsupported case kind %q", specCase.Kind)
	}
}

func renderTemplate(tmpl string, bindings []core.Binding) (string, error) {
	values := make(map[string]any, len(bindings))
	for _, binding := range bindings {
		values[binding.Name] = binding.Value
	}

	var unresolved error
	rendered := variablePattern.ReplaceAllStringFunc(tmpl, func(raw string) string {
		match := variablePattern.FindStringSubmatch(raw)
		if len(match) != 3 {
			return raw
		}
		if match[1] == `\` {
			// escaped \${...} → literal ${...}
			return raw[1:]
		}
		ref := match[2]
		parts := strings.SplitN(ref, ".", 2)
		rootName := parts[0]
		rootValue, ok := values[rootName]
		if !ok {
			unresolved = fmt.Errorf("missing runtime binding for %q", rootName)
			return raw
		}
		if len(parts) == 1 {
			return valueToString(rootValue)
		}
		// Dot-path access
		resolved, err := resolveValue(rootValue, strings.Split(parts[1], "."))
		if err != nil {
			unresolved = fmt.Errorf("cannot resolve %q: %w", ref, err)
			return raw
		}
		return valueToString(resolved)
	})
	if unresolved != nil {
		return "", unresolved
	}
	return rendered, nil
}

func resolveValue(value any, path []string) (any, error) {
	current := value
	for _, key := range path {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("cannot access %q on non-object value", key)
		}
		next, exists := m[key]
		if !exists {
			return nil, fmt.Errorf("key %q not found", key)
		}
		current = next
	}
	return current, nil
}

func valueToString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case nil:
		return ""
	default:
		data, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(data)
	}
}

func variableFailure(specCase core.CaseSpec, err error) core.CaseResult {
	result := core.CaseResult{
		ID:        specCase.ID,
		Kind:      specCase.Kind,
		Block:     specCase.Block.Descriptor(),
		Check:     specCase.Check,
		Label:     specCase.DefaultLabel(),
		Columns:   append([]string(nil), specCase.Columns...),
		RowNumber: specCase.RowNumber,
		Status:    core.StatusFailed,
		Message:   err.Error(),
	}

	switch specCase.Kind {
	case core.CaseKindCode:
		result.Template = specCase.Template
		result.RenderedSource = specCase.Template
	case core.CaseKindTableRow:
		result.TemplateCells = append([]string(nil), specCase.Cells...)
		result.RenderedCells = append([]string(nil), specCase.Cells...)
	}

	result.Events = append(result.Events, core.Event{
		Type:    core.EventCaseFailed,
		ID:      specCase.ID,
		Label:   result.Label,
		Message: result.Message,
	})
	return result
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

func accumulateSummary(summary *core.Summary, result core.DocumentResult) {
	if result.Status == core.StatusPassed {
		summary.SpecsPassed++
	} else {
		summary.SpecsFailed++
	}

	summary.CasesTotal += len(result.Cases)
	for _, item := range result.Cases {
		switch {
		case item.Status == core.StatusPassed:
			summary.CasesPassed++
		case item.ExpectFail:
			summary.CasesExpectedFail++
		default:
			summary.CasesFailed++
		}
	}
}
