package core

import (
	"errors"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
)

type CaseKind string

const (
	CaseKindCode         CaseKind = "code"
	CaseKindTableRow     CaseKind = "tableRow"
	CaseKindInlineExpect CaseKind = "inlineExpect"
	CaseKindAlloy        CaseKind = "alloy"
)

// CodeCaseSpec holds fields specific to executable code block cases.
type CodeCaseSpec struct {
	Block    BlockSpec
	Template string
}

// TableRowCaseSpec holds fields specific to table row and check call cases.
type TableRowCaseSpec struct {
	Check       string
	CheckParams map[string]string
	Columns     []string
	Cells       []string
	RowNumber   int
}

// InlineExpectCaseSpec holds fields specific to inline expect cases.
type InlineExpectCaseSpec struct {
	Template    string
	ExpectValue string
	ExpectFail  bool
}

// AlloyCaseSpec holds fields specific to alloy verification cases.
type AlloyCaseSpec struct {
	Model     string
	Assertion string
	Scope     string
}

// CaseSpec represents an executable case. Exactly one of Code, TableRow,
// InlineExpect, or Alloy is set, matching Kind.
type CaseSpec struct {
	ID         SpecID
	Kind       CaseKind
	References []string

	Code         *CodeCaseSpec
	TableRow     *TableRowCaseSpec
	InlineExpect *InlineExpectCaseSpec
	Alloy        *AlloyCaseSpec
}

type HookSpec struct {
	Kind        HookKind
	Each        bool
	HeadingPath HeadingPath
	Block       BlockSpec
	Source      string
}

type DocumentPlan struct {
	Document    Document
	Cases       []CaseSpec
	Hooks       []HookSpec
	AlloyModels []AlloyModelSpec
}

type Plan struct {
	Documents []DocumentPlan
}

// resolveLink resolves a markdown link relative to the current document.
// Returns empty string if the link should be skipped (external, anchor, non-md).
func resolveLink(link, currentDir string) string {
	if strings.Contains(link, "://") || strings.HasPrefix(link, "#") {
		return ""
	}
	linkPath := link
	if idx := strings.Index(linkPath, "#"); idx >= 0 {
		linkPath = linkPath[:idx]
	}
	if linkPath == "" || !strings.HasSuffix(linkPath, ".md") {
		return ""
	}
	return path.Clean(path.Join(currentDir, linkPath))
}

// crawlState holds mutable state for the BFS document crawl.
type crawlState struct {
	baseDir        string
	entryDir       string
	ignorePrefixes []string
	seen           map[string]struct{}
	docs           []Document
	warnings       []string
}

// processLink resolves a single link from a document and, if valid,
// reads the target document and appends it to the crawl state.
// Returns the new document (for queueing) or nil if skipped.
func (cs *crawlState) processLink(link string, source Document) (*Document, error) {
	currentDir := path.Dir(source.RelativeTo)
	resolved := resolveLink(link, currentDir)
	if resolved == "" {
		return nil, nil
	}
	if !isInsideDir(resolved, cs.entryDir) {
		cs.warnings = append(cs.warnings, fmt.Sprintf(
			"%s: link %q points outside entry directory %s", source.RelativeTo, link, cs.entryDir))
		return nil, nil
	}
	if _, ok := cs.seen[resolved]; ok {
		return nil, nil
	}
	cs.seen[resolved] = struct{}{}

	doc, err := readDocument(cs.baseDir, resolved, cs.ignorePrefixes)
	if err != nil {
		if os.IsNotExist(unwrapPathError(err)) {
			return nil, fmt.Errorf("%s: broken link %q (file not found: %s)", source.RelativeTo, link, resolved)
		}
		return nil, err
	}
	cs.docs = append(cs.docs, doc)
	return &doc, nil
}

func DiscoverFromEntry(baseDir, entryPath string, ignorePrefixes []string) (string, []Document, error) {
	entryPath = path.Clean(entryPath)

	entryDoc, err := readDocument(baseDir, entryPath, ignorePrefixes)
	if err != nil {
		return "", nil, fmt.Errorf("read entry %s: %w", entryPath, err)
	}

	cs := crawlState{
		baseDir:        baseDir,
		entryDir:       path.Dir(entryPath),
		ignorePrefixes: ignorePrefixes,
		seen:           map[string]struct{}{entryPath: {}},
		docs:           []Document{entryDoc},
	}

	queue := []Document{entryDoc}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, link := range parseMarkdownLinks(current.Markdown) {
			doc, linkErr := cs.processLink(link, current)
			if linkErr != nil {
				return "", nil, linkErr
			}
			if doc != nil {
				queue = append(queue, *doc)
			}
		}
	}

	if len(cs.warnings) > 0 {
		cs.docs[0].Warnings = append(cs.docs[0].Warnings, cs.warnings...)
	}

	return entryDoc.Title, cs.docs, nil
}

// isInsideDir checks if a cleaned path is inside the given directory.
func isInsideDir(filePath, dir string) bool {
	if dir == "." {
		// Root-relative: anything without ".." prefix is inside.
		return !strings.HasPrefix(filePath, "..")
	}
	return filePath == dir || strings.HasPrefix(filePath, dir+"/")
}

// unwrapPathError returns the underlying error if it's an *os.PathError.
func unwrapPathError(err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}

// markdownLinkAllPattern matches markdown links to any target.
var markdownLinkAllPattern = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)

// fencedCodeBlockPattern strips fenced code blocks from markdown.
var fencedCodeBlockPattern = regexp.MustCompile("(?m)^```[^\n]*\n(?s:.*?)\n```\\s*$")

// parseMarkdownLinks extracts all markdown link targets from content,
// excluding links inside fenced code blocks.
func parseMarkdownLinks(markdown string) []string {
	// Strip fenced code blocks so we don't follow links inside them.
	stripped := fencedCodeBlockPattern.ReplaceAllString(markdown, "")
	matches := markdownLinkAllPattern.FindAllStringSubmatch(stripped, -1)
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

		if cases[i].Code == nil {
			continue
		}
		for _, captureName := range cases[i].Code.Block.CaptureNames {
			bindings = append(bindings, bindingDefinition{
				Name:        captureName,
				HeadingPath: append([]string(nil), cases[i].ID.HeadingPath...),
			})
		}
	}

	if err := validateProseVariables(doc); err != nil {
		return DocumentPlan{}, err
	}

	cases = append(cases, alloyChecks...)

	return DocumentPlan{
		Document:    doc,
		Cases:       cases,
		Hooks:       hooks,
		AlloyModels: alloyModels,
	}, nil
}

// validateProseVariables walks document nodes in order, accumulating bindings
// from code blocks and checking that prose ${var} references are resolvable.
func validateProseVariables(doc Document) error {
	bindings := make([]bindingDefinition, 0)
	for _, node := range doc.Nodes {
		bindings = appendNodeBindings(bindings, node)
		if prose, ok := node.(ProseNode); ok {
			if err := checkProseRefs(doc.RelativeTo, prose, bindings); err != nil {
				return err
			}
		}
	}
	return nil
}

func appendNodeBindings(bindings []bindingDefinition, node Node) []bindingDefinition {
	block, ok := node.(CodeBlockNode)
	if !ok || block.ID == nil {
		return bindings
	}
	for _, name := range block.Block.CaptureNames {
		bindings = append(bindings, bindingDefinition{
			Name:        name,
			HeadingPath: append([]string(nil), block.ID.HeadingPath...),
		})
	}
	return bindings
}

func checkProseRefs(file string, prose ProseNode, bindings []bindingDefinition) error {
	for _, name := range proseVariableReferences(prose.Raw) {
		if !bindingVisible(bindings, name, prose.HeadingPath) {
			return fmt.Errorf("%s: unresolved variable %q in prose", file, name)
		}
	}
	return nil
}

type bindingDefinition struct {
	Name        string
	HeadingPath HeadingPath
}

func bindingVisible(bindings []bindingDefinition, name string, currentPath HeadingPath) bool {
	for i := len(bindings) - 1; i >= 0; i-- {
		if bindings[i].Name == name && bindings[i].HeadingPath.Reachable(currentPath) {
			return true
		}
	}
	return false
}

func documentMaxOrdinal(doc Document) int {
	maxOrd := 0
	for _, id := range documentOrdinals(doc) {
		if id != nil && id.Ordinal > maxOrd {
			maxOrd = id.Ordinal
		}
	}
	return maxOrd
}

func documentOrdinals(doc Document) []*SpecID {
	var ids []*SpecID
	for _, node := range doc.Nodes {
		switch n := node.(type) {
		case CodeBlockNode:
			ids = append(ids, n.ID)
		case AlloyRefNode:
			ids = append(ids, n.ID)
		case CheckCallNode:
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
		case CheckCallNode:
			cases = appendCheckCallCase(cases, current)
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
				ID:   *inline.ID,
				Kind: CaseKindInlineExpect,
				InlineExpect: &InlineExpectCaseSpec{
					Template:    inline.ExpectExpr,
					ExpectValue: inline.ExpectValue,
					ExpectFail:  inline.ExpectFail,
				},
			})
		case InlineCheck:
			cases = append(cases, CaseSpec{
				ID:   *inline.ID,
				Kind: CaseKindTableRow,
				TableRow: &TableRowCaseSpec{
					Check:       inline.Check,
					CheckParams: inline.CheckParams,
				},
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
		ID:   *block.ID,
		Kind: CaseKindCode,
		Code: &CodeCaseSpec{
			Block:    block.Block,
			Template: block.Source,
		},
	})
}

func appendCheckCallCase(cases []CaseSpec, node CheckCallNode) []CaseSpec {
	if node.ID == nil {
		return cases
	}
	return append(cases, CaseSpec{
		ID:   *node.ID,
		Kind: CaseKindTableRow,
		TableRow: &TableRowCaseSpec{
			Check:       node.Check,
			CheckParams: node.CheckParams,
		},
	})
}

func appendTableCases(cases []CaseSpec, table TableNode) []CaseSpec {
	if table.Check == "" {
		return cases
	}
	for index, row := range table.Rows {
		if row.ID == nil {
			continue
		}
		cases = append(cases, CaseSpec{
			ID:   *row.ID,
			Kind: CaseKindTableRow,
			TableRow: &TableRowCaseSpec{
				Check:       table.Check,
				CheckParams: table.CheckParams,
				Columns:     append([]string(nil), table.Columns...),
				Cells:       append([]string(nil), row.Cells...),
				RowNumber:   index + 1,
			},
		})
	}
	return cases
}

var variableRefPattern = regexp.MustCompile(`(\\?)\$\{([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\}`)
var codeSpanPattern = regexp.MustCompile("``[\\s\\S]*?``|`[^`]+`")

// proseVariableReferences extracts variable references from prose text,
// excluding references inside backtick code spans.
func proseVariableReferences(raw string) []string {
	stripped := codeSpanPattern.ReplaceAllString(raw, "")
	return variableReferences(stripped)
}

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
		// Extract root name (before first dot) for scope checking
		name := match[2]
		if idx := strings.Index(name, "."); idx >= 0 {
			name = name[:idx]
		}
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
		return variableReferences(spec.Code.Template)
	case CaseKindInlineExpect:
		return mergeVariableReferences(spec.InlineExpect.Template, spec.InlineExpect.ExpectValue)
	case CaseKindTableRow:
		return mergeVariableReferences(spec.TableRow.Cells...)
	default:
		return nil
	}
}

func (c CaseSpec) TargetKey() string {
	switch c.Kind {
	case CaseKindCode:
		return c.Code.Block.Descriptor()
	case CaseKindInlineExpect:
		return "expect"
	case CaseKindAlloy:
		return "alloy"
	default:
		return c.TableRow.Check
	}
}

func (c CaseSpec) DisplayKind() string {
	switch c.Kind {
	case CaseKindCode:
		return c.Code.Block.Descriptor()
	case CaseKindInlineExpect:
		return "expect"
	case CaseKindAlloy:
		return "alloy:" + c.Alloy.Model + "#" + c.Alloy.Assertion
	default:
		return "check:" + c.TableRow.Check
	}
}

func (c CaseSpec) DefaultLabel() string {
	if c.Kind == CaseKindAlloy {
		a := c.Alloy
		suffix := "alloy:ref(" + a.Model + "#" + a.Assertion + ", scope=" + a.Scope + ")"
		if len(c.ID.HeadingPath) == 0 {
			return suffix
		}
		return suffix + " @ " + c.ID.HeadingPath[len(c.ID.HeadingPath)-1]
	}
	if len(c.ID.HeadingPath) == 0 {
		return c.DisplayKind()
	}
	suffix := c.ID.HeadingPath[len(c.ID.HeadingPath)-1]
	if c.Kind == CaseKindTableRow {
		return c.DisplayKind() + " @ " + suffix + " row " + fmt.Sprintf("%d", c.TableRow.RowNumber)
	}
	return c.DisplayKind() + " @ " + suffix
}
