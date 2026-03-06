package engine

import (
	"fmt"
	"time"

	"specdown/internal/specdown/adapterhost"
	"specdown/internal/specdown/config"
	"specdown/internal/specdown/core"
)

type describedAdapter struct {
	Config       config.AdapterConfig
	Capabilities adapterhost.Capabilities
}

func Run(specRoot string, cfg config.Config, configDir string) (core.Report, error) {
	docs, err := core.Discover(specRoot)
	if err != nil {
		return core.Report{}, err
	}
	if len(docs) == 0 {
		return core.Report{}, fmt.Errorf("no .spec.md files found under %s", specRoot)
	}

	host := adapterhost.Host{BaseDir: configDir}
	registry, err := describeAdapters(host, cfg.Adapters)
	if err != nil {
		return core.Report{}, err
	}

	results := make([]core.DocumentResult, 0, len(docs))
	summary := core.Summary{SpecsTotal: len(docs)}
	for _, doc := range docs {
		result, err := runDocument(doc, registry, host)
		if err != nil {
			return core.Report{}, err
		}
		results = append(results, result)
		accumulateSummary(&summary, result)
	}

	return core.Report{
		SpecRoot:    specRoot,
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

func runDocument(doc core.Document, registry map[string]describedAdapter, host adapterhost.Host) (core.DocumentResult, error) {
	executable := executableBlocks(doc)
	if len(executable) == 0 {
		return core.DocumentResult{
			Document: doc,
			Status:   core.StatusPassed,
		}, nil
	}

	groups, order, err := assignAdapters(executable, registry)
	if err != nil {
		return core.DocumentResult{}, err
	}

	resultsByKey := make(map[string]core.CaseResult, len(executable))
	for _, adapterName := range order {
		group := groups[adapterName]
		resultMap, err := host.RunCases(group.adapter.Config, group.cases)
		if err != nil {
			return core.DocumentResult{}, err
		}
		for key, result := range resultMap {
			resultsByKey[key] = result
		}
	}

	cases := make([]core.CaseResult, 0, len(executable))
	status := core.StatusPassed
	for _, node := range executable {
		result, ok := resultsByKey[node.ID.Key()]
		if !ok {
			return core.DocumentResult{}, fmt.Errorf("missing result for case %s", node.ID.Key())
		}
		cases = append(cases, result)
		if result.Status == core.StatusFailed {
			status = core.StatusFailed
		}
	}

	return core.DocumentResult{
		Document: doc,
		Status:   status,
		Cases:    cases,
	}, nil
}

type adapterGroup struct {
	adapter describedAdapter
	cases   []core.CodeBlockNode
}

func assignAdapters(cases []core.CodeBlockNode, registry map[string]describedAdapter) (map[string]*adapterGroup, []string, error) {
	groups := make(map[string]*adapterGroup)
	order := make([]string, 0)

	for _, node := range cases {
		adapter, ok := registry[node.Block.String()]
		if !ok {
			return nil, nil, fmt.Errorf("no adapter supports block %q in %s", node.Block.String(), node.ID.Key())
		}

		group, exists := groups[adapter.Config.Name]
		if !exists {
			group = &adapterGroup{adapter: adapter}
			groups[adapter.Config.Name] = group
			order = append(order, adapter.Config.Name)
		}
		group.cases = append(group.cases, node)
	}

	return groups, order, nil
}

func executableBlocks(doc core.Document) []core.CodeBlockNode {
	blocks := make([]core.CodeBlockNode, 0)
	for _, node := range doc.Nodes {
		code, ok := node.(core.CodeBlockNode)
		if ok && code.ID != nil {
			blocks = append(blocks, code)
		}
	}
	return blocks
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
