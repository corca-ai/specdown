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
var alloyRunPattern = regexp.MustCompile(`(?m)^\s*run\s+([A-Za-z_][A-Za-z0-9_]*)\s+\{(?:[^{}]|\{[^{}]*\})*\}\s+for\s+(.+?)\s*$`)
var alloyScopePattern = regexp.MustCompile(`^(?:exactly\s+)?\d+(?:\s+but\s+\d+\s+[A-Za-z_]\w*(?:/[A-Za-z_]\w*)*(?:\s*,\s*\d+\s+[A-Za-z_]\w*(?:/[A-Za-z_]\w*)*)*)?$`)

type parsedCheck struct {
	assertion string
	scope     string
}

func validateAlloyScope(scope string) error {
	if !alloyScopePattern.MatchString(scope) {
		return fmt.Errorf("invalid Alloy scope %q (expected format: \"N\", \"N but M Type\", or \"exactly N\")", scope)
	}
	return nil
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

func parseRunStatements(source string) []parsedCheck {
	matches := alloyRunPattern.FindAllStringSubmatch(source, -1)
	var runs []parsedCheck
	for _, m := range matches {
		runs = append(runs, parsedCheck{
			assertion: m[1],
			scope:     strings.TrimSpace(m[2]),
		})
	}
	return runs
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
				HeadingPath: copyPath(current.HeadingPath),
			})
		case AlloyRefNode:
			if current.ID == nil {
				return nil, nil, nil, fmt.Errorf("%s: alloy reference is missing an id", doc.RelativeTo)
			}
			explicitRefs[current.Model+"#"+current.Assertion] = struct{}{}
			checks = append(checks, CaseSpec{
				ID:   *current.ID,
				Kind: CaseKindAlloy,
				Alloy: &AlloyCaseSpec{
					Model:     current.Model,
					Assertion: current.Assertion,
					Scope:     strings.TrimSpace(current.Scope),
				},
			})
		}
	}

	return models, checks, explicitRefs, nil
}

func appendImplicitChecks(file string, models []AlloyModelSpec, checks []CaseSpec, seen map[string]struct{}, ordinal int) []CaseSpec {
	for _, model := range models {
		for _, fragment := range model.Fragments {
			checks, ordinal = appendParsedStatements(file, model.Name, fragment, parseCheckStatements(fragment.Source), false, checks, seen, ordinal)
			checks, ordinal = appendParsedStatements(file, model.Name, fragment, parseRunStatements(fragment.Source), true, checks, seen, ordinal)
		}
	}
	return checks
}

func appendParsedStatements(file, modelName string, fragment AlloyFragmentSpec, stmts []parsedCheck, isRun bool, checks []CaseSpec, seen map[string]struct{}, ordinal int) (updated []CaseSpec, next int) {
	for _, s := range stmts {
		key := modelName + "#" + s.assertion
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		ordinal++
		checks = append(checks, CaseSpec{
			ID: SpecID{
				File:        file,
				HeadingPath: copyPath(fragment.HeadingPath),
				Ordinal:     ordinal,
			},
			Kind: CaseKindAlloy,
			Alloy: &AlloyCaseSpec{
				Model:     modelName,
				Assertion: s.assertion,
				Scope:     s.scope,
				IsRun:     isRun,
			},
		})
	}
	return checks, ordinal
}

func validateAlloyModelRefs(file string, models []AlloyModelSpec, checks []CaseSpec) error {
	knownModels := make(map[string]struct{}, len(models))
	for _, model := range models {
		knownModels[model.Name] = struct{}{}
	}
	for i := range checks {
		if checks[i].Alloy == nil {
			continue
		}
		if _, ok := knownModels[checks[i].Alloy.Model]; !ok {
			return fmt.Errorf("%s: alloy reference %q targets unknown model %q", file, checks[i].ID.Key(), checks[i].Alloy.Model)
		}
	}
	return nil
}
