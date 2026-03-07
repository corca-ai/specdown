package core

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

type CaseKind string

const (
	CaseKindCode     CaseKind = "code"
	CaseKindTableRow CaseKind = "tableRow"
)

type CaseSpec struct {
	ID            SpecID
	Kind          CaseKind
	Block         BlockSpec
	Fixture       string
	FixtureParams map[string]string
	Template      string
	Columns       []string
	Cells         []string
	RowNumber     int
	References    []string
}

type DocumentPlan struct {
	Document    Document
	Cases       []CaseSpec
	AlloyModels []AlloyModelSpec
	AlloyChecks []AlloyCheckSpec
}

type Plan struct {
	Documents []DocumentPlan
}

func Discover(baseDir string, include []string) ([]Document, error) {
	matches, err := discoverPaths(baseDir, include)
	if err != nil {
		return nil, err
	}

	docs := make([]Document, 0, len(matches))
	for _, match := range matches {
		doc, err := readDocument(baseDir, match)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func CompileDocuments(docs []Document) (Plan, error) {
	plans := make([]DocumentPlan, 0, len(docs))
	for _, doc := range docs {
		plan, err := CompileDocument(doc)
		if err != nil {
			return Plan{}, err
		}
		plans = append(plans, plan)
	}
	return Plan{Documents: plans}, nil
}

func CompileDocument(doc Document) (DocumentPlan, error) {
	cases := executableCases(doc)
	alloyModels, alloyChecks, err := compileAlloy(doc)
	if err != nil {
		return DocumentPlan{}, err
	}
	bindings := make([]bindingDefinition, 0)

	for i := range cases {
		references := caseReferences(cases[i])
		cases[i].References = references

		for _, name := range references {
			if !bindingVisible(bindings, name, cases[i].ID.HeadingPath) {
				return DocumentPlan{}, fmt.Errorf("%s: unresolved variable %q in %s", doc.RelativeTo, name, cases[i].ID.Key())
			}
		}

		for _, captureName := range cases[i].Block.CaptureNames {
			bindings = append(bindings, bindingDefinition{
				Name:        captureName,
				HeadingPath: append([]string(nil), cases[i].ID.HeadingPath...),
			})
		}
	}

	return DocumentPlan{
		Document:    doc,
		Cases:       cases,
		AlloyModels: alloyModels,
		AlloyChecks: alloyChecks,
	}, nil
}

func discoverPaths(baseDir string, include []string) ([]string, error) {
	if len(include) == 0 {
		return nil, fmt.Errorf("no include patterns configured")
	}

	seen := make(map[string]struct{})
	var matches []string
	err := walkFiles(baseDir, func(relativePath string) error {
		if !matchesAnyPattern(relativePath, include) {
			return nil
		}
		if _, ok := seen[relativePath]; ok {
			return nil
		}
		seen[relativePath] = struct{}{}
		matches = append(matches, relativePath)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(matches)
	return matches, nil
}

func matchesAnyPattern(relativePath string, include []string) bool {
	for _, pattern := range include {
		if matchPattern(path.Clean(pattern), relativePath) {
			return true
		}
	}
	return false
}

func matchPattern(pattern string, relativePath string) bool {
	pattern = path.Clean(strings.TrimPrefix(toSlash(pattern), "./"))
	relativePath = toSlash(relativePath)
	return matchSegments(strings.Split(pattern, "/"), strings.Split(relativePath, "/"))
}

func matchSegments(pattern []string, value []string) bool {
	if len(pattern) == 0 {
		return len(value) == 0
	}

	if pattern[0] == "**" {
		if matchSegments(pattern[1:], value) {
			return true
		}
		if len(value) == 0 {
			return false
		}
		return matchSegments(pattern, value[1:])
	}

	if len(value) == 0 {
		return false
	}

	ok, err := path.Match(pattern[0], value[0])
	if err != nil || !ok {
		return false
	}
	return matchSegments(pattern[1:], value[1:])
}

type bindingDefinition struct {
	Name        string
	HeadingPath []string
}

func bindingVisible(bindings []bindingDefinition, name string, currentPath []string) bool {
	for i := len(bindings) - 1; i >= 0; i-- {
		if bindings[i].Name != name {
			continue
		}
		bp := bindings[i].HeadingPath
		// Visible if binding path is a prefix of current path (ancestor or self)
		if headingPathPrefix(bp, currentPath) {
			return true
		}
		// Visible if binding is a sibling: same parent, defined earlier in document order
		if len(bp) > 0 && len(currentPath) > 0 &&
			len(bp) == len(currentPath) &&
			headingPathPrefix(bp[:len(bp)-1], currentPath[:len(currentPath)-1]) {
			return true
		}
	}
	return false
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

func executableCases(doc Document) []CaseSpec {
	cases := make([]CaseSpec, 0)
	for _, node := range doc.Nodes {
		switch current := node.(type) {
		case CodeBlockNode:
			cases = appendCodeCase(cases, current)
		case TableNode:
			cases = appendTableCases(cases, current)
		}
	}
	return cases
}

func appendCodeCase(cases []CaseSpec, block CodeBlockNode) []CaseSpec {
	if block.ID == nil {
		return cases
	}
	return append(cases, CaseSpec{
		ID:       *block.ID,
		Kind:     CaseKindCode,
		Block:    block.Block,
		Template: block.Source,
	})
}

func appendTableCases(cases []CaseSpec, table TableNode) []CaseSpec {
	if table.Fixture == "" {
		return cases
	}
	for index, row := range table.Rows {
		if row.ID == nil {
			continue
		}
		cases = append(cases, CaseSpec{
			ID:            *row.ID,
			Kind:          CaseKindTableRow,
			Fixture:       table.Fixture,
			FixtureParams: table.FixtureParams,
			Columns:       append([]string(nil), table.Columns...),
			Cells:         append([]string(nil), row.Cells...),
			RowNumber:     index + 1,
		})
	}
	return cases
}

var variableRefPattern = regexp.MustCompile(`(\\?)\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func variableReferences(source string) []string {
	matches := variableRefPattern.FindAllStringSubmatch(source, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	refs := make([]string, 0, len(matches))
	for _, match := range matches {
		if match[1] == `\` {
			continue // escaped \${...}
		}
		name := match[2]
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		refs = append(refs, name)
	}
	return refs
}

func caseReferences(spec CaseSpec) []string {
	switch spec.Kind {
	case CaseKindCode:
		return variableReferences(spec.Template)
	case CaseKindTableRow:
		seen := make(map[string]struct{})
		refs := make([]string, 0)
		for _, cell := range spec.Cells {
			for _, name := range variableReferences(cell) {
				if _, ok := seen[name]; ok {
					continue
				}
				seen[name] = struct{}{}
				refs = append(refs, name)
			}
		}
		return refs
	default:
		return nil
	}
}

func (c CaseSpec) TargetKey() string {
	if c.Kind == CaseKindCode {
		return c.Block.Descriptor()
	}
	return c.Fixture
}

func (c CaseSpec) DisplayKind() string {
	if c.Kind == CaseKindCode {
		return c.Block.Descriptor()
	}
	return "fixture:" + c.Fixture
}

func (c CaseSpec) DefaultLabel() string {
	if len(c.ID.HeadingPath) == 0 {
		return c.DisplayKind()
	}
	suffix := c.ID.HeadingPath[len(c.ID.HeadingPath)-1]
	if c.Kind == CaseKindTableRow {
		return c.DisplayKind() + " @ " + suffix + " row " + fmt.Sprintf("%d", c.RowNumber)
	}
	return c.DisplayKind() + " @ " + suffix
}

func (c AlloyCheckSpec) DefaultLabel() string {
	suffix := "alloy:ref(" + c.Model + "#" + c.Assertion + ", scope=" + c.Scope + ")"
	if len(c.ID.HeadingPath) == 0 {
		return suffix
	}
	return suffix + " @ " + c.ID.HeadingPath[len(c.ID.HeadingPath)-1]
}

func toSlash(value string) string {
	return strings.ReplaceAll(value, `\`, "/")
}
