package engine

import (
	"fmt"
	"time"

	"specdown/internal/specdown/alloy"
	"specdown/internal/specdown/config"
	"specdown/internal/specdown/core"
)

func CheckModels(baseDir string, cfg config.Config) (core.Report, error) {
	return checkModelsWithRunner(baseDir, cfg, alloy.Runner{BaseDir: baseDir})
}

func checkModelsWithRunner(baseDir string, cfg config.Config, alloyRunner alloy.DocumentRunner) (core.Report, error) {
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

	results := make([]core.DocumentResult, 0, len(plan.Documents))
	summary := core.Summary{SpecsTotal: len(plan.Documents)}
	for _, docPlan := range plan.Documents {
		alloyChecks, err := alloyRunner.RunDocument(docPlan)
		if err != nil {
			return core.Report{}, err
		}

		status := core.StatusPassed
		for _, item := range alloyChecks {
			if item.Status == core.StatusFailed {
				status = core.StatusFailed
				break
			}
		}

		result := core.DocumentResult{
			Document:    docPlan.Document,
			Status:      status,
			AlloyChecks: alloyChecks,
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
