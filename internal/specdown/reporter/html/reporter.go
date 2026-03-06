package html

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuin/goldmark"

	"specdown/internal/specdown/core"
)

type reportView struct {
	GeneratedAt string
	PassedCount int
	FailedCount int
	Failures    []failureView
	Specs       []specView
}

type specView struct {
	Title    string
	Path     string
	Headings []tocItemView
	Body     template.HTML
}

type tocItemView struct {
	Text   string
	Anchor string
	Level  int
	Status string
}

type failureView struct {
	DocumentTitle string
	Label         string
	Message       string
	Anchor        string
}

func Write(report core.Report, outPath string) error {
	specs := make([]specView, 0, len(report.Results))
	failures := make([]failureView, 0)
	for _, result := range report.Results {
		body, err := renderDocument(result)
		if err != nil {
			return fmt.Errorf("render %s: %w", result.Document.RelativeTo, err)
		}
		specs = append(specs, specView{
			Title:    result.Document.Title,
			Path:     result.Document.RelativeTo,
			Headings: collectHeadings(result),
			Body:     template.HTML(body),
		})

		for _, item := range result.Cases {
			if item.Status != core.StatusFailed {
				continue
			}
			failures = append(failures, failureView{
				DocumentTitle: result.Document.Title,
				Label:         item.Label,
				Message:       item.Message,
				Anchor:        item.ID.Anchor(),
			})
		}
		for _, item := range result.AlloyChecks {
			if item.Status != core.StatusFailed {
				continue
			}
			failures = append(failures, failureView{
				DocumentTitle: result.Document.Title,
				Label:         item.Label,
				Message:       item.Message,
				Anchor:        item.ID.Anchor(),
			})
		}
	}

	view := reportView{
		GeneratedAt: report.GeneratedAt.Format(time.RFC3339),
		PassedCount: report.Summary.CasesPassed + report.Summary.AlloyChecksPassed,
		FailedCount: report.Summary.CasesFailed + report.Summary.AlloyChecksFailed,
		Failures:    failures,
		Specs:       specs,
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("create report dir: %w", err)
	}

	file, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create report: %w", err)
	}
	defer file.Close()

	if err := pageTemplate.Execute(file, view); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

func collectHeadings(result core.DocumentResult) []tocItemView {
	statuses := collectHeadingStatuses(result)
	items := make([]tocItemView, 0)
	for _, node := range result.Document.Nodes {
		heading, ok := node.(core.HeadingNode)
		if !ok {
			continue
		}
		if len(heading.HeadingPath) == 0 {
			continue
		}
		items = append(items, tocItemView{
			Text:   heading.Text,
			Anchor: core.HeadingAnchor(result.Document.RelativeTo, heading.HeadingPath),
			Level:  heading.Level,
			Status: tocStatusClass(statuses[headingPathKey(heading.HeadingPath)]),
		})
	}
	return items
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
		key := headingPathKey(path)
		current := statuses[key]
		if current == core.StatusFailed {
			return
		}
		if status == core.StatusFailed || current == "" {
			statuses[key] = status
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
	for _, node := range result.Document.Nodes {
		switch current := node.(type) {
		case core.HeadingNode:
			html, err := renderHeading(current, result.Document.RelativeTo)
			if err != nil {
				return "", err
			}
			out.WriteString(html)
		case core.ProseNode:
			html, err := markdownToHTML(current.Markdown())
			if err != nil {
				return "", err
			}
			out.WriteString(html)
		case core.CodeBlockNode:
			rendered, err := renderCodeBlock(current, caseResults)
			if err != nil {
				return "", err
			}
			out.WriteString(rendered)
		case core.AlloyModelNode:
			rendered, err := renderAlloyModel(current)
			if err != nil {
				return "", err
			}
			out.WriteString(rendered)
		case core.AlloyRefNode:
			rendered, err := renderAlloyRef(current, alloyResults)
			if err != nil {
				return "", err
			}
			out.WriteString(rendered)
		case core.TableNode:
			rendered, err := renderTable(current, caseResults)
			if err != nil {
				return "", err
			}
			out.WriteString(rendered)
		default:
			return "", fmt.Errorf("unknown node type %T", node)
		}
	}
	return out.String(), nil
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
	out.WriteString(`<div class="exec-header">`)
	out.WriteString(`<div class="exec-labels">`)
	out.WriteString(`<span class="exec-kind">`)
	out.WriteString(template.HTMLEscapeString(result.Block))
	out.WriteString(`</span>`)
	out.WriteString(`<span class="exec-id">`)
	out.WriteString(template.HTMLEscapeString(formatSpecID(*node.ID)))
	out.WriteString(`</span>`)
	out.WriteString(`</div>`)
	out.WriteString(`<span class="status `)
	out.WriteString(template.HTMLEscapeString(string(result.Status)))
	out.WriteString(`">`)
	out.WriteString(template.HTMLEscapeString(string(result.Status)))
	out.WriteString(`</span>`)
	out.WriteString(`</div>`)
	out.WriteString(`<pre class="exec-source"><code>`)
	out.WriteString(template.HTMLEscapeString(result.Template))
	out.WriteString(`</code></pre>`)
	if result.RenderedSource != "" && result.RenderedSource != result.Template {
		out.WriteString(`<p class="exec-note">resolved input</p>`)
		out.WriteString(`<pre class="exec-source resolved"><code>`)
		out.WriteString(template.HTMLEscapeString(result.RenderedSource))
		out.WriteString(`</code></pre>`)
	}
	if len(result.Bindings) > 0 {
		out.WriteString(`<p class="exec-note">captured bindings: `)
		for i, binding := range result.Bindings {
			if i > 0 {
				out.WriteString(`, `)
			}
			out.WriteString(template.HTMLEscapeString(binding.Name))
			out.WriteString(`=`)
			out.WriteString(template.HTMLEscapeString(binding.Value))
		}
		out.WriteString(`</p>`)
	}
	if result.Message != "" {
		out.WriteString(`<p class="exec-message">`)
		out.WriteString(template.HTMLEscapeString(result.Message))
		out.WriteString(`</p>`)
	}
	if result.Expected != "" || result.Actual != "" {
		out.WriteString(renderFailureDiff(result.Expected, result.Actual, false))
	}
	out.WriteString(`</section>`)
	return out.String(), nil
}

func renderHeading(node core.HeadingNode, documentPath string) (string, error) {
	html, err := markdownToHTML(node.Markdown())
	if err != nil {
		return "", err
	}
	if len(node.HeadingPath) == 0 {
		return html, nil
	}
	openTag := fmt.Sprintf("<h%d>", node.Level)
	replacement := fmt.Sprintf("<h%d id=\"%s\">", node.Level, template.HTMLEscapeString(core.HeadingAnchor(documentPath, node.HeadingPath)))
	return strings.Replace(html, openTag, replacement, 1), nil
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
	out.WriteString(`<div class="exec-table-header">`)
	out.WriteString(`<span class="exec-kind">fixture:`)
	out.WriteString(template.HTMLEscapeString(node.Fixture))
	out.WriteString(`</span>`)
	out.WriteString(`</div>`)
	out.WriteString(`<table class="exec-table">`)
	out.WriteString(`<thead><tr>`)
	for _, column := range node.Columns {
		out.WriteString(`<th>`)
		out.WriteString(template.HTMLEscapeString(column))
		out.WriteString(`</th>`)
	}
	out.WriteString(`<th>Status</th></tr></thead>`)
	out.WriteString(`<tbody>`)
	for _, item := range rows {
		row := item.node
		result := item.result
		out.WriteString(`<tr class="`)
		out.WriteString(template.HTMLEscapeString(string(result.Status)))
		out.WriteString(`" id="`)
		out.WriteString(template.HTMLEscapeString(row.ID.Anchor()))
		out.WriteString(`">`)
		for index, cell := range result.TemplateCells {
			out.WriteString(`<td>`)
			out.WriteString(`<div class="cell-template">`)
			out.WriteString(template.HTMLEscapeString(cell))
			out.WriteString(`</div>`)
			if index < len(result.RenderedCells) && result.RenderedCells[index] != cell {
				out.WriteString(`<div class="cell-resolved">`)
				out.WriteString(template.HTMLEscapeString(result.RenderedCells[index]))
				out.WriteString(`</div>`)
			}
			out.WriteString(`</td>`)
		}
		out.WriteString(`<td class="exec-table-status">`)
		out.WriteString(`<span class="status `)
		out.WriteString(template.HTMLEscapeString(string(result.Status)))
		out.WriteString(`">`)
		out.WriteString(template.HTMLEscapeString(string(result.Status)))
		out.WriteString(`</span>`)
		if result.Message != "" {
			out.WriteString(`<div class="exec-table-message">`)
			out.WriteString(template.HTMLEscapeString(result.Message))
			out.WriteString(`</div>`)
		}
		if result.Expected != "" || result.Actual != "" {
			out.WriteString(renderFailureDiff(result.Expected, result.Actual, true))
		}
		out.WriteString(`</td></tr>`)
	}
	out.WriteString(`</tbody></table></section>`)
	return out.String(), nil
}

func renderAlloyModel(node core.AlloyModelNode) (string, error) {
	var out strings.Builder
	out.WriteString(`<section class="exec-block alloy-model">`)
	out.WriteString(`<div class="exec-header">`)
	out.WriteString(`<div class="exec-labels">`)
	out.WriteString(`<span class="exec-kind">`)
	out.WriteString(template.HTMLEscapeString("alloy:model(" + node.Model + ")"))
	out.WriteString(`</span>`)
	out.WriteString(`</div>`)
	out.WriteString(`</div>`)
	out.WriteString(`<pre class="exec-source"><code>`)
	out.WriteString(template.HTMLEscapeString(node.Source))
	out.WriteString(`</code></pre>`)
	out.WriteString(`</section>`)
	return out.String(), nil
}

func renderAlloyRef(node core.AlloyRefNode, alloyResults map[string]core.AlloyCheckResult) (string, error) {
	if node.ID == nil {
		return "", fmt.Errorf("alloy ref is missing an id")
	}

	result, ok := alloyResults[node.ID.Key()]
	if !ok {
		return "", fmt.Errorf("missing alloy result for %s", node.ID.Key())
	}

	var out strings.Builder
	out.WriteString(`<section class="exec-block alloy-ref `)
	out.WriteString(template.HTMLEscapeString(string(result.Status)))
	out.WriteString(`" id="`)
	out.WriteString(template.HTMLEscapeString(node.ID.Anchor()))
	out.WriteString(`">`)
	out.WriteString(`<div class="exec-header">`)
	out.WriteString(`<div class="exec-labels">`)
	out.WriteString(`<span class="exec-kind">`)
	out.WriteString(template.HTMLEscapeString("alloy:ref(" + node.Model + "#" + node.Assertion + ", scope=" + node.Scope + ")"))
	out.WriteString(`</span>`)
	out.WriteString(`<span class="exec-id">`)
	out.WriteString(template.HTMLEscapeString(formatSpecID(*node.ID)))
	out.WriteString(`</span>`)
	out.WriteString(`</div>`)
	out.WriteString(`<span class="status `)
	out.WriteString(template.HTMLEscapeString(string(result.Status)))
	out.WriteString(`">`)
	out.WriteString(template.HTMLEscapeString(string(result.Status)))
	out.WriteString(`</span>`)
	out.WriteString(`</div>`)
	out.WriteString(`<p class="exec-note">assertion <code>`)
	out.WriteString(template.HTMLEscapeString(result.Assertion))
	out.WriteString(`</code> checked at scope <code>`)
	out.WriteString(template.HTMLEscapeString(result.Scope))
	out.WriteString(`</code></p>`)
	if result.Message != "" {
		out.WriteString(`<p class="exec-message">`)
		out.WriteString(template.HTMLEscapeString(result.Message))
		out.WriteString(`</p>`)
	}
	if result.Expected != "" || result.Actual != "" {
		out.WriteString(renderFailureDiff(result.Expected, result.Actual, false))
	}
	out.WriteString(`</section>`)
	return out.String(), nil
}

func renderFailureDiff(expected string, actual string, compact bool) string {
	var out strings.Builder
	out.WriteString(`<dl class="failure-diff`)
	if compact {
		out.WriteString(` compact`)
	}
	out.WriteString(`">`)
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
	return out.String()
}

func markdownToHTML(source string) (string, error) {
	var out bytes.Buffer
	if err := goldmark.Convert([]byte(source), &out); err != nil {
		return "", err
	}
	return out.String(), nil
}

func formatSpecID(id core.SpecID) string {
	if len(id.HeadingPath) == 0 {
		return id.File + "#" + fmt.Sprintf("%d", id.Ordinal)
	}
	return id.File + "#" + strings.Join(id.HeadingPath, " / ") + " / " + fmt.Sprintf("%d", id.Ordinal)
}

var pageTemplate = template.Must(template.New("report").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>specdown report</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f3f3f0;
      --paper: #fcfcfa;
      --ink: #1f1f1b;
      --muted: #66665f;
      --rule: #d6d6cf;
      --pass-ink: #0f7a37;
      --pass-mark: #19a34a;
      --fail-ink: #a1261a;
      --fail-mark: #d63b2d;
      --accent: #4d4d46;
      --code-bg: #efefea;
      --note-bg: #f5f5f1;
    }

    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: "Avenir Next", "Helvetica Neue", "Segoe UI", sans-serif;
      color: var(--ink);
      background: var(--bg);
    }

    main {
      max-width: 78rem;
      margin: 0 auto;
      padding: 2.75rem 1.5rem 4rem;
    }

    .layout {
      display: grid;
      grid-template-columns: 16rem minmax(0, 54rem);
      gap: 2.5rem;
      align-items: start;
    }

    .toc {
      position: sticky;
      top: 1.5rem;
      align-self: start;
      font-size: 0.92rem;
      line-height: 1.5;
    }

    .toc-inner {
      padding: 0.25rem 0;
    }

    .toc-title {
      margin: 0 0 0.75rem;
      color: var(--muted);
      font-size: 0.78rem;
      font-weight: 600;
      letter-spacing: 0.08em;
      text-transform: uppercase;
    }

    .toc-spec {
      margin-bottom: 1.25rem;
    }

    .toc-spec:last-child {
      margin-bottom: 0;
    }

    .toc-spec-title {
      margin: 0 0 0.35rem;
      font-weight: 600;
      color: var(--ink);
    }

    .toc-spec-path {
      margin: 0 0 0.55rem;
      color: var(--muted);
      font-size: 0.82rem;
      font-family: "SFMono-Regular", Menlo, monospace;
    }

    .toc-list {
      list-style: none;
      margin: 0;
      padding: 0;
    }

    .toc-item {
      margin: 0.18rem 0;
    }

    .toc-link {
      display: block;
      text-decoration: none;
      color: var(--muted);
      position: relative;
      transition: color 120ms ease;
    }

    .toc-link:hover {
      color: var(--ink);
    }

    .toc-link.active {
      color: var(--ink);
      font-weight: 600;
    }

    .toc-link.failed {
      padding-left: 0.95rem;
    }

    .toc-link.failed::before {
      content: "";
      position: absolute;
      left: 0;
      top: 0.42rem;
      width: 0.42rem;
      height: 0.42rem;
      border-radius: 999px;
      background: var(--fail-mark);
    }

    .toc-level-1 {
      font-weight: 600;
      color: var(--ink);
    }

    .toc-level-2 { padding-left: 0.7rem; }
    .toc-level-3 { padding-left: 1.4rem; }
    .toc-level-4 { padding-left: 2.1rem; }
    .toc-level-5,
    .toc-level-6 { padding-left: 2.8rem; }

    .content {
      min-width: 0;
    }

    .hero {
      margin-bottom: 2.25rem;
      padding-bottom: 1rem;
      border-bottom: 1px solid var(--rule);
    }

    .hero h1 {
      margin: 0 0 0.35rem;
      font-family: Iowan Old Style, Palatino Linotype, Book Antiqua, Georgia, serif;
      font-size: 1.15rem;
      font-weight: 600;
      letter-spacing: 0.08em;
      text-wrap: balance;
      text-transform: uppercase;
    }

    .meta {
      color: var(--muted);
      margin: 0;
      line-height: 1.65;
    }

    .summary {
      margin-top: 0.2rem;
    }

    .failures {
      margin-top: 1.25rem;
      padding-left: 1rem;
      border-left: 2px solid var(--rule);
    }

    .failures h2 {
      margin: 0 0 0.75rem;
      font-family: Iowan Old Style, Palatino Linotype, Book Antiqua, Georgia, serif;
      font-size: 1.3rem;
      font-weight: 600;
      text-wrap: balance;
    }

    .failure-list {
      margin: 0;
      padding-left: 1.2rem;
    }

    .failure-list li {
      margin: 0 0 0.8rem;
    }

    .failure-link {
      font-weight: 600;
      text-decoration: none;
      color: var(--ink);
    }

    .failure-link:hover {
      text-decoration: underline;
    }

    .failure-message {
      margin: 0.2rem 0 0;
      color: var(--muted);
    }

    .pill {
      display: inline;
      padding: 0;
      border: 0;
      background: transparent;
      color: var(--muted);
    }

    .pill::before {
      content: "· ";
      color: var(--muted);
    }

    .pill.pass {
      color: var(--pass-ink);
    }

    .pill.fail {
      color: var(--fail-ink);
    }

    .spec {
      margin: 0;
      padding: 2rem 0 0 0;
      border-top: 1px solid var(--rule);
    }

    .spec-header {
      margin-bottom: 1rem;
    }

    .spec-meta {
      margin: 0;
      color: var(--muted);
      font-size: 0.95rem;
      line-height: 1.5;
    }

    .spec-path,
    .exec-id {
      color: var(--muted);
      font-family: "SFMono-Regular", Menlo, monospace;
      font-size: 0.9rem;
    }

    .spec-body {
      line-height: 1.82;
      font-size: 1rem;
    }

    .spec-body p,
    .spec-body li,
    .spec-body td,
    .spec-body th {
      font-size: 1rem;
    }

    .spec-body h1,
    .spec-body h2,
    .spec-body h3,
    .spec-body h4,
    .spec-body h5,
    .spec-body h6 {
      font-family: Iowan Old Style, Palatino Linotype, Book Antiqua, Georgia, serif;
      line-height: 1.15;
      text-wrap: balance;
      letter-spacing: -0.01em;
    }

    .spec-body h1 {
      font-size: 2.5rem;
      margin: 0 0 1.1rem;
    }

    .spec-body h2 {
      font-size: 1.85rem;
      margin: 2.9rem 0 0.95rem;
    }

    .spec-body h3 {
      font-size: 1.4rem;
      margin: 2.25rem 0 0.78rem;
    }

    .spec-body h4,
    .spec-body h5,
    .spec-body h6 {
      font-size: 1.08rem;
      margin: 1.7rem 0 0.68rem;
    }

    .spec-body :first-child {
      margin-top: 0;
    }

    .status {
      font-weight: 600;
      font-size: 0.95rem;
    }

    .status.passed {
      color: var(--pass-ink);
    }

    .status.failed {
      color: var(--fail-ink);
    }

    .exec-block {
      margin: 1.35rem 0;
      padding: 0.15rem 0 0.2rem 1rem;
      background: transparent;
      scroll-margin-top: 1.5rem;
    }

    .exec-block.passed {
      background: transparent;
    }

    .exec-block.failed {
      background: transparent;
    }

    .exec-table-block {
      margin: 1.35rem 0;
      padding: 0.15rem 0 0.2rem 1rem;
      background: transparent;
      overflow-x: auto;
    }

    .exec-table-header {
      margin-bottom: 0.55rem;
      position: relative;
      line-height: 1.2;
    }

    .exec-table {
      width: 100%;
      border-collapse: collapse;
      font-size: 0.95rem;
      background: var(--paper);
    }

    .exec-table th,
    .exec-table td {
      padding: 0.7rem 0.75rem;
      border-top: 1px solid var(--rule);
      vertical-align: top;
      text-align: left;
    }

    .exec-table thead th {
      border-top: 0;
      font-size: 0.8rem;
      letter-spacing: 0.06em;
      text-transform: uppercase;
      color: var(--muted);
    }

    .exec-table tbody tr.failed {
      background: transparent;
    }

    .exec-table tbody td:first-child {
      box-shadow: inset 3px 0 0 transparent;
    }

    .exec-table tbody tr.passed td:first-child {
      box-shadow: inset 4px 0 0 var(--pass-mark);
    }

    .exec-table tbody tr.failed td:first-child {
      box-shadow: inset 4px 0 0 var(--fail-mark);
    }

    .cell-template {
      font-family: "SFMono-Regular", Menlo, monospace;
    }

    .cell-resolved {
      margin-top: 0.35rem;
      color: var(--muted);
      font-family: "SFMono-Regular", Menlo, monospace;
      font-size: 0.92rem;
    }

    .exec-table-status {
      min-width: 13rem;
    }

    .exec-table-message {
      margin-top: 0.5rem;
      color: var(--fail-ink);
      font-weight: 700;
    }

    .failure-diff {
      margin: 0.75rem 0 0;
      padding: 0.65rem 0.8rem;
      background: var(--note-bg);
      border-left: 2px solid var(--rule);
      display: grid;
      grid-template-columns: auto 1fr;
      gap: 0.35rem 0.75rem;
    }

    .failure-diff.compact {
      margin-top: 0.75rem;
      padding: 0.65rem 0.75rem;
    }

    .failure-diff dt {
      margin: 0;
      color: var(--muted);
      font-size: 0.82rem;
      text-transform: uppercase;
      letter-spacing: 0.03em;
    }

    .failure-diff dd {
      margin: 0;
      font-family: "SFMono-Regular", Menlo, monospace;
      word-break: break-word;
    }

    .exec-header {
      display: flex;
      flex-wrap: wrap;
      align-items: baseline;
      gap: 0.6rem 1rem;
      margin-bottom: 0.55rem;
      position: relative;
      line-height: 1.2;
    }

    .exec-labels {
      display: flex;
      flex-wrap: wrap;
      gap: 0.25rem 0.8rem;
    }

    .exec-header::before,
    .exec-table-header::before {
      content: "";
      position: absolute;
      left: -1rem;
      top: 0.08em;
      width: 0.28rem;
      height: 1.16em;
      border-radius: 999px;
      background: #c9c0ab;
    }

    .exec-block.passed > .exec-header::before,
    .exec-table-block.passed > .exec-table-header::before {
      background: var(--pass-mark);
    }

    .exec-block.failed > .exec-header::before,
    .exec-table-block.failed > .exec-table-header::before {
      background: var(--fail-mark);
    }

    .exec-kind {
      font-weight: 600;
      color: var(--accent);
      font-family: "SFMono-Regular", Menlo, Consolas, monospace;
      font-size: 0.92rem;
      line-height: 1.2;
    }

    .exec-source {
      margin: 0;
      padding: 0.8rem 0.9rem;
      border-radius: 0.2rem;
      background: var(--code-bg);
      font-family: "SFMono-Regular", Menlo, Consolas, monospace;
      font-size: 0.92rem;
      line-height: 1.45;
      overflow-x: auto;
    }

    .exec-source.resolved {
      margin-top: 0.4rem;
      border: 1px solid var(--rule);
    }

    .exec-note {
      margin: 0.75rem 0 0.35rem;
      color: var(--muted);
      font-size: 0.92rem;
    }

    .exec-message {
      margin: 0.75rem 0 0;
      color: var(--fail-ink);
      font-weight: 600;
    }

    a {
      color: var(--accent);
    }

    code,
    pre,
    kbd,
    samp {
      font-family: "SFMono-Regular", Menlo, Consolas, monospace;
      font-size: 0.94em;
      line-height: 1.45;
    }

    @media (max-width: 960px) {
      .layout {
        grid-template-columns: minmax(0, 1fr);
        gap: 1.5rem;
      }

      .toc {
        position: static;
      }

      .toc-inner {
        padding-left: 0;
        border-left: 0;
        border-bottom: 1px solid var(--rule);
        padding-bottom: 1rem;
      }
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
            <p class="toc-spec-title">{{ .Title }}</p>
            <p class="toc-spec-path">{{ .Path }}</p>
            <ul class="toc-list">
              {{ range .Headings }}
              <li class="toc-item">
                <a class="toc-link toc-level-{{ .Level }} {{ .Status }}" href="#{{ .Anchor }}">{{ .Text }}</a>
              </li>
              {{ end }}
            </ul>
          </section>
          {{ end }}
        </div>
      </aside>

      <div class="content">
        <section class="hero">
          <h1>report</h1>
          <div class="summary">
            <p class="meta">Generated at {{ .GeneratedAt }}<span class="pill pass">pass {{ .PassedCount }}</span><span class="pill fail">fail {{ .FailedCount }}</span></p>
          </div>
          {{ if .Failures }}
          <section class="failures">
            <h2>Failures</h2>
            <ul class="failure-list">
              {{ range .Failures }}
              <li>
                <a class="failure-link" href="#{{ .Anchor }}">{{ .DocumentTitle }}: {{ .Label }}</a>
                <p class="failure-message">{{ .Message }}</p>
              </li>
              {{ end }}
            </ul>
          </section>
          {{ end }}
        </section>

        {{ range .Specs }}
        <article class="spec">
          <header class="spec-header">
            <p class="spec-meta"><span class="spec-path">{{ .Path }}</span></p>
          </header>
          <section class="spec-body">{{ .Body }}</section>
        </article>
        {{ end }}
      </div>
    </div>
  </main>
<script>
(() => {
  const links = Array.from(document.querySelectorAll('.toc-link[href^="#"]'));
  if (!links.length) return;

  const items = links
    .map((link) => {
      const id = decodeURIComponent(link.getAttribute('href').slice(1));
      const heading = document.getElementById(id);
      if (!heading) return null;
      return { link, heading };
    })
    .filter(Boolean);

  if (!items.length) return;

  let frame = 0;

  const update = () => {
    frame = 0;
    const offset = window.scrollY + Math.min(window.innerHeight * 0.22, 180);
    let active = items[0];

    for (const item of items) {
      if (item.heading.offsetTop <= offset) {
        active = item;
        continue;
      }
      break;
    }

    for (const item of items) {
      item.link.classList.toggle('active', item === active);
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
