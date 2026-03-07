package engine

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"specdown/internal/specdown/alloy"
	"specdown/internal/specdown/adapterhost"
	"specdown/internal/specdown/config"
	"specdown/internal/specdown/core"
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
	fixtures map[string]adapterEntry
}

type scopedBinding struct {
	Binding     core.Binding
	HeadingPath []string
	Order       int
}

func Run(baseDir string, cfg config.Config, opts RunOptions) (core.Report, error) {
	host := adapterhost.Host{BaseDir: baseDir}
	return runWithDependencies(baseDir, cfg, host, alloy.Runner{BaseDir: baseDir}, opts)
}

func DumpAlloyModels(baseDir string, cfg config.Config) ([]string, error) {
	docs, err := core.Discover(baseDir, cfg.Include)
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

func runWithDependencies(baseDir string, cfg config.Config, host adapterhost.Host, alloyRunner alloy.DocumentRunner, opts RunOptions) (core.Report, error) {
	docs, err := core.Discover(baseDir, cfg.Include)
	if err != nil {
		return core.Report{}, err
	}
	if len(docs) == 0 {
		return core.Report{}, fmt.Errorf("no .spec.md files matched include patterns")
	}

	plan, err := core.CompileDocuments(docs)
	if err != nil {
		return core.Report{}, err
	}

	if opts.Filter != "" {
		plan = filterPlan(plan, opts.Filter)
	}

	if opts.DryRun {
		return dryRunReport(plan), nil
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
	var filtered []core.DocumentPlan
	for _, doc := range plan.Documents {
		var cases []core.CaseSpec
		for _, c := range doc.Cases {
			path := strings.Join(c.ID.HeadingPath, " > ")
			if strings.Contains(path, filter) {
				cases = append(cases, c)
			}
		}
		var checks []core.AlloyCheckSpec
		for _, c := range doc.AlloyChecks {
			path := strings.Join(c.ID.HeadingPath, " > ")
			if strings.Contains(path, filter) {
				checks = append(checks, c)
			}
		}
		if len(cases) > 0 || len(checks) > 0 {
			filtered = append(filtered, core.DocumentPlan{
				Document:    doc.Document,
				Cases:       cases,
				AlloyModels: doc.AlloyModels,
				AlloyChecks: checks,
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
			cases = append(cases, core.CaseResult{
				ID:      c.ID,
				Kind:    c.Kind,
				Block:   c.Block.Descriptor(),
				Fixture: c.Fixture,
				Label:   dryRunLabel(c),
				Columns: append([]string(nil), c.Columns...),
				RowNumber: c.RowNumber,
			})
		}
		alloyChecks := make([]core.AlloyCheckResult, 0, len(doc.AlloyChecks))
		for _, c := range doc.AlloyChecks {
			alloyChecks = append(alloyChecks, core.AlloyCheckResult{
				ID:        c.ID,
				Model:     c.Model,
				Assertion: c.Assertion,
				Scope:     c.Scope,
			})
		}
		results = append(results, core.DocumentResult{
			Document:    doc.Document,
			Cases:       cases,
			AlloyChecks: alloyChecks,
		})
		summary.CasesTotal += len(doc.Cases)
		summary.AlloyChecksTotal += len(doc.AlloyChecks)
	}

	return core.Report{
		GeneratedAt: time.Now(),
		Results:     results,
		Summary:     summary,
	}
}

func dryRunLabel(c core.CaseSpec) string {
	if len(c.ID.HeadingPath) == 0 {
		return c.DisplayKind()
	}
	return c.DisplayKind() + " @ " + c.ID.HeadingPath[len(c.ID.HeadingPath)-1]
}

func buildRegistry(adapters []config.AdapterConfig) (adapterRegistry, error) {
	registry := adapterRegistry{
		blocks:   make(map[string]adapterEntry),
		fixtures: make(map[string]adapterEntry),
	}
	for _, adapter := range adapters {
		entry := adapterEntry{Config: adapter}
		for _, block := range adapter.Blocks {
			if previous, exists := registry.blocks[block]; exists {
				return adapterRegistry{}, fmt.Errorf("block %q is declared by both adapter %q and %q", block, previous.Config.Name, adapter.Name)
			}
			registry.blocks[block] = entry
		}
		for _, fixture := range adapter.Fixtures {
			if previous, exists := registry.fixtures[fixture]; exists {
				return adapterRegistry{}, fmt.Errorf("fixture %q is declared by both adapter %q and %q", fixture, previous.Config.Name, adapter.Name)
			}
			registry.fixtures[fixture] = entry
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
		entry, ok := r.fixtures[specCase.Fixture]
		if !ok {
			return adapterEntry{}, fmt.Errorf("no adapter supports fixture %q in %s", specCase.Fixture, specCase.ID.Key())
		}
		return entry, nil
	default:
		return adapterEntry{}, fmt.Errorf("unsupported case kind %q", specCase.Kind)
	}
}

func runDocument(plan core.DocumentPlan, registry adapterRegistry, host adapterhost.Host, alloyRunner alloy.DocumentRunner) (core.DocumentResult, error) {
	if len(plan.Cases) == 0 && len(plan.AlloyChecks) == 0 {
		return core.DocumentResult{
			Document: plan.Document,
			Status:   core.StatusPassed,
		}, nil
	}

	sessions := make(map[string]*adapterhost.Session)
	defer func() { _ = closeSessions(sessions) }()

	cases, status, err := runDocumentCases(plan, registry, host, sessions)
	if err != nil {
		return core.DocumentResult{}, err
	}

	// Send teardown before closing
	for _, session := range sessions {
		_ = session.Teardown()
	}

	if err := closeSessions(sessions); err != nil {
		return core.DocumentResult{}, err
	}

	alloyResults, err := alloyRunner.RunDocument(plan)
	if err != nil {
		return core.DocumentResult{}, err
	}
	if hasFailedAlloyCheck(alloyResults) {
		status = core.StatusFailed
	}

	return core.DocumentResult{
		Document:    plan.Document,
		Status:      status,
		Cases:       cases,
		AlloyChecks: alloyResults,
	}, nil
}

func runDocumentCases(plan core.DocumentPlan, registry adapterRegistry, host adapterhost.Host, sessions map[string]*adapterhost.Session) ([]core.CaseResult, core.Status, error) {
	setupSessions := make(map[string]bool)
	bindings := make([]scopedBinding, 0)
	cases := make([]core.CaseResult, 0, len(plan.Cases))
	status := core.StatusPassed
	timeoutMs := plan.Document.Frontmatter.Timeout

	for _, specCase := range plan.Cases {
		result, err := runSingleCase(specCase, registry, host, sessions, setupSessions, bindings, timeoutMs)
		if err != nil {
			return nil, "", err
		}
		cases = append(cases, result)
		if result.Status == core.StatusFailed {
			status = core.StatusFailed
			continue
		}
		for _, binding := range result.Bindings {
			bindings = append(bindings, scopedBinding{
				Binding:     binding,
				HeadingPath: append([]string(nil), specCase.ID.HeadingPath...),
				Order:       len(bindings),
			})
		}
	}
	return cases, status, nil
}

func runSingleCase(specCase core.CaseSpec, registry adapterRegistry, host adapterhost.Host, sessions map[string]*adapterhost.Session, setupSessions map[string]bool, bindings []scopedBinding, timeoutMs int) (core.CaseResult, error) {
	adapter, err := registry.adapterFor(specCase)
	if err != nil {
		return core.CaseResult{}, err
	}

	visible := visibleBindings(bindings, specCase.ID.HeadingPath)
	prepared, err := prepareCase(specCase, visible)
	if err != nil {
		return variableFailure(specCase, err), nil
	}

	session, err := sessionFor(sessions, host, adapter.Config)
	if err != nil {
		return core.CaseResult{}, err
	}

	if !setupSessions[adapter.Config.Name] {
		setupSessions[adapter.Config.Name] = true
		if err := session.Setup(); err != nil {
			return core.CaseResult{}, err
		}
	}

	return session.RunCase(specCase, prepared, visible, timeoutMs)
}

func hasFailedAlloyCheck(results []core.AlloyCheckResult) bool {
	for _, result := range results {
		if result.Status == core.StatusFailed {
			return true
		}
	}
	return false
}

func sessionFor(sessions map[string]*adapterhost.Session, host adapterhost.Host, adapter config.AdapterConfig) (*adapterhost.Session, error) {
	if session, ok := sessions[adapter.Name]; ok {
		return session, nil
	}
	session, err := host.StartSession(adapter)
	if err != nil {
		return nil, err
	}
	sessions[adapter.Name] = session
	return session, nil
}

func closeSessions(sessions map[string]*adapterhost.Session) error {
	var firstErr error
	for name, session := range sessions {
		if session == nil {
			continue
		}
		if err := session.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close adapter session %q: %w", name, err)
		}
		delete(sessions, name)
	}
	return firstErr
}

func bindingReachable(bp []string, current []string) bool {
	// Ancestor or self
	if headingPathPrefix(bp, current) {
		return true
	}
	// Sibling: same depth, same parent
	if len(bp) > 0 && len(current) > 0 &&
		len(bp) == len(current) &&
		headingPathPrefix(bp[:len(bp)-1], current[:len(current)-1]) {
		return true
	}
	return false
}

func visibleBindings(bindings []scopedBinding, headingPath []string) []core.Binding {
	selected := make(map[string]scopedBinding)
	for _, binding := range bindings {
		if !bindingReachable(binding.HeadingPath, headingPath) {
			continue
		}
		current, ok := selected[binding.Binding.Name]
		if !ok || binding.Order >= current.Order {
			selected[binding.Binding.Name] = binding
		}
	}

	items := make([]core.Binding, 0, len(selected))
	names := make([]string, 0, len(selected))
	for name := range selected {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		items = append(items, selected[name].Binding)
	}
	return items
}

func headingPathPrefix(prefix []string, current []string) bool {
	if len(prefix) > len(current) {
		return false
	}
	for i := range prefix {
		if prefix[i] != current[i] {
			return false
		}
	}
	return true
}

var variablePattern = regexp.MustCompile(`(\\?)\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

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
	case core.CaseKindTableRow:
		rendered := make([]string, 0, len(specCase.Cells))
		for _, cell := range specCase.Cells {
			value, err := renderTemplate(cell, bindings)
			if err != nil {
				return core.CaseSpec{}, err
			}
			rendered = append(rendered, value)
		}
		prepared.Cells = rendered
		return prepared, nil
	default:
		return core.CaseSpec{}, fmt.Errorf("unsupported case kind %q", specCase.Kind)
	}
}

func renderTemplate(tmpl string, bindings []core.Binding) (string, error) {
	values := make(map[string]string, len(bindings))
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
		value, ok := values[match[2]]
		if !ok {
			unresolved = fmt.Errorf("missing runtime binding for %q", match[2])
			return raw
		}
		return value
	})
	if unresolved != nil {
		return "", unresolved
	}
	return rendered, nil
}

func variableFailure(specCase core.CaseSpec, err error) core.CaseResult {
	result := core.CaseResult{
		ID:        specCase.ID,
		Kind:      specCase.Kind,
		Block:     specCase.Block.Descriptor(),
		Fixture:   specCase.Fixture,
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


func accumulateSummary(summary *core.Summary, result core.DocumentResult) {
	if result.Status == core.StatusPassed {
		summary.SpecsPassed++
	} else {
		summary.SpecsFailed++
	}

	summary.CasesTotal += len(result.Cases)
	for _, item := range result.Cases {
		if item.Status == core.StatusPassed {
			summary.CasesPassed++
		} else {
			summary.CasesFailed++
		}
	}

	summary.AlloyChecksTotal += len(result.AlloyChecks)
	for _, item := range result.AlloyChecks {
		if item.Status == core.StatusPassed {
			summary.AlloyChecksPassed++
		} else {
			summary.AlloyChecksFailed++
		}
	}
}
