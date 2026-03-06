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
	Summary     core.Summary
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
		Summary:     report.Summary,
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
			html, err := markdownToHTML(current.Markdown())
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

func renderTable(node core.TableNode, caseResults map[string]core.CaseResult) (string, error) {
	if node.Fixture == "" {
		return markdownToHTML(node.Markdown())
	}

	var out strings.Builder
	out.WriteString(`<section class="exec-table-block">`)
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
	for _, row := range node.Rows {
		if row.ID == nil {
			continue
		}
		result, ok := caseResults[row.ID.Key()]
		if !ok {
			return markdownToHTML(node.Markdown())
		}
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
	renderArtifactLink(&out, "bundle artifact", result.BundlePath, outPath)
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
      --bg: #f4f1e8;
      --panel: #fffdf8;
      --ink: #1f1f1b;
      --muted: #5b594f;
      --border: #d8d0bd;
      --pass-bg: #dff5e3;
      --pass-ink: #1d5d2a;
      --fail-bg: #f9dfdb;
      --fail-ink: #7a2618;
      --accent: #a34b2a;
      --code-bg: #f7f1e3;
    }

    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: Georgia, "Times New Roman", serif;
      color: var(--ink);
      background:
        radial-gradient(circle at top left, #fff7dd 0, transparent 28rem),
        linear-gradient(180deg, #f7f3e8 0%, var(--bg) 100%);
    }

    main {
      max-width: 60rem;
      margin: 0 auto;
      padding: 3rem 1.25rem 4rem;
    }

    .hero,
    .spec {
      background: var(--panel);
      border: 1px solid var(--border);
      border-radius: 1rem;
      box-shadow: 0 0.8rem 2rem rgba(74, 56, 24, 0.08);
    }

    .hero {
      padding: 1.5rem;
      margin-bottom: 1.5rem;
    }

    .hero h1 {
      margin: 0 0 0.75rem;
      font-size: 2rem;
    }

    .meta {
      color: var(--muted);
      margin: 0;
    }

    .summary {
      display: flex;
      flex-wrap: wrap;
      gap: 0.75rem;
      margin-top: 1rem;
    }

    .failures {
      margin-top: 1.25rem;
      padding: 1rem 1rem 0.35rem;
      border-radius: 0.9rem;
      border: 1px solid #efb9ae;
      background: #fff1ee;
    }

    .failures h2 {
      margin: 0 0 0.75rem;
      font-size: 1rem;
      color: var(--fail-ink);
    }

    .failure-list {
      list-style: none;
      margin: 0;
      padding: 0;
    }

    .failure-list li {
      margin: 0 0 0.75rem;
      padding: 0 0 0.75rem;
      border-bottom: 1px solid #efc7bf;
    }

    .failure-list li:last-child {
      border-bottom: 0;
      margin-bottom: 0;
    }

    .failure-link {
      display: inline-block;
      font-weight: 700;
      text-decoration: none;
      color: var(--fail-ink);
    }

    .failure-message {
      margin: 0.25rem 0 0;
      color: var(--ink);
    }

    .pill {
      display: inline-block;
      padding: 0.45rem 0.75rem;
      border-radius: 999px;
      background: #efe8d7;
      color: var(--ink);
      border: 1px solid var(--border);
    }

    .pill.pass {
      background: var(--pass-bg);
      color: var(--pass-ink);
      border-color: #b7ddbf;
    }

    .pill.fail {
      background: var(--fail-bg);
      color: var(--fail-ink);
      border-color: #efb9ae;
    }

    .spec {
      overflow: hidden;
      margin-top: 1rem;
    }

    .spec-header {
      display: flex;
      flex-wrap: wrap;
      justify-content: space-between;
      gap: 0.75rem;
      align-items: center;
      padding: 1rem 1.25rem;
      border-bottom: 1px solid var(--border);
      background: linear-gradient(180deg, #fdf7eb 0%, #fbf7f0 100%);
    }

    .spec-header h2 {
      margin: 0;
      font-size: 1.2rem;
    }

    .spec-path,
    .exec-id {
      color: var(--muted);
      font-family: "SFMono-Regular", Menlo, monospace;
      font-size: 0.9rem;
    }

    .spec-body {
      padding: 1.25rem;
      line-height: 1.65;
    }

    .spec-body :first-child {
      margin-top: 0;
    }

    .status {
      display: inline-block;
      padding: 0.35rem 0.65rem;
      border-radius: 999px;
      font-weight: 700;
      letter-spacing: 0.02em;
      text-transform: uppercase;
      font-size: 0.82rem;
    }

    .status.passed {
      background: var(--pass-bg);
      color: var(--pass-ink);
    }

    .status.failed {
      background: var(--fail-bg);
      color: var(--fail-ink);
    }

    .exec-block {
      margin: 1rem 0;
      padding: 0.9rem 1rem 1rem;
      border: 1px solid var(--border);
      border-radius: 0.85rem;
      background: #fffaf0;
      scroll-margin-top: 1.5rem;
    }

    .exec-block.failed {
      border-color: #efb9ae;
      background: #fff3f0;
    }

    .exec-table-block {
      margin: 1rem 0;
      padding: 0.9rem 1rem 1rem;
      border: 1px solid var(--border);
      border-radius: 0.85rem;
      background: #fffaf0;
      overflow-x: auto;
    }

    .exec-table-header {
      margin-bottom: 0.75rem;
    }

    .exec-table {
      width: 100%;
      border-collapse: collapse;
      font-size: 0.98rem;
    }

    .exec-table th,
    .exec-table td {
      padding: 0.75rem 0.85rem;
      border-top: 1px solid var(--border);
      vertical-align: top;
      text-align: left;
    }

    .exec-table thead th {
      border-top: 0;
      font-size: 0.88rem;
      letter-spacing: 0.02em;
      text-transform: uppercase;
      color: var(--muted);
    }

    .exec-table tbody tr.passed {
      background: #fbfff8;
    }

    .exec-table tbody tr.failed {
      background: #fff3f0;
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
      padding: 0.75rem 0.85rem;
      border-radius: 0.75rem;
      background: #fff7f4;
      border: 1px solid #efc7bf;
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
      justify-content: space-between;
      align-items: center;
      gap: 0.75rem;
      margin-bottom: 0.75rem;
    }

    .exec-labels {
      display: flex;
      flex-direction: column;
      gap: 0.2rem;
    }

    .exec-kind {
      font-weight: 700;
      color: var(--accent);
      font-family: "SFMono-Regular", Menlo, monospace;
    }

    .exec-source {
      margin: 0;
      padding: 0.9rem 1rem;
      border-radius: 0.75rem;
      background: var(--code-bg);
      overflow-x: auto;
    }

    .exec-source.resolved {
      margin-top: 0.4rem;
      border: 1px solid var(--border);
    }

    .exec-note {
      margin: 0.75rem 0 0.35rem;
      color: var(--muted);
      font-size: 0.95rem;
    }

    .exec-message {
      margin: 0.75rem 0 0;
      color: var(--fail-ink);
      font-weight: 700;
    }

    a {
      color: var(--accent);
    }
  </style>
</head>
<body>
  <main>
    <section class="hero">
      <h1>specdown report</h1>
      <p class="meta">Adapter-hosted run. Documents are parsed into headings, prose, executable blocks, fixture tables, Alloy model fragments, and Alloy references. Adapter cases execute in document order, embedded Alloy bundles are checked alongside them, failures are summarized, and status is annotated inline.</p>
      <p class="meta">Generated at {{ .GeneratedAt }}</p>
      <div class="summary">
        <span class="pill">specs {{ .Summary.SpecsTotal }}</span>
        <span class="pill pass">spec pass {{ .Summary.SpecsPassed }}</span>
        <span class="pill fail">spec fail {{ .Summary.SpecsFailed }}</span>
        <span class="pill">cases {{ .Summary.CasesTotal }}</span>
        <span class="pill pass">case pass {{ .Summary.CasesPassed }}</span>
        <span class="pill fail">case fail {{ .Summary.CasesFailed }}</span>
        <span class="pill">alloy {{ .Summary.AlloyChecksTotal }}</span>
        <span class="pill pass">alloy pass {{ .Summary.AlloyChecksPassed }}</span>
        <span class="pill fail">alloy fail {{ .Summary.AlloyChecksFailed }}</span>
      </div>
      {{ if .Failures }}
      <section class="failures">
        <h2>Failed checks</h2>
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
        <div>
          <h2>{{ .Title }}</h2>
          <div class="spec-path">{{ .Path }}</div>
        </div>
        <span class="status {{ .Status }}">{{ .Status }}</span>
      </header>
      <section class="spec-body">{{ .Body }}</section>
    </article>
    {{ end }}
  </main>
</body>
</html>
`))
