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
		body, err := renderDocument(result)
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

func renderDocument(result core.DocumentResult) (string, error) {
	caseResults := make(map[string]core.CaseResult, len(result.Cases))
	for _, item := range result.Cases {
		caseResults[item.ID.Key()] = item
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
		return "", fmt.Errorf("missing case result for %s", node.ID.Key())
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
	out.WriteString(template.HTMLEscapeString(node.Block.String()))
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
	out.WriteString(template.HTMLEscapeString(node.Source))
	out.WriteString(`</code></pre>`)
	if result.Message != "" {
		out.WriteString(`<p class="exec-message">`)
		out.WriteString(template.HTMLEscapeString(result.Message))
		out.WriteString(`</p>`)
	}
	out.WriteString(`</section>`)
	return out.String(), nil
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
      <p class="meta">Phase 3 run. Documents are parsed into headings, prose, and fenced code blocks. Supported executable and verification blocks run against shared document state, failures are summarized, and block status is annotated inline.</p>
      <p class="meta">Generated at {{ .GeneratedAt }}</p>
      <div class="summary">
        <span class="pill">specs {{ .Summary.SpecsTotal }}</span>
        <span class="pill pass">spec pass {{ .Summary.SpecsPassed }}</span>
        <span class="pill fail">spec fail {{ .Summary.SpecsFailed }}</span>
        <span class="pill">cases {{ .Summary.CasesTotal }}</span>
        <span class="pill pass">case pass {{ .Summary.CasesPassed }}</span>
        <span class="pill fail">case fail {{ .Summary.CasesFailed }}</span>
      </div>
      {{ if .Failures }}
      <section class="failures">
        <h2>Failed cases</h2>
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
