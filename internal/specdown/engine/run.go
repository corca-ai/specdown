package engine

import (
	"fmt"
	"regexp"
	"sort"
	"time"

	"specdown/internal/specdown/adapterhost"
	"specdown/internal/specdown/config"
	"specdown/internal/specdown/core"
)

type describedAdapter struct {
	Config       config.AdapterConfig
	Capabilities adapterhost.Capabilities
}

type scopedBinding struct {
	Binding     core.Binding
	HeadingPath []string
	Order       int
}

func Run(baseDir string, cfg config.Config) (core.Report, error) {
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

	host := adapterhost.Host{BaseDir: baseDir}
	registry, err := describeAdapters(host, cfg.Adapters)
	if err != nil {
		return core.Report{}, err
	}

	results := make([]core.DocumentResult, 0, len(plan.Documents))
	summary := core.Summary{SpecsTotal: len(plan.Documents)}
	for _, docPlan := range plan.Documents {
		result, err := runDocument(docPlan, registry, host)
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

func describeAdapters(host adapterhost.Host, adapters []config.AdapterConfig) (map[string]describedAdapter, error) {
	registry := make(map[string]describedAdapter)
	for _, adapter := range adapters {
		caps, err := host.Describe(adapter)
		if err != nil {
			return nil, err
		}
		described := describedAdapter{
			Config:       adapter,
			Capabilities: caps,
		}
		for _, block := range caps.Blocks {
			if previous, exists := registry[block]; exists {
				return nil, fmt.Errorf("block %q is supported by both adapter %q and %q", block, previous.Config.Name, adapter.Name)
			}
			registry[block] = described
		}
	}
	return registry, nil
}

func runDocument(plan core.DocumentPlan, registry map[string]describedAdapter, host adapterhost.Host) (core.DocumentResult, error) {
	if len(plan.Cases) == 0 {
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
		adapter, ok := registry[specCase.Block.Descriptor()]
		if !ok {
			return core.DocumentResult{}, fmt.Errorf("no adapter supports block %q in %s", specCase.Block.Descriptor(), specCase.ID.Key())
		}

		visible := visibleBindings(bindings, specCase.ID.HeadingPath)
		renderedSource, err := renderSource(specCase.Source, visible)
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

		result, err := session.RunCase(specCase, renderedSource, visible)
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

	return core.DocumentResult{
		Document: plan.Document,
		Status:   status,
		Cases:    cases,
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

func visibleBindings(bindings []scopedBinding, headingPath []string) []core.Binding {
	selected := make(map[string]scopedBinding)
	for _, binding := range bindings {
		if !headingPathPrefix(binding.HeadingPath, headingPath) {
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

var variablePattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func renderSource(template string, bindings []core.Binding) (string, error) {
	if len(bindings) == 0 {
		matches := variablePattern.FindAllStringSubmatch(template, -1)
		if len(matches) == 0 {
			return template, nil
		}
		return "", fmt.Errorf("missing runtime binding for %q", matches[0][1])
	}

	values := make(map[string]string, len(bindings))
	for _, binding := range bindings {
		values[binding.Name] = binding.Value
	}

	var unresolved error
	rendered := variablePattern.ReplaceAllStringFunc(template, func(raw string) string {
		match := variablePattern.FindStringSubmatch(raw)
		if len(match) != 2 {
			return raw
		}
		value, ok := values[match[1]]
		if !ok {
			unresolved = fmt.Errorf("missing runtime binding for %q", match[1])
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
		ID:             specCase.ID,
		Block:          specCase.Block.Descriptor(),
		Label:          defaultLabel(specCase),
		Template:       specCase.Source,
		RenderedSource: specCase.Source,
		Status:         core.StatusFailed,
		Message:        err.Error(),
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
		return specCase.Block.Descriptor()
	}
	return specCase.Block.Descriptor() + " @ " + specCase.ID.HeadingPath[len(specCase.ID.HeadingPath)-1]
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
}
