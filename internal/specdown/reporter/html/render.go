package html

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"regexp"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

// rewriteMarkdownLinks rewrites href attributes pointing to .md or .spec.md files
// to point to .html files instead, preserving fragments.
var hrefMDPattern = regexp.MustCompile(`href="([^"]*\.(?:spec\.md|md))(#[^"]*)?"`)

func rewriteMarkdownLinks(html string) string {
	return hrefMDPattern.ReplaceAllStringFunc(html, func(match string) string {
		parts := hrefMDPattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		linkPath := parts[1]
		fragment := ""
		if len(parts) > 2 {
			fragment = parts[2]
		}
		// Rewrite extension.
		if strings.HasSuffix(linkPath, ".spec.md") {
			linkPath = strings.TrimSuffix(linkPath, ".spec.md") + ".html"
		} else if strings.HasSuffix(linkPath, ".md") {
			linkPath = strings.TrimSuffix(linkPath, ".md") + ".html"
		}
		return `href="` + linkPath + fragment + `"`
	})
}

func renderDocument(result core.DocumentResult) (string, error) {
	caseResults := make(map[string]core.CaseResult, len(result.Cases))
	for i := range result.Cases {
		caseResults[result.Cases[i].ID.Key()] = result.Cases[i]
	}

	var out htmlBuilder
	var sectionStack []int
	var accBindings []core.Binding
	for _, node := range result.Document.Nodes {
		sectionStack = closeSections(&out, sectionStack, node, result.Document.RelativeTo)
		var rendered string
		var err error
		if prose, ok := node.(core.ProseNode); ok {
			rendered, err = renderProseNode(prose, caseResults, accBindings)
		} else {
			rendered, err = renderNode(node, caseResults)
		}
		if err != nil {
			return "", err
		}
		out.raw(rendered)
		accBindings = accumulateNodeBindings(accBindings, node, caseResults)
	}
	for range sectionStack {
		out.raw(`</section>`)
	}
	return out.String(), nil
}

func closeSections(out *htmlBuilder, sectionStack []int, node core.Node, documentPath string) []int {
	heading, ok := node.(core.HeadingNode)
	if !ok {
		return sectionStack
	}
	for len(sectionStack) > 0 && sectionStack[len(sectionStack)-1] >= heading.Level {
		out.raw(`</section>`)
		sectionStack = sectionStack[:len(sectionStack)-1]
	}
	if len(heading.HeadingPath) > 0 {
		fmt.Fprintf(&out.Builder, `<section class="s%d" id=%q>`, heading.Level, template.HTMLEscapeString(core.HeadingAnchor(documentPath, heading.HeadingPath)))
	} else {
		fmt.Fprintf(&out.Builder, `<section class="s%d">`, heading.Level)
	}
	return append(sectionStack, heading.Level)
}

func renderNode(node core.Node, caseResults map[string]core.CaseResult) (string, error) {
	switch current := node.(type) {
	case core.HeadingNode:
		return renderHeading(current)
	case core.ProseNode:
		return markdownToHTML(current.Markdown())
	case core.CodeBlockNode:
		return renderCodeBlock(current, caseResults)
	case core.AlloyModelNode:
		return renderAlloyModel(current, caseResults), nil
	case core.AlloyRefNode:
		return renderAlloyRef(current, caseResults)
	case core.TableNode:
		return renderTable(current, caseResults)
	case core.HookNode:
		return renderHookBlock(current), nil
	case core.CheckCallNode:
		return renderCheckCall(current, caseResults), nil
	case core.CheckDirectiveNode:
		// Check directives are consumed during compilation; nothing to render.
		return "", nil
	default:
		return "", fmt.Errorf("unknown node type %T", node)
	}
}

func renderCodeBlock(node core.CodeBlockNode, caseResults map[string]core.CaseResult) (string, error) {
	if node.ID == nil {
		if fields := strings.Fields(node.Block.Raw); len(fields) > 0 && fields[0] == "mermaid" {
			return renderMermaidBlock(node), nil
		}
		return markdownToHTML(node.Markdown())
	}

	result, ok := caseResults[node.ID.Key()]
	if !ok {
		return markdownToHTML(node.Markdown())
	}

	var out htmlBuilder
	out.raw(`<section class="exec-block `)
	if result.ExpectFail {
		out.raw("expected-fail")
	} else {
		out.text(string(result.Status))
	}
	if node.Summary != "" {
		out.raw(` has-summary`)
	}
	out.raw(`" id="`)
	out.text(node.ID.Anchor())
	out.raw(`">`)

	if node.Summary != "" {
		// Collapsible block: summary line shown, code is hidden by default.
		// Failed blocks auto-expand so failures are never hidden.
		out.raw(`<details class="exec-detail"`)
		if result.Status == core.StatusFailed && !result.ExpectFail {
			out.raw(` open`)
		}
		out.raw(`>`)
		out.raw(`<summary class="exec-source">`)
		out.raw(`<span class="exec-summary-text">`)
		out.text(node.Summary)
		out.raw(`</span>`)
		out.raw(`<span class="exec-expand-marker"></span>`)
		out.raw(`</summary>`)
		out.raw(`<div class="exec-source exec-source-body">`)
		renderCodeSourceStripped(&out, result)
		renderVisibleBindings(&out, result.VisibleBindings)
		renderBindings(&out, result.Bindings)
		out.raw(`</div>`)
		out.raw(`</details>`)
	} else {
		out.raw(`<div class="exec-source">`)
		if result.Code != nil && len(result.Code.Steps) > 0 {
			renderDoctestSteps(&out, result.Code.Steps)
		} else {
			renderCodeSource(&out, result)
		}
		renderVisibleBindings(&out, result.VisibleBindings)
		renderBindings(&out, result.Bindings)
		out.raw(`</div>`)
	}

	out.raw(`<p class="exec-block-footer">`)
	block := ""
	if result.Code != nil {
		block = result.Code.Block
	}
	out.text(block)
	out.raw(`</p>`)
	out.raw(`</section>`)
	return out.String(), nil
}

func renderMermaidBlock(node core.CodeBlockNode) string {
	var out strings.Builder
	out.WriteString(`<div class="mermaid-diagram">`)
	out.WriteString(`<pre class="mermaid">`)
	out.WriteString(template.HTMLEscapeString(node.Source))
	out.WriteString(`</pre>`)
	out.WriteString(`</div>`)
	return out.String()
}

func renderCodeSource(out *strings.Builder, result core.CaseResult) {
	source := result.Template
	if result.RenderedSource != "" {
		source = result.RenderedSource
	}
	out.raw(`<code>`)
	out.text(source)
	out.raw(`</code>`)
	if result.Status == core.StatusFailed && (result.Message != "" || result.Expected != "" || result.Actual != "") {
		renderFailureDiff(out, result.Message, result.Expected, result.Actual)
	}
}

// renderCodeSourceStripped renders the code block with the first comment line
// (the summary line) stripped from the displayed source.
func renderCodeSourceStripped(out *htmlBuilder, result core.CaseResult) {
	source := ""
	if result.Code != nil {
		source = result.Code.Template
		if result.Code.RenderedSource != "" {
			source = result.Code.RenderedSource
		}
	}
	source = stripFirstCommentLine(source)
	if source != "" {
		out.raw(`<code>`)
		out.text(source)
		out.raw(`</code>`)
	}
	if result.Status == core.StatusFailed && (result.Message != "" || result.Expected != "" || result.Actual != "") {
		renderFailureDiff(out, result.Message, result.Expected, result.Actual)
	}
}

// stripFirstCommentLine removes the first line if it is a comment.
func stripFirstCommentLine(source string) string {
	idx := strings.IndexByte(source, '\n')
	if idx < 0 {
		return ""
	}
	return source[idx+1:]
}

func renderVisibleBindings(out *htmlBuilder, bindings []core.Binding) {
	if len(bindings) == 0 {
		return
	}
	out.open("div", "exec-bindings visible-bindings")
	for i, b := range bindings {
		if i > 0 {
			out.raw(`, `)
		}
		out.raw(`$`)
		out.text(b.Name)
		out.raw(`=`)
		out.text(bindingValueToString(b.Value))
	}
	out.close("div")
}

func renderBindings(out *htmlBuilder, bindings []core.Binding) {
	if len(bindings) == 0 {
		return
	}
	out.open("div", "exec-bindings")
	for i, b := range bindings {
		if i > 0 {
			out.raw(`, `)
		}
		out.text(b.Name)
		out.raw(`=`)
		out.text(bindingValueToString(b.Value))
	}
	out.close("div")
}

func renderHeading(node core.HeadingNode) (string, error) {
	return markdownToHTML(node.Markdown())
}

type renderedRow struct {
	node   core.TableRowNode
	result core.CaseResult
}

func tableStatusClass(rows []renderedRow) string {
	status := ""
	for i := range rows {
		item := rows[i]
		switch {
		case item.result.Status == core.StatusFailed && !item.result.ExpectFail:
			return string(core.StatusFailed)
		case item.result.ExpectFail && status == "":
			status = "expected-fail"
		case status == "":
			status = string(core.StatusPassed)
		}
	}
	return status
}

func renderTable(node core.TableNode, caseResults map[string]core.CaseResult) (string, error) {
	if node.Check == "" {
		return markdownToHTML(node.Markdown())
	}

	rows := make([]renderedRow, 0, len(node.Rows))
	for _, row := range node.Rows {
		if row.ID == nil {
			continue
		}
		result, ok := caseResults[row.ID.Key()]
		if !ok {
			return markdownToHTML(node.Markdown())
		}
		rows = append(rows, renderedRow{node: row, result: result})
	}
	tableStatus := tableStatusClass(rows)

	var out htmlBuilder
	class := "exec-table-block"
	if tableStatus != "" {
		class += " " + tableStatus
	}
	out.open("section", class)
	out.raw(`<table class="exec-table">`)
	out.raw(`<thead><tr>`)
	for _, column := range node.Columns {
		out.raw(`<th>`)
		out.text(column)
		out.raw(`</th>`)
	}
	out.raw(`</tr></thead>`)
	out.raw(`<tbody>`)
	for i := range rows {
		renderTableRow(&out, rows[i].node, rows[i].result)
	}
	out.raw(`</tbody></table>`)
	out.raw(`<p class="exec-table-footer">check:`)
	out.text(node.Check)
	out.raw(`</p>`)
	out.close("section")
	return out.String(), nil
}

func renderTableRow(out *htmlBuilder, row core.TableRowNode, result core.CaseResult) {
	out.raw(`<tr class="`)
	if result.ExpectFail {
		out.raw("expected-fail")
	} else {
		out.text(string(result.Status))
	}
	out.raw(`" id="`)
	out.text(row.ID.Anchor())
	out.raw(`">`)
	var cells []string
	if result.Table != nil {
		cells = result.Table.TemplateCells
		if len(result.Table.RenderedCells) == len(cells) {
			cells = result.Table.RenderedCells
		}
	}
	lastIndex := len(cells) - 1
	for index, cell := range cells {
		out.raw(`<td>`)
		out.open("div", "cell-template")
		out.text(core.UnescapeCell(cell))
		out.close("div")
		if result.Status == core.StatusFailed && index == lastIndex {
			renderFailureDiff(out, result.Message, result.Expected, result.Actual)
		}
		out.raw(`</td>`)
	}
	out.raw(`</tr>`)
}

func renderFailureDiff(out *htmlBuilder, message, expected, actual string) {
	if message == "" && expected == "" && actual == "" {
		return
	}
	if expected == "" && actual == "" {
		out.open("div", "cell-actual")
		out.text(message)
		out.close("div")
		return
	}
	out.raw(`<dl class="failure-diff compact">`)
	if message != "" {
		out.raw(`<dt>error</dt><dd>`)
		out.text(message)
		out.raw(`</dd>`)
	}
	if expected != "" {
		out.raw(`<dt>expected</dt><dd>`)
		out.text(expected)
		out.raw(`</dd>`)
	}
	if actual != "" {
		out.raw(`<dt>actual</dt><dd>`)
		out.text(actual)
		out.raw(`</dd>`)
	}
	out.raw(`</dl>`)
}

func renderAlloyModel(node core.AlloyModelNode, caseResults map[string]core.CaseResult) string {
	// Find checks targeting this model
	var failedResult *core.CaseResult
	hasCheck := false
	for k := range caseResults {
		r := caseResults[k]
		if r.Kind != core.CaseKindAlloy || r.Alloy == nil || r.Alloy.Model != node.Model {
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

	var out htmlBuilder
	out.open("section", "exec-block alloy-model"+statusClass)
	out.open("div", "exec-source")
	out.raw(`<pre><code>`)
	out.text(node.Source)
	out.raw(`</code></pre>`)
	if failedResult != nil {
		msg := failedResult.Message
		if msg != "" {
			out.open("div", "cell-actual")
			out.text(msg)
			out.close("div")
		}
	}
	out.close("div")
	out.raw(`<p class="exec-block-footer">`)
	out.text("alloy:model(" + node.Model + ")")
	out.raw(`</p>`)
	out.close("section")
	return out.String()
}

func renderHookBlock(node core.HookNode) string {
	label := string(node.Hook)
	if node.Each {
		label += ":each"
	}
	var out htmlBuilder
	class := "exec-block hook-block"
	if node.Summary != "" {
		class += " has-summary"
	}
	out.open("section", class)

	if node.Summary != "" {
		out.raw(`<details class="exec-detail">`)
		out.raw(`<summary class="exec-source">`)
		out.raw(`<span class="exec-summary-text">`)
		out.text(node.Summary)
		out.raw(`</span>`)
		out.raw(`<span class="exec-expand-marker"></span>`)
		out.raw(`</summary>`)
		out.raw(`<div class="exec-source exec-source-body">`)
		out.raw(`<code>`)
		out.text(stripFirstCommentLine(node.Source))
		out.raw(`</code>`)
		out.raw(`</div>`)
		out.raw(`</details>`)
	} else {
		out.open("div", "exec-source")
		out.raw(`<code>`)
		out.text(node.Source)
		out.raw(`</code>`)
		out.close("div")
	}

	out.raw(`<p class="exec-block-footer">`)
	out.text(label + " · " + node.Block.Descriptor())
	out.raw(`</p>`)
	out.close("section")
	return out.String()
}

func renderCheckCall(node core.CheckCallNode, caseResults map[string]core.CaseResult) string {
	if node.ID == nil {
		return ""
	}
	result, ok := caseResults[node.ID.Key()]
	if !ok {
		return ""
	}

	var out htmlBuilder
	out.raw(`<section class="exec-block check-call `)
	out.text(string(result.Status))
	out.raw(`" id="`)
	out.text(node.ID.Anchor())
	out.raw(`">`)
	out.open("div", "exec-source")
	out.raw(`<code>`)
	label := node.Check
	if len(node.CheckParams) > 0 {
		var params []string
		for k, v := range node.CheckParams {
			params = append(params, k+"="+v)
		}
		sort.Strings(params)
		label += "(" + strings.Join(params, ", ") + ")"
	}
	out.text(label)
	out.raw(`</code>`)
	if result.Status == core.StatusFailed && (result.Message != "" || result.Expected != "" || result.Actual != "") {
		renderFailureDiff(&out, result.Message, result.Expected, result.Actual)
	}
	out.close("div")
	out.raw(`<p class="exec-block-footer">check</p>`)
	out.close("section")
	return out.String()
}

func renderAlloyRef(node core.AlloyRefNode, caseResults map[string]core.CaseResult) (string, error) {
	// Alloy failures are now shown inline in the model block.
	return "", nil
}

func mergeBindings(bindings, newBindings []core.Binding) []core.Binding {
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
var htmlCodeCheckPattern = regexp.MustCompile(`<code>check:([A-Za-z0-9_-]+)\(([^)]*)\)</code>`)
var proseVarPattern = regexp.MustCompile(`\$\{(` + core.VariableRefExpr + `)\}`)
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

	// Replace <code>check:name(params)</code> with inline check result spans
	checkIdx := 0
	checks := filterInlinesByKind(node.Inlines, core.InlineCheck)
	html = htmlCodeCheckPattern.ReplaceAllStringFunc(html, func(match string) string {
		if checkIdx >= len(checks) {
			return match
		}
		inline := checks[checkIdx]
		checkIdx++
		if inline.ID == nil {
			return match
		}
		cr, ok := caseResults[inline.ID.Key()]
		if !ok {
			return match
		}
		return renderInlineCheckSpan(inline, cr)
	})

	// Replace ${var} in non-<code> parts with variable display spans
	bindingMap := make(map[string]string, len(accBindings))
	for _, b := range accBindings {
		bindingMap[b.Name] = bindingValueToString(b.Value)
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
	var out htmlBuilder
	switch {
	case cr.Status == core.StatusPassed:
		out.raw(`<span class="inline-expect passed" title="`)
		out.text("expected " + cr.Expected)
		out.raw(`">`)
		out.text(cr.Actual)
		out.raw(`</span>`)
	case cr.ExpectFail:
		out.raw(`<span class="inline-expect expected-fail" title="`)
		out.text("expected failure: " + cr.Message)
		out.raw(`">`)
		out.text(cr.Actual)
		out.raw(`<span class="annotation">`)
		out.text(cr.Expected)
		out.raw(`</span></span>`)
	default:
		out.raw(`<span class="inline-expect failed" title="`)
		out.text(cr.Message)
		out.raw(`">`)
		out.text(cr.Actual)
		out.raw(`<span class="annotation">`)
		out.text(cr.Expected)
		out.raw(`</span></span>`)
	}
	return out.String()
}

func renderInlineCheckSpan(inline core.InlineElement, cr core.CaseResult) string {
	var out htmlBuilder
	out.raw(`<span class="inline-check `)
	out.text(string(cr.Status))
	out.raw(`" title="`)
	if cr.Status == core.StatusFailed && cr.Message != "" {
		out.text(cr.Message)
	} else {
		out.text(inline.Raw)
	}
	out.raw(`">`)
	if cr.Actual != "" {
		out.text(cr.Actual)
		out.raw(`<span class="annotation">`)
		out.text(inline.Check)
		out.raw(`</span></span>`)
	} else {
		out.text(inline.Check)
		out.raw(`</span>`)
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

func bindingValueToString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case nil:
		return ""
	default:
		data, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(data)
	}
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

func renderDoctestSteps(out *htmlBuilder, steps []core.DoctestStep) {
	out.open("div", "doctest-steps")
	for _, step := range steps {
		out.open("div", "doctest-command")
		out.raw(`<span class="doctest-prompt">$ </span>`)
		out.text(step.Command)
		out.close("div")
		if step.Status == core.StatusPassed {
			if step.Actual != "" {
				renderDoctestPassedOutput(out, step)
			}
		} else {
			if step.Actual != "" {
				out.open("div", "doctest-output failed")
				out.text(step.Actual)
				out.close("div")
			}
			if step.Expected != "" {
				out.open("div", "doctest-expected")
				out.text(step.Expected)
				out.raw(` <span class="doctest-expected-label">(expected)</span>`)
				out.close("div")
			}
		}
	}
	out.close("div")
}

// renderDoctestPassedOutput renders actual output for a passed step.
// When the expected output contains "..." wildcards, the matched lines
// are collapsed into an expandable "... (N lines)" summary.
func renderDoctestPassedOutput(out *htmlBuilder, step core.DoctestStep) {
	segments := annotateWildcard(step.Expected, step.Actual)
	if segments == nil {
		// No wildcards — render as before.
		out.open("div", "doctest-output passed")
		out.text(step.Actual)
		out.close("div")
		return
	}
	out.open("div", "doctest-output passed")
	for _, seg := range segments {
		if !seg.Wildcard {
			out.text(seg.Text)
		} else {
			unit := "lines"
			if seg.Lines == 1 {
				unit = "line"
			}
			summary := fmt.Sprintf("... (%d %s)", seg.Lines, unit)
			out.raw(`<details class="wildcard-fold"><summary>`)
			out.text(summary)
			out.raw(`</summary><span class="wildcard-expanded">`)
			out.text(seg.Text)
			out.raw(`</span></details>`)
		}
	}
	out.close("div")
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
