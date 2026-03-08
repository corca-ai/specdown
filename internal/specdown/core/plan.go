package core

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

type CaseKind string

const (
	CaseKindCode         CaseKind = "code"
	CaseKindTableRow     CaseKind = "tableRow"
	CaseKindInlineExpect CaseKind = "inlineExpect"
)

type CaseSpec struct {
	ID            SpecID
	Kind          CaseKind
	Block         BlockSpec
	Fixture       string
	FixtureParams map[string]string
	Template      string
	ExpectValue   string
	ExpectFail    bool
	Columns       []string
	Cells         []string
	RowNumber     int
	References    []string
}

type HookSpec struct {
	Kind        HookKind
	Each        bool
	HeadingPath []string
	Block       BlockSpec
	Source      string
}

type DocumentPlan struct {
	Document    Document
	Cases       []CaseSpec
	Hooks       []HookSpec
	AlloyModels []AlloyModelSpec
	AlloyChecks []AlloyCheckSpec
}

type Plan struct {
	Documents []DocumentPlan
}

func DiscoverFromEntry(baseDir string, entryPath string) (string, []Document, error) {
	fullPath := filepath.Join(baseDir, filepath.FromSlash(entryPath))
	body, err := os.ReadFile(fullPath)
	if err != nil {
		return "", nil, fmt.Errorf("read entry %s: %w", entryPath, err)
	}

	content := string(body)
	title := parseEntryTitle(content)
	if title == "" {
		return "", nil, fmt.Errorf("entry file %s must have an H1 heading", entryPath)
	}

	links := parseEntryLinks(content)
	if len(links) == 0 {
		return "", nil, fmt.Errorf("entry file %s contains no links to spec files", entryPath)
	}

	entryDir := path.Dir(path.Clean(entryPath))
	docs := make([]Document, 0, len(links))
	seen := make(map[string]struct{})
	for _, link := range links {
		relativePath := path.Clean(path.Join(entryDir, link))
		if _, ok := seen[relativePath]; ok {
			continue
		}
		seen[relativePath] = struct{}{}
		doc, err := readDocument(baseDir, relativePath)
		if err != nil {
			return "", nil, err
		}
		docs = append(docs, doc)
	}
	return title, docs, nil
}

var markdownLinkPattern = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+\.spec\.md)\)`)

func parseEntryTitle(markdown string) string {
	for _, line := range strings.Split(markdown, "\n") {
		if level, text, ok := parseHeading(line + "\n"); ok && level == 1 {
			return text
		}
	}
	return ""
}

func parseEntryLinks(markdown string) []string {
	matches := markdownLinkPattern.FindAllStringSubmatch(markdown, -1)
	var paths []string
	for _, match := range matches {
		paths = append(paths, match[2])
	}
	return paths
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
	hooks := extractHooks(doc)
	alloyModels, alloyChecks, err := compileAlloy(doc, documentMaxOrdinal(doc))
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
		Hooks:       hooks,
		AlloyModels: alloyModels,
		AlloyChecks: alloyChecks,
	}, nil
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

func documentMaxOrdinal(doc Document) int {
	max := 0
	for _, id := range documentOrdinals(doc) {
		if id != nil && id.Ordinal > max {
			max = id.Ordinal
		}
	}
	return max
}

func documentOrdinals(doc Document) []*SpecID {
	var ids []*SpecID
	for _, node := range doc.Nodes {
		switch n := node.(type) {
		case CodeBlockNode:
			ids = append(ids, n.ID)
		case AlloyRefNode:
			ids = append(ids, n.ID)
		case FixtureCallNode:
			ids = append(ids, n.ID)
		case TableNode:
			for i := range n.Rows {
				ids = append(ids, n.Rows[i].ID)
			}
		case ProseNode:
			for i := range n.Inlines {
				ids = append(ids, n.Inlines[i].ID)
			}
		}
	}
	return ids
}

func extractHooks(doc Document) []HookSpec {
	var hooks []HookSpec
	for _, node := range doc.Nodes {
		if h, ok := node.(HookNode); ok {
			hooks = append(hooks, HookSpec{
				Kind:        h.Hook,
				Each:        h.Each,
				HeadingPath: append([]string(nil), h.HeadingPath...),
				Block:       h.Block,
				Source:      h.Source,
			})
		}
	}
	return hooks
}

func executableCases(doc Document) []CaseSpec {
	cases := make([]CaseSpec, 0)
	for _, node := range doc.Nodes {
		switch current := node.(type) {
		case CodeBlockNode:
			cases = appendCodeCase(cases, current)
		case TableNode:
			cases = appendTableCases(cases, current)
		case FixtureCallNode:
			cases = appendFixtureCallCase(cases, current)
		case ProseNode:
			cases = appendInlineCases(cases, current)
		}
	}
	return cases
}

func appendInlineCases(cases []CaseSpec, node ProseNode) []CaseSpec {
	for _, inline := range node.Inlines {
		if inline.ID == nil {
			continue
		}
		switch inline.Kind {
		case InlineExpect:
			cases = append(cases, CaseSpec{
				ID:          *inline.ID,
				Kind:        CaseKindInlineExpect,
				Template:    inline.ExpectExpr,
				ExpectValue: inline.ExpectValue,
				ExpectFail:  inline.ExpectFail,
			})
		case InlineFixture:
			cases = append(cases, CaseSpec{
				ID:            *inline.ID,
				Kind:          CaseKindTableRow,
				Fixture:       inline.Fixture,
				FixtureParams: inline.FixtureParams,
			})
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

func appendFixtureCallCase(cases []CaseSpec, node FixtureCallNode) []CaseSpec {
	if node.ID == nil {
		return cases
	}
	return append(cases, CaseSpec{
		ID:            *node.ID,
		Kind:          CaseKindTableRow,
		Fixture:       node.Fixture,
		FixtureParams: node.FixtureParams,
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

func mergeVariableReferences(sources ...string) []string {
	seen := make(map[string]struct{})
	refs := make([]string, 0)
	for _, source := range sources {
		for _, name := range variableReferences(source) {
			if _, ok := seen[name]; !ok {
				seen[name] = struct{}{}
				refs = append(refs, name)
			}
		}
	}
	return refs
}

func caseReferences(spec CaseSpec) []string {
	switch spec.Kind {
	case CaseKindCode:
		return variableReferences(spec.Template)
	case CaseKindInlineExpect:
		return mergeVariableReferences(spec.Template, spec.ExpectValue)
	case CaseKindTableRow:
		return mergeVariableReferences(spec.Cells...)
	default:
		return nil
	}
}

func (c CaseSpec) TargetKey() string {
	switch c.Kind {
	case CaseKindCode:
		return c.Block.Descriptor()
	case CaseKindInlineExpect:
		return "expect"
	default:
		return c.Fixture
	}
}

func (c CaseSpec) DisplayKind() string {
	switch c.Kind {
	case CaseKindCode:
		return c.Block.Descriptor()
	case CaseKindInlineExpect:
		return "expect"
	default:
		return "fixture:" + c.Fixture
	}
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

