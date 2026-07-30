package engine

import (
	"fmt"

	"github.com/corca-ai/specdown/internal/specdown/config"
	"github.com/corca-ai/specdown/internal/specdown/core"
)

// adapterEntry holds an adapter config for registry lookups.
type adapterEntry struct {
	Config config.AdapterConfig
}

type adapterRegistry struct {
	blocks map[string]adapterEntry
	checks map[string]adapterEntry
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
