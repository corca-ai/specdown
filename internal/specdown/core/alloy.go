package core

import (
	"fmt"
	"regexp"
	"strings"
)

type AlloyFragmentSpec struct {
	Model       string
	Source      string
	HeadingPath []string
}

type AlloyModelSpec struct {
	Name      string
	Fragments []AlloyFragmentSpec
}

type AlloyCheckSpec struct {
	ID        SpecID
	Model     string
	Assertion string
	Scope     string
}

var moduleDeclPattern = regexp.MustCompile(`(?m)^\s*module\b`)

func compileAlloy(doc Document) ([]AlloyModelSpec, []AlloyCheckSpec, error) {
	modelsByName := make(map[string]int)
	models := make([]AlloyModelSpec, 0)
	checks := make([]AlloyCheckSpec, 0)

	for _, node := range doc.Nodes {
		switch current := node.(type) {
		case AlloyModelNode:
			index, ok := modelsByName[current.Model]
			if !ok {
				modelsByName[current.Model] = len(models)
				models = append(models, AlloyModelSpec{Name: current.Model})
				index = len(models) - 1
			} else if moduleDeclPattern.MatchString(current.Source) {
				return nil, nil, fmt.Errorf("%s: alloy:model(%s) may declare module only in its first fragment", doc.RelativeTo, current.Model)
			}

			models[index].Fragments = append(models[index].Fragments, AlloyFragmentSpec{
				Model:       current.Model,
				Source:      current.Source,
				HeadingPath: append([]string(nil), current.HeadingPath...),
			})
		case AlloyRefNode:
			if current.ID == nil {
				return nil, nil, fmt.Errorf("%s: alloy reference is missing an id", doc.RelativeTo)
			}
			checks = append(checks, AlloyCheckSpec{
				ID:        *current.ID,
				Model:     current.Model,
				Assertion: current.Assertion,
				Scope:     strings.TrimSpace(current.Scope),
			})
		}
	}

	knownModels := make(map[string]struct{}, len(models))
	for _, model := range models {
		knownModels[model.Name] = struct{}{}
	}

	for _, check := range checks {
		if _, ok := knownModels[check.Model]; !ok {
			return nil, nil, fmt.Errorf("%s: alloy reference %q targets unknown model %q", doc.RelativeTo, check.ID.Key(), check.Model)
		}
	}

	return models, checks, nil
}
