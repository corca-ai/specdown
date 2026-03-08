package html

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

type reportView struct {
	Title string
	Meta  template.HTML
	Specs []specView
}

type specView struct {
	Title    string
	Anchor   string
	Status   string
	Headings []tocItemView
	Body     template.HTML
}

type tocItemView struct {
	Text     string
	Anchor   string
	Level    int
	Status   string
	Children []tocItemView
}

func Write(report core.Report, outPath string) (err error) {
	title := report.Title
	if title == "" {
		title = "Specification"
	}
	specs := make([]specView, 0, len(report.Results))
	for _, result := range report.Results {
		body, err := renderDocument(result)
		if err != nil {
			return fmt.Errorf("render %s: %w", result.Document.RelativeTo, err)
		}
		specs = append(specs, specView{
			Title:    result.Document.Title,
			Anchor:   core.HeadingAnchor(result.Document.RelativeTo, []string{result.Document.Title}),
			Status:   specStatusClass(result),
			Headings: collectHeadings(result),
			Body:     template.HTML(body),
		})
	}

	view := reportView{
		Title: title,
		Meta:  template.HTML(buildMeta(report)),
		Specs: specs,
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("create report dir: %w", err)
	}

	file, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create report: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close report: %w", cerr)
		}
	}()

	if err := pageTemplate.Execute(file, view); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

func collectHeadings(result core.DocumentResult) []tocItemView {
	statuses := collectHeadingStatuses(result)
	var roots []tocItemView
	for _, node := range result.Document.Nodes {
		heading, ok := node.(core.HeadingNode)
		if !ok {
			continue
		}
		if len(heading.HeadingPath) == 0 || heading.Level == 1 {
			continue
		}
		item := tocItemView{
			Text:   heading.Text,
			Anchor: core.HeadingAnchor(result.Document.RelativeTo, heading.HeadingPath),
			Level:  heading.Level,
			Status: tocStatusClass(statuses[headingPathKey(heading.HeadingPath)]),
		}
		if heading.Level == 2 {
			roots = append(roots, item)
		} else if len(roots) > 0 {
			roots[len(roots)-1].Children = append(roots[len(roots)-1].Children, item)
		}
	}
	return roots
}

func specStatusClass(result core.DocumentResult) string {
	for _, item := range result.Cases {
		if item.Status == core.StatusFailed {
			return "failed"
		}
	}
	for _, item := range result.AlloyChecks {
		if item.Status == core.StatusFailed {
			return "failed"
		}
	}
	return ""
}

func tocStatusClass(status core.Status) string {
	if status == core.StatusFailed {
		return string(status)
	}
	return ""
}

func collectHeadingStatuses(result core.DocumentResult) map[string]core.Status {
	statuses := make(map[string]core.Status)
	mark := func(path []string, status core.Status) {
		// Mark exact path and all ancestor paths
		for i := 1; i <= len(path); i++ {
			key := headingPathKey(path[:i])
			current := statuses[key]
			if current == core.StatusFailed {
				continue
			}
			if status == core.StatusFailed || current == "" {
				statuses[key] = status
			}
		}
	}

	for _, item := range result.Cases {
		mark(item.ID.HeadingPath, item.Status)
	}
	for _, item := range result.AlloyChecks {
		mark(item.ID.HeadingPath, item.Status)
	}
	return statuses
}

func headingPathKey(path []string) string {
	return strings.Join(path, "\x00")
}

func buildMeta(report core.Report) string {
	passed := report.Summary.CasesPassed + report.Summary.AlloyChecksPassed
	failed := report.Summary.CasesFailed + report.Summary.AlloyChecksFailed
	var b strings.Builder
	b.WriteString(`<p class="content-meta">`)
	b.WriteString(template.HTMLEscapeString(report.GeneratedAt.Format(time.RFC3339)))
	b.WriteString(`<span class="pill pass">pass `)
	fmt.Fprintf(&b, "%d", passed)
	b.WriteString(`</span>`)
	b.WriteString(`<span class="pill fail">fail `)
	fmt.Fprintf(&b, "%d", failed)
	b.WriteString(`</span>`)
	b.WriteString(`</p>`)
	return b.String()
}

func renderDocument(result core.DocumentResult) (string, error) {
	caseResults := make(map[string]core.CaseResult, len(result.Cases))
	for _, item := range result.Cases {
		caseResults[item.ID.Key()] = item
	}
	alloyResults := make(map[string]core.AlloyCheckResult, len(result.AlloyChecks))
	for _, item := range result.AlloyChecks {
		alloyResults[item.ID.Key()] = item
	}

	var out strings.Builder
	var sectionStack []int
	var accBindings []core.Binding
	for _, node := range result.Document.Nodes {
		sectionStack = closeSections(&out, sectionStack, node)
		var rendered string
		var err error
		if prose, ok := node.(core.ProseNode); ok {
			rendered, err = renderProseNode(prose, caseResults, accBindings)
		} else {
			rendered, err = renderNode(node, result.Document.RelativeTo, caseResults, alloyResults)
		}
		if err != nil {
			return "", err
		}
		out.WriteString(rendered)
		accBindings = accumulateNodeBindings(accBindings, node, caseResults)
	}
	for range sectionStack {
		out.WriteString(`</section>`)
	}
	return out.String(), nil
}

func closeSections(out *strings.Builder, sectionStack []int, node core.Node) []int {
	heading, ok := node.(core.HeadingNode)
	if !ok {
		return sectionStack
	}
	for len(sectionStack) > 0 && sectionStack[len(sectionStack)-1] >= heading.Level {
		out.WriteString(`</section>`)
		sectionStack = sectionStack[:len(sectionStack)-1]
	}
	fmt.Fprintf(out, `<section class="s%d">`, heading.Level)
	return append(sectionStack, heading.Level)
}

func renderNode(node core.Node, documentPath string, caseResults map[string]core.CaseResult, alloyResults map[string]core.AlloyCheckResult) (string, error) {
	switch current := node.(type) {
	case core.HeadingNode:
		return renderHeading(current, documentPath)
	case core.ProseNode:
		return markdownToHTML(current.Markdown())
	case core.CodeBlockNode:
		return renderCodeBlock(current, caseResults)
	case core.AlloyModelNode:
		return renderAlloyModel(current, alloyResults), nil
	case core.AlloyRefNode:
		return renderAlloyRef(current, alloyResults)
	case core.TableNode:
		return renderTable(current, caseResults)
	case core.HookNode:
		return renderHookBlock(current), nil
	case core.FixtureCallNode:
		return renderFixtureCall(current, caseResults), nil
	default:
		return "", fmt.Errorf("unknown node type %T", node)
	}
}

func renderCodeBlock(node core.CodeBlockNode, caseResults map[string]core.CaseResult) (string, error) {
	if node.ID == nil {
		return markdownToHTML(node.Markdown())
	}

	result, ok := caseResults[node.ID.Key()]
	if !ok {
		return markdownToHTML(node.Markdown())
	}

	var out strings.Builder
	out.WriteString(`<section class="exec-block `)
	out.WriteString(template.HTMLEscapeString(string(result.Status)))
	out.WriteString(`" id="`)
	out.WriteString(template.HTMLEscapeString(node.ID.Anchor()))
	out.WriteString(`">`)
	out.WriteString(`<div class="exec-source">`)
	if strings.HasPrefix(result.Block, "doctest:") && len(result.Steps) > 0 {
		renderDoctestSteps(&out, result.Steps)
	} else {
		renderCodeSource(&out, result)
	}
	renderVisibleBindings(&out, result.VisibleBindings)
	renderBindings(&out, result.Bindings)
	out.WriteString(`</div>`)
	out.WriteString(`<p class="exec-block-footer">`)
	out.WriteString(template.HTMLEscapeString(result.Block))
	out.WriteString(`</p>`)
	out.WriteString(`</section>`)
	return out.String(), nil
}

func renderCodeSource(out *strings.Builder, result core.CaseResult) {
	source := result.Template
	if result.RenderedSource != "" {
		source = result.RenderedSource
	}
	out.WriteString(`<code>`)
	out.WriteString(template.HTMLEscapeString(source))
	out.WriteString(`</code>`)
	if result.Status == core.StatusFailed && (result.Message != "" || result.Expected != "" || result.Actual != "") {
		renderFailureDiff(out, result.Message, result.Expected, result.Actual)
	}
}

func renderVisibleBindings(out *strings.Builder, bindings []core.Binding) {
	if len(bindings) == 0 {
		return
	}
	out.WriteString(`<div class="exec-bindings visible-bindings">`)
	for i, b := range bindings {
		if i > 0 {
			out.WriteString(`, `)
		}
		out.WriteString(`$`)
		out.WriteString(template.HTMLEscapeString(b.Name))
		out.WriteString(`=`)
		out.WriteString(template.HTMLEscapeString(b.Value))
	}
	out.WriteString(`</div>`)
}

func renderBindings(out *strings.Builder, bindings []core.Binding) {
	if len(bindings) == 0 {
		return
	}
	out.WriteString(`<div class="exec-bindings">`)
	for i, b := range bindings {
		if i > 0 {
			out.WriteString(`, `)
		}
		out.WriteString(template.HTMLEscapeString(b.Name))
		out.WriteString(`=`)
		out.WriteString(template.HTMLEscapeString(b.Value))
	}
	out.WriteString(`</div>`)
}

func renderHeading(node core.HeadingNode, documentPath string) (string, error) {
	html, err := markdownToHTML(node.Markdown())
	if err != nil {
		return "", err
	}
	rendered := node.Level + 1
	if rendered > 6 {
		rendered = 6
	}
	openTag := fmt.Sprintf("<h%d>", node.Level)
	closeTag := fmt.Sprintf("</h%d>", node.Level)
	var replacement string
	if len(node.HeadingPath) > 0 {
		replacement = fmt.Sprintf("<h%d id=%q>", rendered, template.HTMLEscapeString(core.HeadingAnchor(documentPath, node.HeadingPath)))
	} else {
		replacement = fmt.Sprintf("<h%d>", rendered)
	}
	closeReplacement := fmt.Sprintf("</h%d>", rendered)
	result := strings.Replace(html, openTag, replacement, 1)
	result = strings.Replace(result, closeTag, closeReplacement, 1)
	return result, nil
}

func renderTable(node core.TableNode, caseResults map[string]core.CaseResult) (string, error) {
	if node.Fixture == "" {
		return markdownToHTML(node.Markdown())
	}

	type renderedRow struct {
		node   core.TableRowNode
		result core.CaseResult
	}

	rows := make([]renderedRow, 0, len(node.Rows))
	tableStatus := ""
	for _, row := range node.Rows {
		if row.ID == nil {
			continue
		}
		result, ok := caseResults[row.ID.Key()]
		if !ok {
			return markdownToHTML(node.Markdown())
		}
		rows = append(rows, renderedRow{node: row, result: result})
		if result.Status == core.StatusFailed {
			tableStatus = string(core.StatusFailed)
		} else if tableStatus == "" {
			tableStatus = string(core.StatusPassed)
		}
	}

	var out strings.Builder
	out.WriteString(`<section class="exec-table-block`)
	if tableStatus != "" {
		out.WriteString(` `)
		out.WriteString(template.HTMLEscapeString(tableStatus))
	}
	out.WriteString(`">`)
	out.WriteString(`<table class="exec-table">`)
	out.WriteString(`<thead><tr>`)
	for _, column := range node.Columns {
		out.WriteString(`<th>`)
		out.WriteString(template.HTMLEscapeString(column))
		out.WriteString(`</th>`)
	}
	out.WriteString(`</tr></thead>`)
	out.WriteString(`<tbody>`)
	for _, item := range rows {
		renderTableRow(&out, item.node, item.result)
	}
	out.WriteString(`</tbody></table>`)
	out.WriteString(`<p class="exec-table-footer">fixture:`)
	out.WriteString(template.HTMLEscapeString(node.Fixture))
	out.WriteString(`</p>`)
	out.WriteString(`</section>`)
	return out.String(), nil
}

func renderTableRow(out *strings.Builder, row core.TableRowNode, result core.CaseResult) {
	out.WriteString(`<tr class="`)
	out.WriteString(template.HTMLEscapeString(string(result.Status)))
	out.WriteString(`" id="`)
	out.WriteString(template.HTMLEscapeString(row.ID.Anchor()))
	out.WriteString(`">`)
	cells := result.TemplateCells
	if len(result.RenderedCells) == len(cells) {
		cells = result.RenderedCells
	}
	lastIndex := len(cells) - 1
	for index, cell := range cells {
		out.WriteString(`<td>`)
		out.WriteString(`<div class="cell-template">`)
		out.WriteString(template.HTMLEscapeString(core.UnescapeCell(cell)))
		out.WriteString(`</div>`)
		if result.Status == core.StatusFailed && index == lastIndex {
			renderFailureDiff(out, result.Message, result.Expected, result.Actual)
		}
		out.WriteString(`</td>`)
	}
	out.WriteString(`</tr>`)
}

func renderFailureDiff(out *strings.Builder, message, expected, actual string) {
	if message == "" && expected == "" && actual == "" {
		return
	}
	if expected == "" && actual == "" {
		out.WriteString(`<div class="cell-actual">`)
		out.WriteString(template.HTMLEscapeString(message))
		out.WriteString(`</div>`)
		return
	}
	out.WriteString(`<dl class="failure-diff compact">`)
	if message != "" {
		out.WriteString(`<dt>error</dt><dd>`)
		out.WriteString(template.HTMLEscapeString(message))
		out.WriteString(`</dd>`)
	}
	if expected != "" {
		out.WriteString(`<dt>expected</dt><dd>`)
		out.WriteString(template.HTMLEscapeString(expected))
		out.WriteString(`</dd>`)
	}
	if actual != "" {
		out.WriteString(`<dt>actual</dt><dd>`)
		out.WriteString(template.HTMLEscapeString(actual))
		out.WriteString(`</dd>`)
	}
	out.WriteString(`</dl>`)
}

func renderAlloyModel(node core.AlloyModelNode, alloyResults map[string]core.AlloyCheckResult) string {
	// Find checks targeting this model
	var failedResult *core.AlloyCheckResult
	hasCheck := false
	for _, r := range alloyResults {
		if r.Model != node.Model {
			continue
		}
		hasCheck = true
		if r.Status == core.StatusFailed {
			rCopy := r
			failedResult = &rCopy
			break
		}
	}

	statusClass := ""
	if hasCheck {
		statusClass = " passed"
		if failedResult != nil {
			statusClass = " failed"
		}
	}

	var out strings.Builder
	out.WriteString(`<section class="exec-block alloy-model` + statusClass + `">`)
	out.WriteString(`<div class="exec-source">`)
	out.WriteString(`<pre><code>`)
	out.WriteString(template.HTMLEscapeString(node.Source))
	out.WriteString(`</code></pre>`)
	if failedResult != nil {
		msg := failedResult.Message
		if msg != "" {
			out.WriteString(`<div class="cell-actual">`)
			out.WriteString(template.HTMLEscapeString(msg))
			out.WriteString(`</div>`)
		}
	}
	out.WriteString(`</div>`)
	out.WriteString(`<p class="exec-block-footer">`)
	out.WriteString(template.HTMLEscapeString("alloy:model(" + node.Model + ")"))
	out.WriteString(`</p>`)
	out.WriteString(`</section>`)
	return out.String()
}

func renderHookBlock(node core.HookNode) string {
	label := string(node.Hook)
	if node.Each {
		label += ":each"
	}
	var out strings.Builder
	out.WriteString(`<section class="exec-block hook-block">`)
	out.WriteString(`<div class="exec-source">`)
	out.WriteString(`<code>`)
	out.WriteString(template.HTMLEscapeString(node.Source))
	out.WriteString(`</code>`)
	out.WriteString(`</div>`)
	out.WriteString(`<p class="exec-block-footer">`)
	out.WriteString(template.HTMLEscapeString(label + " · " + node.Block.Descriptor()))
	out.WriteString(`</p>`)
	out.WriteString(`</section>`)
	return out.String()
}

func renderFixtureCall(node core.FixtureCallNode, caseResults map[string]core.CaseResult) string {
	if node.ID == nil {
		return ""
	}
	result, ok := caseResults[node.ID.Key()]
	if !ok {
		return ""
	}

	var out strings.Builder
	out.WriteString(`<section class="exec-block fixture-call `)
	out.WriteString(template.HTMLEscapeString(string(result.Status)))
	out.WriteString(`" id="`)
	out.WriteString(template.HTMLEscapeString(node.ID.Anchor()))
	out.WriteString(`">`)
	out.WriteString(`<div class="exec-source">`)
	out.WriteString(`<code>`)
	label := node.Fixture
	if len(node.FixtureParams) > 0 {
		var params []string
		for k, v := range node.FixtureParams {
			params = append(params, k+"="+v)
		}
		sort.Strings(params)
		label += "(" + strings.Join(params, ", ") + ")"
	}
	out.WriteString(template.HTMLEscapeString(label))
	out.WriteString(`</code>`)
	if result.Status == core.StatusFailed && (result.Message != "" || result.Expected != "" || result.Actual != "") {
		renderFailureDiff(&out, result.Message, result.Expected, result.Actual)
	}
	out.WriteString(`</div>`)
	out.WriteString(`<p class="exec-block-footer">fixture</p>`)
	out.WriteString(`</section>`)
	return out.String()
}

func renderAlloyRef(node core.AlloyRefNode, alloyResults map[string]core.AlloyCheckResult) (string, error) {
	// Alloy failures are now shown inline in the model block.
	return "", nil
}

// nodeSpecIDs returns the SpecIDs from executable nodes.
func nodeSpecIDs(node core.Node) []*core.SpecID {
	switch n := node.(type) {
	case core.CodeBlockNode:
		return []*core.SpecID{n.ID}
	case core.TableNode:
		ids := make([]*core.SpecID, len(n.Rows))
		for i := range n.Rows {
			ids[i] = n.Rows[i].ID
		}
		return ids
	case core.FixtureCallNode:
		return []*core.SpecID{n.ID}
	case core.ProseNode:
		var ids []*core.SpecID
		for i := range n.Inlines {
			ids = append(ids, n.Inlines[i].ID)
		}
		return ids
	default:
		return nil
	}
}

func mergeBindings(bindings []core.Binding, newBindings []core.Binding) []core.Binding {
	for _, b := range newBindings {
		found := false
		for i, existing := range bindings {
			if existing.Name == b.Name {
				bindings[i] = b
				found = true
				break
			}
		}
		if !found {
			bindings = append(bindings, b)
		}
	}
	return bindings
}

func accumulateNodeBindings(bindings []core.Binding, node core.Node, caseResults map[string]core.CaseResult) []core.Binding {
	for _, id := range nodeSpecIDs(node) {
		if id == nil {
			continue
		}
		if cr, ok := caseResults[id.Key()]; ok && cr.Status == core.StatusPassed {
			bindings = mergeBindings(bindings, cr.Bindings)
		}
	}
	return bindings
}

var htmlCodeExpectPattern = regexp.MustCompile(`<code>expect:\s*(.+?)\s*==\s*(.+?)\s*</code>`)
var htmlCodeFixturePattern = regexp.MustCompile(`<code>fixture:([A-Za-z0-9_-]+)\(([^)]*)\)</code>`)
var proseVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
var htmlCodeTagPattern = regexp.MustCompile(`<code[^>]*>[^<]*</code>`)

func renderProseNode(node core.ProseNode, caseResults map[string]core.CaseResult, accBindings []core.Binding) (string, error) {
	html, err := markdownToHTML(node.Markdown())
	if err != nil {
		return "", err
	}

	// Replace <code>expect: ... == ...</code> with inline expect result spans
	expectIdx := 0
	expects := filterInlinesByKind(node.Inlines, core.InlineExpect)
	html = htmlCodeExpectPattern.ReplaceAllStringFunc(html, func(match string) string {
		if expectIdx >= len(expects) {
			return match
		}
		inline := expects[expectIdx]
		expectIdx++
		if inline.ID == nil {
			return match
		}
		cr, ok := caseResults[inline.ID.Key()]
		if !ok {
			return match
		}
		return renderInlineExpectSpan(cr)
	})

	// Replace <code>fixture:name(params)</code> with inline fixture result spans
	fixtureIdx := 0
	fixtures := filterInlinesByKind(node.Inlines, core.InlineFixture)
	html = htmlCodeFixturePattern.ReplaceAllStringFunc(html, func(match string) string {
		if fixtureIdx >= len(fixtures) {
			return match
		}
		inline := fixtures[fixtureIdx]
		fixtureIdx++
		if inline.ID == nil {
			return match
		}
		cr, ok := caseResults[inline.ID.Key()]
		if !ok {
			return match
		}
		return renderInlineFixtureSpan(inline, cr)
	})

	// Replace ${var} in non-<code> parts with variable display spans
	bindingMap := make(map[string]string, len(accBindings))
	for _, b := range accBindings {
		bindingMap[b.Name] = b.Value
	}
	html = replaceProseVariables(html, bindingMap)

	return html, nil
}

func filterInlinesByKind(inlines []core.InlineElement, kind core.InlineKind) []core.InlineElement {
	var result []core.InlineElement
	for _, inline := range inlines {
		if inline.Kind == kind {
			result = append(result, inline)
		}
	}
	return result
}

func renderInlineExpectSpan(cr core.CaseResult) string {
	var out strings.Builder
	switch cr.Status {
	case core.StatusPassed:
		out.WriteString(`<span class="inline-expect passed" title="`)
		out.WriteString(template.HTMLEscapeString("expected " + cr.Expected))
		out.WriteString(`">`)
		out.WriteString(template.HTMLEscapeString(cr.Actual))
		out.WriteString(`</span>`)
	default:
		out.WriteString(`<ruby class="inline-expect failed" title="`)
		out.WriteString(template.HTMLEscapeString(cr.Message))
		out.WriteString(`">`)
		out.WriteString(template.HTMLEscapeString(cr.Actual))
		out.WriteString(`<rp>(</rp><rt>`)
		out.WriteString(template.HTMLEscapeString(cr.Expected))
		out.WriteString(`</rt><rp>)</rp></ruby>`)
	}
	return out.String()
}

func renderInlineFixtureSpan(inline core.InlineElement, cr core.CaseResult) string {
	var out strings.Builder
	if cr.Actual != "" {
		// Ruby: fixture name as annotation, actual value as main content
		out.WriteString(`<ruby class="inline-fixture `)
		out.WriteString(template.HTMLEscapeString(string(cr.Status)))
		out.WriteString(`" title="`)
		if cr.Status == core.StatusFailed && cr.Message != "" {
			out.WriteString(template.HTMLEscapeString(cr.Message))
		} else {
			out.WriteString(template.HTMLEscapeString(inline.Raw))
		}
		out.WriteString(`">`)
		out.WriteString(template.HTMLEscapeString(cr.Actual))
		out.WriteString(`<rp>(</rp><rt>`)
		out.WriteString(template.HTMLEscapeString(inline.Fixture))
		out.WriteString(`</rt><rp>)</rp></ruby>`)
	} else {
		out.WriteString(`<span class="inline-fixture `)
		out.WriteString(template.HTMLEscapeString(string(cr.Status)))
		out.WriteString(`" title="`)
		if cr.Status == core.StatusFailed && cr.Message != "" {
			out.WriteString(template.HTMLEscapeString(cr.Message))
		} else {
			out.WriteString(template.HTMLEscapeString(inline.Raw))
		}
		out.WriteString(`">`)
		out.WriteString(template.HTMLEscapeString(inline.Fixture))
		out.WriteString(`</span>`)
	}
	return out.String()
}

func replaceProseVariables(html string, bindings map[string]string) string {
	if len(bindings) == 0 {
		return html
	}
	// Split by <code>...</code> segments to avoid replacing inside code spans
	codeLocs := htmlCodeTagPattern.FindAllStringIndex(html, -1)
	if len(codeLocs) == 0 {
		return replaceVarRefs(html, bindings)
	}
	var out strings.Builder
	lastEnd := 0
	for _, loc := range codeLocs {
		out.WriteString(replaceVarRefs(html[lastEnd:loc[0]], bindings))
		out.WriteString(html[loc[0]:loc[1]])
		lastEnd = loc[1]
	}
	out.WriteString(replaceVarRefs(html[lastEnd:], bindings))
	return out.String()
}

func replaceVarRefs(text string, bindings map[string]string) string {
	return proseVarPattern.ReplaceAllStringFunc(text, func(match string) string {
		name := proseVarPattern.FindStringSubmatch(match)[1]
		value, ok := bindings[name]
		if !ok {
			return match
		}
		return `<span class="inline-var" title="$` +
			template.HTMLEscapeString(name) + `">` +
			template.HTMLEscapeString(value) + `</span>`
	})
}


func renderDoctestSteps(out *strings.Builder, steps []core.DoctestStep) {
	out.WriteString(`<div class="doctest-steps">`)
	for _, step := range steps {
		out.WriteString(`<div class="doctest-command">`)
		out.WriteString(`<span class="doctest-prompt">$ </span>`)
		out.WriteString(template.HTMLEscapeString(step.Command))
		out.WriteString(`</div>`)
		if step.Status == core.StatusPassed {
			if step.Actual != "" {
				renderDoctestPassedOutput(out, step)
			}
		} else {
			if step.Actual != "" {
				out.WriteString(`<div class="doctest-output failed">`)
				out.WriteString(template.HTMLEscapeString(step.Actual))
				out.WriteString(`</div>`)
			}
			if step.Expected != "" {
				out.WriteString(`<div class="doctest-expected">`)
				out.WriteString(template.HTMLEscapeString(step.Expected))
				out.WriteString(` <span class="doctest-expected-label">(expected)</span>`)
				out.WriteString(`</div>`)
			}
		}
	}
	out.WriteString(`</div>`)
}

// renderDoctestPassedOutput renders actual output for a passed step.
// When the expected output contains "..." wildcards, the matched lines
// are collapsed into an expandable "... (N lines)" summary.
func renderDoctestPassedOutput(out *strings.Builder, step core.DoctestStep) {
	segments := annotateWildcard(step.Expected, step.Actual)
	if segments == nil {
		// No wildcards — render as before.
		out.WriteString(`<div class="doctest-output passed">`)
		out.WriteString(template.HTMLEscapeString(step.Actual))
		out.WriteString(`</div>`)
		return
	}
	out.WriteString(`<div class="doctest-output passed">`)
	for _, seg := range segments {
		if !seg.Wildcard {
			out.WriteString(template.HTMLEscapeString(seg.Text))
		} else {
			unit := "lines"
			if seg.Lines == 1 {
				unit = "line"
			}
			summary := fmt.Sprintf("... (%d %s)", seg.Lines, unit)
			out.WriteString(`<details class="wildcard-fold"><summary>`)
			out.WriteString(template.HTMLEscapeString(summary))
			out.WriteString(`</summary><span class="wildcard-expanded">`)
			out.WriteString(template.HTMLEscapeString(seg.Text))
			out.WriteString(`</span></details>`)
		}
	}
	out.WriteString(`</div>`)
}

// wildcardSegment represents a contiguous chunk of actual output,
// either literally matched or absorbed by a "..." wildcard.
type wildcardSegment struct {
	Text     string
	Lines    int
	Wildcard bool
}

// annotateWildcard aligns expected (which may contain "..." lines)
// against actual and returns segments. Returns nil if no wildcards.
func annotateWildcard(expected, actual string) []wildcardSegment {
	expectedLines := strings.Split(expected, "\n")
	if !hasWildcardLine(expectedLines) {
		return nil
	}
	actualLines := strings.Split(actual, "\n")
	mapping := matchWildcardMapping(actualLines, expectedLines, 0, 0)
	if mapping == nil {
		return nil
	}
	return buildSegments(actualLines, mapping)
}

func hasWildcardLine(lines []string) bool {
	for _, l := range lines {
		if l == "..." {
			return true
		}
	}
	return false
}

// matchWildcardMapping returns a per-actual-line boolean slice where
// true means the line was consumed by a "..." wildcard.
func matchWildcardMapping(actual, expected []string, ai, ei int) []bool {
	mapping := make([]bool, len(actual))
	if !doMatch(actual, expected, ai, ei, mapping) {
		return nil
	}
	return mapping
}

func skipWildcards(expected []string, ei int) int {
	for ei < len(expected) && expected[ei] == "..." {
		ei++
	}
	return ei
}

func markWildcard(mapping []bool, from, to int) {
	for i := from; i < to; i++ {
		mapping[i] = true
	}
}

func snapshotMapping(mapping []bool) []bool {
	snap := make([]bool, len(mapping))
	copy(snap, mapping)
	return snap
}

func doMatchWildcard(actual, expected []string, ai, ei int, mapping []bool) bool {
	ei = skipWildcards(expected, ei)
	if ei >= len(expected) {
		markWildcard(mapping, ai, len(actual))
		return true
	}
	for tryAi := ai; tryAi <= len(actual); tryAi++ {
		snap := snapshotMapping(mapping)
		markWildcard(snap, ai, tryAi)
		if doMatch(actual, expected, tryAi, ei, snap) {
			copy(mapping, snap)
			return true
		}
	}
	return false
}

func doMatch(actual, expected []string, ai, ei int, mapping []bool) bool {
	for ei < len(expected) {
		if expected[ei] == "..." {
			return doMatchWildcard(actual, expected, ai, ei, mapping)
		}
		if ai >= len(actual) || actual[ai] != expected[ei] {
			return false
		}
		ai++
		ei++
	}
	return ai >= len(actual)
}

func collectRun(mapping []bool, start int, want bool) int {
	j := start
	for j < len(mapping) && mapping[j] == want {
		j++
	}
	return j
}

func buildSegments(actualLines []string, mapping []bool) []wildcardSegment {
	var segments []wildcardSegment
	i := 0
	for i < len(actualLines) {
		isWild := mapping[i]
		j := collectRun(mapping, i, isWild)
		text := strings.Join(actualLines[i:j], "\n")
		if j < len(actualLines) {
			text += "\n"
		}
		segments = append(segments, wildcardSegment{
			Text:     text,
			Lines:    j - i,
			Wildcard: isWild,
		})
		i = j
	}
	return segments
}

var mdConverter = goldmark.New(goldmark.WithExtensions(extension.Table))

func markdownToHTML(source string) (string, error) {
	var out bytes.Buffer
	if err := mdConverter.Convert([]byte(source), &out); err != nil {
		return "", err
	}
	return out.String(), nil
}

var pageTemplate = template.Must(template.New("report").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
  <title>{{ .Title }}</title>
  <style>
    /* ── Design tokens ── */
    :root {
      color-scheme: light;
      --bg: #f3f3f0;
      --paper: #fcfcfa;
      --ink: #1f1f1b;
      --muted: #66665f;
      --rule: #d6d6cf;
      --pass-ink: #0a8f3b;
      --pass-mark: #10b34a;
      --fail-ink: #a1261a;
      --fail-mark: #d63b2d;
      --accent: #2f64b3;
      --code-bg: #efefea;
      --note-bg: #f5f5f1;
      --pass-bg: #e8f0e6;
      --fail-bg: #f0e4e2;
      --font-mono: "SFMono-Regular", Menlo, Consolas, monospace;

      --safe-top: env(safe-area-inset-top, 0px);
      --safe-right: env(safe-area-inset-right, 0px);
      --safe-bottom: env(safe-area-inset-bottom, 0px);
      --safe-left: env(safe-area-inset-left, 0px);
    }

    /* ── Reset ── */
    *, *::before, *::after { box-sizing: border-box; margin: 0; }

    /* ── Body ── */
    body {
      font-family: "Avenir Next", "Helvetica Neue", "Segoe UI", sans-serif;
      color: var(--ink);
      background: var(--bg);
    }

    /* ── Page layout ── */
    main {
      max-width: 78rem;
      margin-inline: auto;
      padding:
        calc(2.75rem + var(--safe-top))
        calc(1.5rem + var(--safe-right))
        calc(4rem + var(--safe-bottom))
        calc(1.5rem + var(--safe-left));
    }

    .layout {
      display: grid;
      grid-template-columns: 16rem minmax(0, 54rem);
      column-gap: 2.5rem;
      align-items: baseline;
    }

    /* ── Table of contents ── */
    .toc {
      position: sticky;
      top: calc(1.5rem + var(--safe-top));
      max-height: calc(100dvh - var(--safe-top) - var(--safe-bottom));
      overflow-y: auto;
      font-size: 0.82rem;
      line-height: 1.45;
    }

    .toc-inner { padding: 0 0 0 0.85rem; }

    .toc-title {
      margin-bottom: 0.75rem;
      color: var(--muted);
      font-size: 0.78rem;
      font-weight: 600;
      letter-spacing: 0.08em;
      text-transform: uppercase;
    }

    .toc-spec {
      margin-bottom: 1.25rem;
      &:last-child { margin-bottom: 0; }
    }

    .toc-spec-title {
      display: block;
      margin-bottom: 0.35rem;
      font-weight: 600;
      color: var(--ink);
      text-decoration: none;
      position: relative;
    }

    .toc-list {
      list-style: none;
      padding-left: 0.85rem;
      display: none;
    }

    .toc-spec.expanded > .toc-list { display: block; }

    .toc-item {
      margin: 0.1rem 0;
      &:first-child { margin-top: 0; }
    }

    .toc-link {
      display: block;
      text-decoration: none;
      color: var(--muted);
      position: relative;
      transition: color 120ms ease;

      &:hover { color: var(--ink); }
      &.active { color: var(--ink); font-weight: 600; }
    }

    :is(.toc-spec-title, .toc-link).failed::before {
      content: "";
      position: absolute;
      left: -0.85rem;
      top: 50%;
      translate: 0 -50%;
      width: 0.38rem;
      height: 0.38rem;
      border-radius: 50%;
      background: var(--fail-mark);
    }

    .toc-level-4 { padding-left: 0.7rem; }
    .toc-level-5,
    .toc-level-6 { padding-left: 1.4rem; }

    .toc-children {
      list-style: none;
      padding-left: 0.85rem;
      display: none;
    }

    .toc-item.expanded > .toc-children { display: block; }

    /* ── Content area ── */
    .content { min-width: 0; }

    .report-title {
      font-family: Iowan Old Style, Palatino Linotype, Book Antiqua, Georgia, serif;
      font-size: 2.8rem;
      line-height: 1.15;
      letter-spacing: -0.01em;
      margin-bottom: 0.4rem;
    }

    .content-meta {
      margin-bottom: 2rem;
      color: var(--muted);
      font-size: 0.82rem;
      line-height: 1.65;
    }

    .pill {
      &::before { content: "· "; color: var(--muted); }
      &.pass { color: var(--pass-ink); }
      &.fail { color: var(--fail-ink); }
    }

    /* ── Spec articles ── */
    .spec + .spec { padding-top: 2rem; }

    .spec-body {
      line-height: 1.9;

      & > :first-child { margin-top: 0; }

      /* ── Sticky headings ──
         Each level stacks below the ones above it.
         The border-bottom separates the heading bar from content.
         No ::before hacks — just padding + solid background. */
      /* Sticky headings stack below each other.
         All levels share em-based padding so height = font-size × 2 + 1px border. */
      & :is(h2, h3, h4, h5, h6) {
        font-family: Iowan Old Style, Palatino Linotype, Book Antiqua, Georgia, serif;
        line-height: 1.15;
        padding: 0.5em 0 0.35em;
        margin-top: 0.75em;
        text-wrap: balance;
        letter-spacing: -0.01em;
        position: sticky;
        background: var(--bg);
        border-bottom: 1px solid color-mix(in srgb, var(--rule) 60%, transparent);
      }

      & :is(h2, h3, h4, h5, h6).stuck-last::after {
        content: "";
        position: absolute;
        inset: 100% 0 auto 0;
        height: 4px;
        background: linear-gradient(rgba(0, 0, 0, 0.06), transparent);
        pointer-events: none;
      }

      & > :first-child > :first-child { margin-top: 0; }

      & :is(p, ul, ol, dl, blockquote):not(.exec-block-footer):not(.exec-table-footer) { margin: 1.25rem 0 0; }
      & :is(p, ul, ol, dl, blockquote):not(.exec-block-footer):not(.exec-table-footer):first-child { margin-top: 0; }
      & pre { margin: 1rem 0; }
      & li { margin: 0.25rem 0; }
      & ul, & ol { padding-left: 1.5rem; }

      & h2 { font-size: 2.5rem; top: var(--safe-top); z-index: 4; scroll-margin-top: var(--safe-top); }
      & h3 { font-size: 1.85rem; top: calc(5rem + 1px + var(--safe-top)); z-index: 3; scroll-margin-top: calc(5rem + 1px + var(--safe-top)); }
      & h4 { font-size: 1.4rem;  top: calc(8.7rem + 2px + var(--safe-top)); z-index: 2; scroll-margin-top: calc(8.7rem + 2px + var(--safe-top)); }
      & :is(h5, h6) { font-size: 1.08rem; top: calc(11.5rem + 3px + var(--safe-top)); z-index: 1; scroll-margin-top: calc(11.5rem + 3px + var(--safe-top)); }
    }

    .status {
      font-weight: 600;
      font-size: 0.95rem;
      &.passed { color: var(--pass-ink); }
      &.failed { color: var(--fail-ink); }
    }

    /* ── Executable blocks ── */
    .exec-block,
    .exec-table-block {
      margin: 0.75rem 0;
    }

    .exec-block { scroll-margin-top: 1.5rem; }

    .exec-source {
      padding: 0.8rem 0.9rem;
      border: 1px solid var(--rule);
      border-radius: 0.2rem;
      border-left: 3px solid transparent;
      font-family: var(--font-mono);
      font-size: 0.92rem;
      line-height: 1.45;
      white-space: pre-wrap;
      overflow-x: auto;
      background: var(--code-bg);

      &.resolved {
        margin-top: 0.4rem;
        border: 1px solid var(--rule);
      }
    }

    .exec-block.passed > .exec-source:not(.resolved) {
      border-left-color: var(--pass-mark);
      background: var(--pass-bg);
    }

    .exec-block.failed > .exec-source:not(.resolved) {
      border-left-color: var(--fail-mark);
      background: var(--fail-bg);
    }

    .doctest-steps {
      font-family: var(--font-mono);
      font-size: 0.92rem;
      line-height: 1.45;
      white-space: pre-wrap;
    }

    .doctest-prompt { color: var(--muted); user-select: none; }

    .doctest-output.passed { color: var(--pass-ink); }
    .doctest-output.failed { color: var(--fail-ink); }

    .doctest-expected {
      color: var(--muted);
      font-style: italic;
    }

    .doctest-expected-label {
      font-size: 0.82rem;
    }

    .wildcard-fold {
      display: inline;
    }
    .wildcard-fold > summary {
      display: inline;
      cursor: pointer;
      color: var(--muted);
      font-style: italic;
      list-style: none;
    }
    .wildcard-fold > summary::-webkit-details-marker { display: none; }
    .wildcard-fold[open] > summary {
      display: block;
      color: var(--muted);
      opacity: 0.6;
    }
    .wildcard-expanded {
      border-left: 2px solid var(--pass-mark);
      padding-left: 0.6em;
      display: inline-block;
    }

    .exec-bindings {
      margin-top: 0.35rem;
      font-size: 0.85rem;
      font-style: italic;
      color: var(--muted);
      font-family: var(--font-mono);
    }

    :is(.exec-block-footer, .exec-table-footer) {
      text-align: right;
      font-size: 0.8rem;
      color: var(--muted);
      font-family: var(--font-mono);
    }

    .exec-note {
      margin: 0.75rem 0 0.35rem;
      color: var(--muted);
      font-size: 0.92rem;
    }

    .exec-message {
      margin-top: 0.75rem;
      color: var(--fail-ink);
      font-weight: 600;
    }

    /* ── Executable tables ── */
    .exec-table-block { overflow-x: auto; }

    .exec-table {
      width: 100%;
      border-collapse: collapse;
      font-size: 0.95rem;

      & :is(th, td) {
        padding: 0.7rem 0.75rem;
        border: 1px solid var(--rule);
        vertical-align: top;
        text-align: left;
      }

      & thead th {
        border: 0;
        padding-bottom: 0;
        font-weight: normal;
        font-size: 0.8rem;
        letter-spacing: 0.06em;
        text-transform: uppercase;
        color: var(--muted);
        background: var(--bg);
      }

      & thead th:first-child { border-left: 3px solid transparent; }
      & tbody td:first-child { border-left: 3px solid transparent; }

      & tbody tr.passed td { background: var(--pass-bg); }
      & tbody tr.passed td:first-child { border-left-color: var(--pass-mark); }
      & tbody tr.failed td { background: var(--fail-bg); }
      & tbody tr.failed td:first-child { border-left-color: var(--fail-mark); }
    }

    /* ── Prose code blocks & tables ── */
    .spec-body :not(.exec-source) > pre {
      padding: 0.8rem 0.9rem;
      background: var(--code-bg);
      border: 1px solid var(--rule);
      border-radius: 0.2rem;
      overflow-x: auto;
    }

    .spec-body table:not(.exec-table) {
      width: 100%;
      border-collapse: collapse;
      font-size: 0.95rem;
      margin: 1rem 0;
      overflow-x: auto;
      display: block;

      & :is(th, td) {
        padding: 0.5rem 0.75rem;
        border: 1px solid var(--rule);
        text-align: left;
      }

      & th {
        background: var(--code-bg);
        font-size: 0.85rem;
      }
    }

    /* ── Inline assertions ── */
    .inline-var {
      font-family: var(--font-mono);
      font-size: 0.94em;
      padding: 0.1em 0.35em;
      border-radius: 0.2rem;
      background: var(--pass-bg);
      color: var(--pass-ink);
    }

    .inline-expect {
      font-family: var(--font-mono);
      font-size: 0.94em;
      padding: 0.1em 0.35em;
      border-radius: 0.2rem;
    }

    .inline-expect.passed {
      background: var(--pass-bg);
      color: var(--pass-ink);
    }

    .inline-expect.failed {
      background: var(--fail-bg);
      color: var(--fail-ink);
      position: relative;
      padding-top: 0.02em;
      padding-bottom: 0.02em;
    }
    ruby.inline-expect.failed rt {
      position: absolute;
      left: 0;
      bottom: 100%;
      margin-bottom: 0;
      font-size: 0.8em;
      line-height: 1;
      color: var(--muted);
      font-style: italic;
      white-space: nowrap;
    }

    .spec-body :is(p, li):has(.inline-expect.failed, .inline-fixture.failed) {
      position: relative;
    }
    .spec-body :is(p, li):has(.inline-expect.failed, .inline-fixture.failed)::before {
      content: "";
      position: absolute;
      left: -0.85rem;
      top: 0.75em;
      width: 0.38rem;
      height: 0.38rem;
      border-radius: 50%;
      background: var(--fail-mark);
    }

    .inline-fixture {
      font-family: var(--font-mono);
      font-size: 0.94em;
      padding: 0.1em 0.35em;
      border-radius: 0.2rem;
    }

    .inline-fixture.passed {
      background: var(--pass-bg);
      color: var(--pass-ink);
    }

    .inline-fixture.failed {
      background: var(--fail-bg);
      color: var(--fail-ink);
    }

    ruby.inline-fixture {
      position: relative;
      padding-top: 0.02em;
      padding-bottom: 0.02em;
    }
    ruby.inline-fixture rt {
      position: absolute;
      left: 0;
      bottom: 100%;
      margin-bottom: 0;
      font-size: 0.8em;
      line-height: 1;
      color: var(--muted);
      font-style: italic;
      white-space: nowrap;
    }

    /* ── Cell styles ── */
    .cell-template { font-family: var(--font-mono); white-space: pre-wrap; }

    .cell-resolved {
      margin-top: 0.35rem;
      color: var(--muted);
      font-family: var(--font-mono);
      font-size: 0.92rem;
    }

    .cell-actual {
      margin-top: 0.35rem;
      color: var(--fail-ink);
      font-size: 0.85rem;
      font-style: italic;
      white-space: pre-wrap;
    }

    .failure-diff {
      margin: 0.75rem 0 0;
      padding: 0.65rem 0.8rem;
      background: var(--fail-bg);
      border-left: 3px solid var(--fail-mark);
      display: grid;
      grid-template-columns: auto 1fr;
      gap: 0.35rem 0.75rem;
      align-items: baseline;

      &.compact { padding: 0.65rem 0.75rem; border-left: 0; }

      & dt {
        color: var(--muted);
        font-size: 0.82rem;
        line-height: 1.45;
        text-transform: uppercase;
        letter-spacing: 0.03em;
      }

      & dd {
        font-family: var(--font-mono);
        line-height: 1.45;
        word-break: break-word;
      }
    }

    /* ── Links & code ── */
    a { color: var(--accent); }

    code, pre, kbd, samp {
      font-family: var(--font-mono);
      font-size: 0.94em;
      line-height: 1.45;
    }

    :not(pre) > code {
      padding: 0.15em 0.35em;
      background: #e6e6df;
      border-radius: 0.2rem;
    }

    .exec-source :not(pre) > code,
    .exec-source > code {
      padding: 0;
      background: transparent;
    }

    /* ── Mobile layout ── */
    @media (max-width: 960px) {
      .layout {
        grid-template-columns: minmax(0, 1fr);
        gap: 0;
      }

      .content { display: contents; }

      .toc {
        position: static;
        order: 2;
        margin-bottom: 1.5rem;
      }

      .toc-inner { padding-left: 0; padding-bottom: 1rem; }
      .content-header { order: 1; }
      .content-body { order: 3; }
    }

    .site-footer {
      max-width: 52rem;
      margin: 3rem auto 0;
      padding: 0 1rem 4rem;
      text-align: center;
      font-size: 0.84rem;
      color: var(--muted);
    }
    .site-footer hr {
      border: none;
      border-top: 1px solid var(--muted);
      opacity: 0.4;
      margin-bottom: 0.75rem;
    }
    .site-footer a {
      color: var(--muted);
      text-decoration: underline;
    }
  </style>
</head>
<body>
  <main>
    <div class="layout">
      <aside class="toc" aria-label="Table of contents">
        <div class="toc-inner">
          <p class="toc-title">Contents</p>
          {{ range .Specs }}
          <section class="toc-spec">
            <a class="toc-spec-title {{ .Status }}" href="#{{ .Anchor }}">{{ .Title }}</a>
            <ul class="toc-list">
              {{ range .Headings }}
              <li class="toc-item" data-anchor="{{ .Anchor }}">
                <a class="toc-link toc-level-{{ .Level }} {{ .Status }}" href="#{{ .Anchor }}">{{ .Text }}</a>
                {{ if .Children }}
                <ul class="toc-children">
                  {{ range .Children }}
                  <li class="toc-item">
                    <a class="toc-link toc-level-{{ .Level }} {{ .Status }}" href="#{{ .Anchor }}">{{ .Text }}</a>
                  </li>
                  {{ end }}
                </ul>
                {{ end }}
              </li>
              {{ end }}
            </ul>
          </section>
          {{ end }}
        </div>
      </aside>

      <div class="content">
        <div class="content-header">
          <h1 class="report-title">{{ .Title }}</h1>
          {{ .Meta }}
        </div>
        <div class="content-body">
          {{ range .Specs }}
          <article class="spec">
            <section class="spec-body">{{ .Body }}</section>
          </article>
          {{ end }}
        </div>
      </div>
    </div>
  </main>
  <footer class="site-footer">
    <hr>
    <p><a href="https://github.com/corca-ai/specdown">github.com/corca-ai/specdown</a> · written by ak@corca.ai</p>
  </footer>
<script>
(() => {
  const allLinks = Array.from(document.querySelectorAll('.toc-link[href^="#"]'));
  if (!allLinks.length) return;

  const allItems = allLinks
    .map((link) => {
      const id = decodeURIComponent(link.getAttribute('href').slice(1));
      const heading = document.getElementById(id);
      if (!heading) return null;
      return { link, heading };
    })
    .filter(Boolean);

  if (!allItems.length) return;

  // Spec sections in the ToC
  const specSections = Array.from(document.querySelectorAll('.toc-spec'));
  const specEntries = specSections
    .map((section) => {
      const titleLink = section.querySelector('.toc-spec-title');
      if (!titleLink) return null;
      const id = decodeURIComponent(titleLink.getAttribute('href').slice(1));
      const heading = document.getElementById(id);
      if (!heading) return null;
      return { section, heading };
    })
    .filter(Boolean);

  // H2 items are direct children of .toc-list with data-anchor
  const h2Items = Array.from(document.querySelectorAll('.toc-list > .toc-item[data-anchor]'));

  const h2Entries = h2Items
    .map((li) => {
      const anchor = li.getAttribute('data-anchor');
      const heading = document.getElementById(anchor);
      if (!heading) return null;
      return { li, heading };
    })
    .filter(Boolean);

  // Precompute sticky top values for shadow detection
  const stickyHeadings = Array.from(document.querySelectorAll('.spec-body :is(h2,h3,h4,h5,h6)'))
    .map(el => ({ el, top: parseFloat(getComputedStyle(el).top) || 0 }));
  let prevStuckLast = null;

  let frame = 0;

  const update = () => {
    frame = 0;

    // Shadow on the bottommost stuck heading (computed first for offset)
    let stuckLast = null;
    for (const item of stickyHeadings) {
      if (Math.abs(item.el.getBoundingClientRect().top - item.top) < 2) {
        stuckLast = item.el;
      }
    }
    if (prevStuckLast !== stuckLast) {
      prevStuckLast?.classList.remove('stuck-last');
      stuckLast?.classList.add('stuck-last');
      prevStuckLast = stuckLast;
    }

    // Use sticky header stack bottom as offset so short sections aren't overshot
    const stickyBottom = stuckLast ? stuckLast.getBoundingClientRect().bottom : 0;
    const offset = window.scrollY + Math.max(stickyBottom + 20, 50);

    // Find active link among all links (H2 + H3+)
    let active = allItems[0];
    for (const item of allItems) {
      if (item.heading.offsetTop <= offset) {
        active = item;
        continue;
      }
      break;
    }

    for (const item of allItems) {
      item.link.classList.toggle('active', item === active);
    }

    // Find active spec section
    let activeSpec = specEntries[0];
    for (const entry of specEntries) {
      if (entry.heading.offsetTop <= offset) {
        activeSpec = entry;
        continue;
      }
      break;
    }

    for (const entry of specEntries) {
      entry.section.classList.toggle('expanded', entry === activeSpec);
    }

    // Find active H2 section and expand it
    let activeH2 = h2Entries[0];
    for (const entry of h2Entries) {
      if (entry.heading.offsetTop <= offset) {
        activeH2 = entry;
        continue;
      }
      break;
    }

    for (const entry of h2Entries) {
      entry.li.classList.toggle('expanded', entry === activeH2);
    }
  };

  const schedule = () => {
    if (frame) return;
    frame = window.requestAnimationFrame(update);
  };

  window.addEventListener('scroll', schedule, { passive: true });
  window.addEventListener('resize', schedule);
  update();
})();
</script>
</body>
</html>
`))
