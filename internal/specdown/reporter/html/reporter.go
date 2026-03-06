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
	Title  string
	Path   string
	Status string
	Body   template.HTML
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
		body, err := renderDocument(result, outPath)
		if err != nil {
			return fmt.Errorf("render %s: %w", result.Document.RelativeTo, err)
		}
		specs = append(specs, specView{
			Title:  result.Document.Title,
			Path:   result.Document.RelativeTo,
			Status: string(result.Status),
			Body:   template.HTML(body),
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

func renderDocument(result core.DocumentResult, outPath string) (string, error) {
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
			rendered, err := renderAlloyRef(current, alloyResults, outPath)
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

func renderAlloyRef(node core.AlloyRefNode, alloyResults map[string]core.AlloyCheckResult, outPath string) (string, error) {
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
	if result.SourceRef != "" {
		out.WriteString(`<p class="exec-note">source ref: `)
		out.WriteString(renderSourceRefLink(result.SourceRef))
		out.WriteString(`</p>`)
	}
	if result.BundleLine > 0 {
		out.WriteString(`<p class="exec-note">bundle line: <code>`)
		out.WriteString(template.HTMLEscapeString(fmt.Sprintf("%d", result.BundleLine)))
		out.WriteString(`</code></p>`)
	}
	renderArtifactLink(&out, "bundle artifact", result.BundlePath, outPath)
	renderArtifactLink(&out, "source map", result.SourceMapPath, outPath)
	renderArtifactLink(&out, "counterexample", result.CounterexamplePath, outPath)
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

func renderArtifactLink(out *strings.Builder, label string, assetPath string, reportPath string) {
	if assetPath == "" {
		return
	}
	out.WriteString(`<p class="exec-note">`)
	out.WriteString(template.HTMLEscapeString(label))
	out.WriteString(`: `)
	href := relativeAssetHref(reportPath, assetPath)
	if href != "" {
		out.WriteString(`<a href="`)
		out.WriteString(template.HTMLEscapeString(href))
		out.WriteString(`">`)
		out.WriteString(template.HTMLEscapeString(assetPath))
		out.WriteString(`</a>`)
	} else {
		out.WriteString(template.HTMLEscapeString(assetPath))
	}
	out.WriteString(`</p>`)
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

func relativeAssetHref(reportPath string, assetPath string) string {
	if reportPath == "" || assetPath == "" {
		return ""
	}
	relative, err := filepath.Rel(filepath.Dir(reportPath), assetPath)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(relative)
}

func renderSourceRefLink(sourceRef string) string {
	file, headingPath, ok := parseSourceRef(sourceRef)
	if !ok {
		return `<code>` + template.HTMLEscapeString(sourceRef) + `</code>`
	}
	anchor := core.HeadingAnchor(file, headingPath)
	var out strings.Builder
	out.WriteString(`<a href="#`)
	out.WriteString(template.HTMLEscapeString(anchor))
	out.WriteString(`"><code>`)
	out.WriteString(template.HTMLEscapeString(sourceRef))
	out.WriteString(`</code></a>`)
	return out.String()
}

func parseSourceRef(sourceRef string) (string, []string, bool) {
	parts := strings.SplitN(sourceRef, "#", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", nil, false
	}
	rawPath := strings.Split(parts[1], "/")
	headingPath := make([]string, 0, len(rawPath))
	for _, part := range rawPath {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			return "", nil, false
		}
		headingPath = append(headingPath, trimmed)
	}
	return parts[0], headingPath, true
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
      --bg: #f7f2e7;
      --paper: #fffdf8;
      --ink: #1f1f1b;
      --muted: #5b594f;
      --rule: #d8d0bd;
      --pass-ink: #1f5e2e;
      --fail-ink: #7a2618;
      --accent: #355c7a;
      --code-bg: #f4ecdc;
      --note-bg: #fbf7ef;
      --fail-bg: #fff5f2;
    }

    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: Iowan Old Style, Palatino Linotype, Book Antiqua, Georgia, serif;
      color: var(--ink);
      background: var(--bg);
    }

    main {
      max-width: 48rem;
      margin: 0 auto;
      padding: 2.75rem 1.5rem 4rem;
    }

    .hero {
      margin-bottom: 2.25rem;
      padding-bottom: 1rem;
      border-bottom: 1px solid var(--rule);
    }

    .hero h1 {
      margin: 0 0 0.35rem;
      font-size: 0.95rem;
      font-weight: 600;
      letter-spacing: 0.08em;
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
      border-left: 2px solid #e2b6ac;
    }

    .failures h2 {
      margin: 0 0 0.75rem;
      font-size: 1rem;
      font-weight: 600;
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
      padding: 2rem 0 0 0.9rem;
      border-top: 1px solid var(--rule);
      border-left: 2px solid var(--rule);
    }

    .spec.passed {
      border-left-color: #9ab9a0;
    }

    .spec.failed {
      border-left-color: #d9a597;
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
      font-size: 1.04rem;
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
      border-left: 3px solid var(--rule);
      background: transparent;
      scroll-margin-top: 1.5rem;
    }

    .exec-block.passed {
      border-left-color: #94b79a;
      background: linear-gradient(90deg, #f4faf5 0%, transparent 18rem);
    }

    .exec-block.failed {
      border-left-color: #d9a597;
      background: linear-gradient(90deg, var(--fail-bg) 0%, transparent 20rem);
    }

    .exec-table-block {
      margin: 1.35rem 0;
      padding: 0.15rem 0 0.2rem 1rem;
      border-left: 3px solid var(--rule);
      background: transparent;
      overflow-x: auto;
    }

    .exec-table-block.passed {
      border-left-color: #94b79a;
    }

    .exec-table-block.failed {
      border-left-color: #d9a597;
    }

    .exec-table-header {
      margin-bottom: 0.55rem;
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
      background: var(--fail-bg);
    }

    .exec-table tbody td:first-child {
      box-shadow: inset 3px 0 0 transparent;
    }

    .exec-table tbody tr.passed td:first-child {
      box-shadow: inset 3px 0 0 #94b79a;
    }

    .exec-table tbody tr.failed td:first-child {
      box-shadow: inset 3px 0 0 #d9a597;
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
      border-left: 2px solid #e2b6ac;
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
    }

    .exec-labels {
      display: flex;
      flex-wrap: wrap;
      gap: 0.25rem 0.8rem;
    }

    .exec-kind {
      font-weight: 600;
      color: var(--accent);
      font-family: "SFMono-Regular", Menlo, monospace;
      font-size: 0.92rem;
    }

    .exec-source {
      margin: 0;
      padding: 0.8rem 0.9rem;
      border-radius: 0.2rem;
      background: var(--code-bg);
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
  </style>
</head>
<body>
  <main>
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
    <article class="spec {{ .Status }}">
      <header class="spec-header">
        <p class="spec-meta"><span class="spec-path">{{ .Path }}</span> · <span class="status {{ .Status }}">{{ .Status }}</span></p>
      </header>
      <section class="spec-body">{{ .Body }}</section>
    </article>
    {{ end }}
  </main>
</body>
</html>
`))
