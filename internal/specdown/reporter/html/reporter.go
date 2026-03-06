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
	Specs []specView
}

type specView struct {
	Title    string
	Headings []tocItemView
	Body     template.HTML
}

type tocItemView struct {
	Text   string
	Anchor string
	Level  int
	Status string
}

func Write(report core.Report, outPath string) error {
	specs := make([]specView, 0, len(report.Results))
	meta := buildMeta(report)
	for _, result := range report.Results {
		body, err := renderDocument(result, meta)
		if err != nil {
			return fmt.Errorf("render %s: %w", result.Document.RelativeTo, err)
		}
		specs = append(specs, specView{
			Title:    result.Document.Title,
			Headings: collectHeadings(result),
			Body:     template.HTML(body),
		})
		meta = "" // only inject meta after first spec's h1
	}

	view := reportView{
		Specs: specs,
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
		if len(heading.HeadingPath) == 0 || heading.Level == 1 {
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

func buildMeta(report core.Report) string {
	passed := report.Summary.CasesPassed + report.Summary.AlloyChecksPassed
	failed := report.Summary.CasesFailed + report.Summary.AlloyChecksFailed
	var b strings.Builder
	b.WriteString(`<p class="content-meta">`)
	b.WriteString(template.HTMLEscapeString(report.GeneratedAt.Format(time.RFC3339)))
	b.WriteString(`<span class="pill pass">pass `)
	b.WriteString(fmt.Sprintf("%d", passed))
	b.WriteString(`</span>`)
	b.WriteString(`<span class="pill fail">fail `)
	b.WriteString(fmt.Sprintf("%d", failed))
	b.WriteString(`</span>`)
	b.WriteString(`</p>`)
	return b.String()
}

func renderDocument(result core.DocumentResult, meta string) (string, error) {
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
			if current.Level == 1 && meta != "" {
				out.WriteString(meta)
				meta = ""
			}
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
			rendered, err := renderAlloyModel(current, alloyResults)
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
	source := result.Template
	if result.RenderedSource != "" {
		source = result.RenderedSource
	}
	out.WriteString(`<div class="exec-source">`)
	out.WriteString(`<code>`)
	out.WriteString(template.HTMLEscapeString(source))
	out.WriteString(`</code>`)
	if result.Actual != "" {
		out.WriteString(`<div class="cell-actual">`)
		out.WriteString(template.HTMLEscapeString(result.Actual))
		out.WriteString(`</div>`)
	} else if result.Message != "" {
		out.WriteString(`<div class="cell-actual">`)
		out.WriteString(template.HTMLEscapeString(result.Message))
		out.WriteString(`</div>`)
	}
	if len(result.Bindings) > 0 {
		out.WriteString(`<div class="exec-bindings">`)
		for i, b := range result.Bindings {
			if i > 0 {
				out.WriteString(`, `)
			}
			out.WriteString(template.HTMLEscapeString(b.Name))
			out.WriteString(`=`)
			out.WriteString(template.HTMLEscapeString(b.Value))
		}
		out.WriteString(`</div>`)
	}
	out.WriteString(`</div>`)
	out.WriteString(`<p class="exec-block-footer">`)
	out.WriteString(template.HTMLEscapeString(result.Block))
	out.WriteString(`</p>`)
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
		row := item.node
		result := item.result
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
			out.WriteString(template.HTMLEscapeString(cell))
			out.WriteString(`</div>`)
			if result.Status == core.StatusFailed && index == lastIndex && result.Actual != "" {
				out.WriteString(`<div class="cell-actual">`)
				out.WriteString(template.HTMLEscapeString(result.Actual))
				out.WriteString(`</div>`)
			}
			out.WriteString(`</td>`)
		}
		out.WriteString(`</tr>`)
	}
	out.WriteString(`</tbody></table>`)
	out.WriteString(`<p class="exec-table-footer">fixture:`)
	out.WriteString(template.HTMLEscapeString(node.Fixture))
	out.WriteString(`</p>`)
	out.WriteString(`</section>`)
	return out.String(), nil
}

func renderAlloyModel(node core.AlloyModelNode, alloyResults map[string]core.AlloyCheckResult) (string, error) {
	// Find checks in this model block and match against alloy results
	var failedResult *core.AlloyCheckResult
	hasCheck := false
	for _, r := range alloyResults {
		if r.Model != node.Model {
			continue
		}
		checkPattern := "check " + r.Assertion
		if !strings.Contains(node.Source, checkPattern) {
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
		actual := failedResult.Actual
		if actual == "" {
			actual = failedResult.Message
		}
		if actual != "" {
			out.WriteString(`<div class="cell-actual">`)
			out.WriteString(template.HTMLEscapeString(actual))
			out.WriteString(`</div>`)
		}
	}
	out.WriteString(`</div>`)
	out.WriteString(`<p class="exec-block-footer">`)
	out.WriteString(template.HTMLEscapeString("alloy:model(" + node.Model + ")"))
	out.WriteString(`</p>`)
	out.WriteString(`</section>`)
	return out.String(), nil
}

func renderAlloyRef(node core.AlloyRefNode, alloyResults map[string]core.AlloyCheckResult) (string, error) {
	// Alloy failures are now shown inline in the model block.
	return "", nil
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
      margin-inline: auto;
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
      font-size: 0.92rem;
      line-height: 1.5;
    }

    .toc-inner { padding: 1.1rem 0 0; }

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
      &:last-child { margin-bottom: 0; }
    }

    .toc-spec-title {
      margin: 0 0 0.35rem;
      font-weight: 600;
      color: var(--ink);
    }

    .toc-list {
      list-style: none;
      margin: 0;
      padding: 0;
    }

    .toc-item {
      margin: 0.1rem 0;
      &:has(> .toc-level-1) { margin-top: 0.6rem; }
      &:has(> .toc-level-2) { margin-top: 0.4rem; }
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

      &.failed::before {
        content: "";
        position: absolute;
        left: -0.75rem;
        top: 0.42rem;
        width: 0.42rem;
        height: 0.42rem;
        border-radius: 999px;
        background: var(--fail-mark);
      }
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

    .content { min-width: 0; }

    .content-meta {
      margin: 0 0 1.5rem;
      color: var(--muted);
      font-size: 0.82rem;
      line-height: 1.65;
    }

    .pill {
      display: inline;
      padding: 0;
      border: 0;
      background: transparent;
      color: var(--muted);

      &::before { content: "· "; color: var(--muted); }
      &.pass { color: var(--pass-ink); }
      &.fail { color: var(--fail-ink); }
    }

    .spec {
      margin: 0;
    }

    .spec + .spec {
      padding-top: 2rem;
    }

    .spec-body {
      line-height: 1.82;

      & :first-child { margin-top: 0; }

      & :is(h1, h2, h3, h4, h5, h6) {
        font-family: Iowan Old Style, Palatino Linotype, Book Antiqua, Georgia, serif;
        line-height: 1.15;
        text-wrap: balance;
        letter-spacing: -0.01em;
      }

      & h1 { font-size: 2.5rem; margin: 0 0 1.1rem; }
      & h2 { font-size: 1.85rem; margin: 2.9rem 0 0.95rem; }
      & h3 { font-size: 1.4rem; margin: 2.25rem 0 0.78rem; }
      & :is(h4, h5, h6) { font-size: 1.08rem; margin: 1.7rem 0 0.68rem; }
    }

    .status {
      font-weight: 600;
      font-size: 0.95rem;
      &.passed { color: var(--pass-ink); }
      &.failed { color: var(--fail-ink); }
    }

    .exec-block,
    .exec-table-block {
      margin: 1.35rem 0;
    }

    .exec-block { scroll-margin-top: 1.5rem; }
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

    .cell-template { font-family: var(--font-mono); }

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
        margin: 0;
        color: var(--muted);
        font-size: 0.82rem;
        line-height: 1.45;
        text-transform: uppercase;
        letter-spacing: 0.03em;
      }

      & dd {
        margin: 0;
        font-family: var(--font-mono);
        line-height: 1.45;
        word-break: break-word;
      }
    }


    .exec-bindings {
      margin-top: 0.35rem;
      font-size: 0.85rem;
      font-style: italic;
      color: var(--muted);
      font-family: var(--font-mono);
    }

    :is(.exec-block-footer, .exec-table-footer) {
      margin: 0;
      text-align: right;
      font-size: 0.8rem;
      color: var(--muted);
      font-family: var(--font-mono);
    }

    .exec-kind {
      color: var(--muted);
      font-family: var(--font-mono);
      font-size: 0.8rem;
      line-height: 1.2;
    }

    .exec-source {
      margin: 0;
      padding: 0.8rem 0.9rem;
      border: 1px solid var(--rule);
      border-radius: 0.2rem;
      background: var(--code-bg);
      font-family: var(--font-mono);
      font-size: 0.92rem;
      line-height: 1.45;
      overflow-x: auto;
      border-left: 3px solid transparent;

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

    a { color: var(--accent); }

    code, pre, kbd, samp {
      font-family: var(--font-mono);
      font-size: 0.94em;
      line-height: 1.45;
    }

    @media (max-width: 960px) {
      .layout {
        grid-template-columns: minmax(0, 1fr);
        gap: 1.5rem;
      }

      .toc { position: static; }
      .toc-inner { padding-bottom: 1rem; }
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
        {{ range .Specs }}
        <article class="spec">
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
