package core

import (
	"fmt"
	"regexp"
	"strings"
)

type AlloyFragmentSpec struct {
	Model       string
	Source      string
	HeadingPath HeadingPath
}

type AlloyModelSpec struct {
	Name      string
	Fragments []AlloyFragmentSpec
}

var moduleDeclPattern = regexp.MustCompile(`(?m)^\s*module\b`)
var alloyCheckPattern = regexp.MustCompile(`(?m)^\s*check\s+([A-Za-z_][A-Za-z0-9_]*)\s+for\s+(.+?)\s*$`)

type parsedCheck struct {
	assertion string
	scope     string
}

func parseCheckStatements(source string) []parsedCheck {
	matches := alloyCheckPattern.FindAllStringSubmatch(source, -1)
	var checks []parsedCheck
	for _, m := range matches {
		checks = append(checks, parsedCheck{
			assertion: m[1],
			scope:     strings.TrimSpace(m[2]),
		})
	}
	return checks
}

//nolint:gocognit // alloy compilation is inherently complex
func compileAlloy(doc Document, maxOrdinal int) ([]AlloyModelSpec, []CaseSpec, error) {
	models, checks, explicitRefs, err := collectAlloyNodes(doc)
	if err != nil {
		return nil, nil, err
	}

	checks = appendImplicitChecks(doc.RelativeTo, models, checks, explicitRefs, maxOrdinal)

	if err := validateAlloyModelRefs(doc.RelativeTo, models, checks); err != nil {
		return nil, nil, err
	}

	return models, checks, nil
}

func collectAlloyNodes(doc Document) ([]AlloyModelSpec, []CaseSpec, map[string]struct{}, error) {
	modelsByName := make(map[string]int)
	models := make([]AlloyModelSpec, 0)
	checks := make([]CaseSpec, 0)
	explicitRefs := make(map[string]struct{})

	for _, node := range doc.Nodes {
		switch current := node.(type) {
		case AlloyModelNode:
			index, ok := modelsByName[current.Model]
			if !ok {
				modelsByName[current.Model] = len(models)
				models = append(models, AlloyModelSpec{Name: current.Model})
				index = len(models) - 1
			} else if moduleDeclPattern.MatchString(current.Source) {
				return nil, nil, nil, fmt.Errorf("%s: alloy:model(%s) may declare module only in its first fragment", doc.RelativeTo, current.Model)
			}

			models[index].Fragments = append(models[index].Fragments, AlloyFragmentSpec{
				Model:       current.Model,
				Source:      current.Source,
				HeadingPath: append([]string(nil), current.HeadingPath...),
			})
		case AlloyRefNode:
			if current.ID == nil {
				return nil, nil, nil, fmt.Errorf("%s: alloy reference is missing an id", doc.RelativeTo)
			}
			explicitRefs[current.Model+"#"+current.Assertion] = struct{}{}
			checks = append(checks, CaseSpec{
				ID:        *current.ID,
				Kind:      CaseKindAlloy,
				Model:     current.Model,
				Assertion: current.Assertion,
				Scope:     strings.TrimSpace(current.Scope),
			})
		}
	}

	return models, checks, explicitRefs, nil
}

func appendImplicitChecks(file string, models []AlloyModelSpec, checks []CaseSpec, seen map[string]struct{}, ordinal int) []CaseSpec {
	for _, model := range models {
		for _, fragment := range model.Fragments {
			for _, pc := range parseCheckStatements(fragment.Source) {
				key := model.Name + "#" + pc.assertion
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				ordinal++
				checks = append(checks, CaseSpec{
					ID: SpecID{
						File:        file,
						HeadingPath: append([]string(nil), fragment.HeadingPath...),
						Ordinal:     ordinal,
					},
					Kind:      CaseKindAlloy,
					Model:     model.Name,
					Assertion: pc.assertion,
					Scope:     pc.scope,
				})
			}
		}
	}
	return checks
}

func validateAlloyModelRefs(file string, models []AlloyModelSpec, checks []CaseSpec) error {
	knownModels := make(map[string]struct{}, len(models))
	for _, model := range models {
		knownModels[model.Name] = struct{}{}
	}
	for _, check := range checks {
		if check.Kind != CaseKindAlloy {
			continue
		}
		if _, ok := knownModels[check.Model]; !ok {
			return fmt.Errorf("%s: alloy reference %q targets unknown model %q", file, check.ID.Key(), check.Model)
		}
	}
	return nil
}
