package engine

import (
	"context"
	"strings"

	"github.com/corca-ai/specdown/internal/specdown/alloy"
	"github.com/corca-ai/specdown/internal/specdown/config"
	"github.com/corca-ai/specdown/internal/specdown/core"
)

// ModelExplorer runs Alloy models and returns instance-level results.
type ModelExplorer interface {
	ExploreDocument(
		ctx context.Context,
		plan core.DocumentPlan,
		opts alloy.ExploreOptions,
	) ([]alloy.ExploreModelResult, error)
}

// ModelDumper can write model artifacts without running verification.
type ModelDumper interface {
	DumpModels(plan core.DocumentPlan) ([]string, error)
}

// ExploreModels runs Alloy models from all discovered documents and returns
// per-model results grouped by document path.
func ExploreModels(
	baseDir string,
	cfg config.Config,
	explorer ModelExplorer,
	filter string,
	opts alloy.ExploreOptions,
) (map[string][]alloy.ExploreModelResult, error) {
	return ExploreModelsContext(context.Background(), baseDir, cfg, explorer, filter, opts)
}

func ExploreModelsContext(
	ctx context.Context,
	baseDir string,
	cfg config.Config,
	explorer ModelExplorer,
	filter string,
	opts alloy.ExploreOptions,
) (map[string][]alloy.ExploreModelResult, error) {
	_, documents, err := core.DiscoverFromEntry(baseDir, cfg.Entry, cfg.IgnorePrefixes)
	if err != nil {
		return nil, err
	}

	plan, err := core.CompileDocuments(documents)
	if err != nil {
		return nil, err
	}
	if filter != "" {
		plan = filterPlanByDoc(plan, filter)
	}

	results := make(map[string][]alloy.ExploreModelResult)
	for i := range plan.Documents {
		documentPath := plan.Documents[i].Document.RelativeTo
		explored, exploreErr := explorer.ExploreDocument(ctx, plan.Documents[i], opts)
		if exploreErr != nil {
			return nil, exploreErr
		}
		if len(explored) > 0 {
			results[documentPath] = explored
		}
	}
	return results, nil
}

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
	_, documents, err := core.DiscoverFromEntry(baseDir, cfg.Entry, cfg.IgnorePrefixes)
	if err != nil {
		return nil, err
	}

	plan, err := core.CompileDocuments(documents)
	if err != nil {
		return nil, err
	}

	var paths []string
	for i := range plan.Documents {
		dumped, dumpErr := dumper.DumpModels(plan.Documents[i])
		if dumpErr != nil {
			return nil, dumpErr
		}
		paths = append(paths, dumped...)
	}
	return paths, nil
}
