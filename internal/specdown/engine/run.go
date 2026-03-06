package engine

import (
	"fmt"
	"regexp"
	"sort"
	"time"

	"specdown/internal/specdown/alloy"
	"specdown/internal/specdown/adapterhost"
	"specdown/internal/specdown/config"
	"specdown/internal/specdown/core"
)

type describedAdapter struct {
	Config       config.AdapterConfig
	Capabilities adapterhost.Capabilities
}

type adapterRegistry struct {
	blocks   map[string]describedAdapter
	fixtures map[string]describedAdapter
}

type scopedBinding struct {
	Binding     core.Binding
	HeadingPath []string
	Order       int
}

func Run(baseDir string, cfg config.Config) (core.Report, error) {
	host := adapterhost.Host{BaseDir: baseDir}
	return runWithDependencies(baseDir, cfg, host, alloy.Runner{BaseDir: baseDir})
}

func runWithDependencies(baseDir string, cfg config.Config, host adapterhost.Host, alloyRunner alloy.DocumentRunner) (core.Report, error) {
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
	registry, err := describeAdapters(host, cfg.Adapters)
	if err != nil {
		return core.Report{}, err
	}

	results := make([]core.DocumentResult, 0, len(plan.Documents))
	summary := core.Summary{SpecsTotal: len(plan.Documents)}
	for _, docPlan := range plan.Documents {
		result, err := runDocument(docPlan, registry, host, alloyRunner)
		if err != nil {
			return core.Report{}, err
		}
		results = append(results, result)
		accumulateSummary(&summary, result)
	}

	return core.Report{
		GeneratedAt: time.Now(),
		Results:     results,
		Summary:     summary,
	}, nil
}

func describeAdapters(host adapterhost.Host, adapters []config.AdapterConfig) (adapterRegistry, error) {
	registry := adapterRegistry{
		blocks:   make(map[string]describedAdapter),
		fixtures: make(map[string]describedAdapter),
	}
	for _, adapter := range adapters {
		caps, err := host.Describe(adapter)
		if err != nil {
			return adapterRegistry{}, err
		}
		described := describedAdapter{
			Config:       adapter,
			Capabilities: caps,
		}
		for _, block := range caps.Blocks {
			if previous, exists := registry.blocks[block]; exists {
				return adapterRegistry{}, fmt.Errorf("block %q is supported by both adapter %q and %q", block, previous.Config.Name, adapter.Name)
			}
			registry.blocks[block] = described
		}
		for _, fixture := range caps.Fixtures {
			if previous, exists := registry.fixtures[fixture]; exists {
				return adapterRegistry{}, fmt.Errorf("fixture %q is supported by both adapter %q and %q", fixture, previous.Config.Name, adapter.Name)
			}
			registry.fixtures[fixture] = described
		}
	}
	return registry, nil
}

func (r adapterRegistry) adapterFor(specCase core.CaseSpec) (describedAdapter, error) {
	switch specCase.Kind {
	case core.CaseKindCode:
		adapter, ok := r.blocks[specCase.Block.Descriptor()]
		if !ok {
			return describedAdapter{}, fmt.Errorf("no adapter supports block %q in %s", specCase.Block.Descriptor(), specCase.ID.Key())
		}
		return adapter, nil
	case core.CaseKindTableRow:
		adapter, ok := r.fixtures[specCase.Fixture]
		if !ok {
			return describedAdapter{}, fmt.Errorf("no adapter supports fixture %q in %s", specCase.Fixture, specCase.ID.Key())
		}
		return adapter, nil
	default:
		return describedAdapter{}, fmt.Errorf("unsupported case kind %q", specCase.Kind)
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
	defer closeSessions(sessions)

	bindings := make([]scopedBinding, 0)
	cases := make([]core.CaseResult, 0, len(plan.Cases))
	status := core.StatusPassed
	for _, specCase := range plan.Cases {
		adapter, err := registry.adapterFor(specCase)
		if err != nil {
			return core.DocumentResult{}, err
		}

		visible := visibleBindings(bindings, specCase.ID.HeadingPath)
		prepared, err := prepareCase(specCase, visible)
		if err != nil {
			result := variableFailure(specCase, err)
			cases = append(cases, result)
			status = core.StatusFailed
			continue
		}

		session, err := sessionFor(sessions, host, adapter.Config)
		if err != nil {
			return core.DocumentResult{}, err
		}

		result, err := session.RunCase(specCase, prepared, visible)
		if err != nil {
			return core.DocumentResult{}, err
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

	if err := closeSessions(sessions); err != nil {
		return core.DocumentResult{}, err
	}

	alloyResults, err := alloyRunner.RunDocument(plan)
	if err != nil {
		return core.DocumentResult{}, err
	}
	for _, result := range alloyResults {
		if result.Status == core.StatusFailed {
			status = core.StatusFailed
			break
		}
	}

	return core.DocumentResult{
		Document:    plan.Document,
		Status:      status,
		Cases:       cases,
		AlloyChecks: alloyResults,
	}, nil
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
		Label:     defaultLabel(specCase),
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

func defaultLabel(specCase core.CaseSpec) string {
	if len(specCase.ID.HeadingPath) == 0 {
		return specCase.DisplayKind()
	}
	suffix := specCase.ID.HeadingPath[len(specCase.ID.HeadingPath)-1]
	if specCase.Kind == core.CaseKindTableRow {
		return specCase.DisplayKind() + " @ " + suffix + " row " + fmt.Sprintf("%d", specCase.RowNumber)
	}
	return specCase.DisplayKind() + " @ " + suffix
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
