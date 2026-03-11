package html

import (
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

//go:embed assets/style.css
var styleCSS string

//go:embed assets/script.js
var scriptJS string

type pageView struct {
	Title        string
	Meta         template.HTML
	AssetRoot    string
	GlobalTOC    []globalTocEntry
	Headings     []tocItemView
	Body         template.HTML
	TraceContext template.HTML
}

type tocItemView struct {
	Text     string
	Anchor   string
	Level    int
	Status   string
	Children []tocItemView
}

type globalTocEntry struct {
	Title    string
	Snippet  string
	Href     string
	Status   string
	Current  bool
	Children []tocItemView
}

type docTOC struct {
	title    string
	htmlPath string
	status   string
	snippet  string
	headings []tocItemView
}

// Write generates a multi-page HTML site in outDir.
// outDir is the output directory. Each document result becomes a separate HTML page.
// Shared CSS and JS are written as style.css and script.js.
//
//nolint:gocognit // orchestrator function
func Write(report core.Report, outDir string) error {
	// Remove any existing non-directory at outDir so MkdirAll succeeds.
	if info, err := os.Stat(outDir); err == nil && !info.IsDir() {
		if err := os.Remove(outDir); err != nil {
			return fmt.Errorf("remove stale file at output dir: %w", err)
		}
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	// Write shared assets.
	if err := os.WriteFile(filepath.Join(outDir, "style.css"), []byte(styleCSS), 0o644); err != nil {
		return fmt.Errorf("write style.css: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "script.js"), []byte(scriptJS), 0o644); err != nil {
		return fmt.Errorf("write script.js: %w", err)
	}

	// Determine entry path for relative path computation.
	entryPath := ""
	if len(report.Results) > 0 {
		entryPath = report.Results[0].Document.RelativeTo
	}
	entryDir := path.Dir(path.Clean(entryPath))

	docs := collectDocTOCs(report.Results, entryDir)

	for i, result := range report.Results {
		docType := result.Document.Frontmatter.Type
		meta := buildDocMeta(result, report.GeneratedAt, docType)
		if i == 0 {
			meta = buildMeta(report, docType)
		}
		assetRoot := computeAssetRoot(path.Dir(docs[i].htmlPath))
		globalTOC := buildGlobalTOC(docs, i, assetRoot)

		if err := writePage(outDir, entryDir, result, meta, globalTOC, report.TraceGraph); err != nil {
			return err
		}
	}

	return nil
}

func writePage(outDir, entryDir string, result core.DocumentResult, meta string, globalTOC []globalTocEntry, traceGraph *core.TraceGraphData) error {
	htmlPath := docToHTMLPath(result.Document.RelativeTo, entryDir)
	fullPath := filepath.Join(outDir, filepath.FromSlash(htmlPath))

	body, err := renderDocument(result)
	if err != nil {
		return fmt.Errorf("render %s: %w", result.Document.RelativeTo, err)
	}
	body = rewriteMarkdownLinks(body)
	body = rewriteTraceLinks(body)

	// Build per-page trace context panel if trace data exists.
	var traceCtx string
	if traceGraph != nil {
		traceCtx = renderPageTraceContext(result.Document.RelativeTo, result.Document.Title, traceGraph, entryDir)
	}

	title := result.Document.Title
	if title == "" {
		title = "Specification"
	}

	// Find current page headings from globalTOC.
	var headings []tocItemView
	for _, entry := range globalTOC {
		if entry.Current {
			headings = entry.Children
			break
		}
	}

	view := pageView{
		Title:        title,
		Meta:         template.HTML(meta), //nolint:gosec // meta is internally generated
		AssetRoot:    computeAssetRoot(path.Dir(htmlPath)),
		GlobalTOC:    globalTOC,
		Headings:     headings,
		Body:         template.HTML(body),         //nolint:gosec // body is internally generated
		TraceContext: template.HTML(traceCtx), //nolint:gosec // internally generated
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", htmlPath, err)
	}
	return writeHTMLFile(fullPath, view)
}

func writeHTMLFile(outPath string, view pageView) (err error) {
	file, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	return pageTemplate.Execute(file, view)
}

// docToHTMLPath converts a document's relative path to an HTML output path.
// The path is relative to the entry directory.
func docToHTMLPath(docPath string, entryDir string) string {
	docPath = path.Clean(docPath)
	rel := docPath
	if entryDir != "." && strings.HasPrefix(docPath, entryDir+"/") {
		rel = docPath[len(entryDir)+1:]
	}
	// Replace .spec.md or .md extension with .html
	if strings.HasSuffix(rel, ".spec.md") {
		rel = strings.TrimSuffix(rel, ".spec.md") + ".html"
	} else if strings.HasSuffix(rel, ".md") {
		rel = strings.TrimSuffix(rel, ".md") + ".html"
	}
	return rel
}

// computeAssetRoot returns a relative path from pageDir to the output root.
func computeAssetRoot(pageDir string) string {
	if pageDir == "." || pageDir == "" {
		return "."
	}
	depth := strings.Count(pageDir, "/") + 1
	parts := make([]string, depth)
	for i := range parts {
		parts[i] = ".."
	}
	return strings.Join(parts, "/")
}

func writePills(b *strings.Builder, passed, failed, xfail int) {
	fmt.Fprintf(b, `<span class="pill pass">%d passed</span>`, passed)
	fmt.Fprintf(b, `<span class="pill fail">%d failed</span>`, failed)
	if xfail > 0 {
		fmt.Fprintf(b, `<span class="pill xfail">%d expected</span>`, xfail)
	}
}

func writeTypeBadge(b *strings.Builder, docType string) {
	if docType == "" {
		return
	}
	hue := typeHue(docType)
	fmt.Fprintf(b,
		`<span class="doc-type" style="--type-hue:%d">%s</span>`,
		hue, template.HTMLEscapeString(docType))
}

func buildDocMeta(result core.DocumentResult, generatedAt time.Time, docType string) string {
	passed := 0
	failed := 0
	xfail := 0
	for _, c := range result.Cases {
		switch {
		case c.Status == core.StatusPassed:
			passed++
		case c.ExpectFail:
			xfail++
		default:
			failed++
		}
	}
	var b strings.Builder
	b.WriteString(`<p class="content-meta">`)
	writeTypeBadge(&b, docType)
	writePills(&b, passed, failed, xfail)
	fmt.Fprintf(&b, `<span class="meta-date">%s</span>`,
		template.HTMLEscapeString(generatedAt.Format("2 Jan 2006 15:04")))
	b.WriteString(`</p>`)
	return b.String()
}

func buildMeta(report core.Report, docType string) string {
	passed := report.Summary.CasesPassed
	failed := report.Summary.CasesFailed
	xfail := report.Summary.CasesExpectedFail
	var b strings.Builder
	b.WriteString(`<p class="content-meta">`)
	writeTypeBadge(&b, docType)
	writePills(&b, passed, failed, xfail)
	fmt.Fprintf(&b, `<span class="meta-date">%s</span>`,
		template.HTMLEscapeString(report.GeneratedAt.Format("2 Jan 2006 15:04")))
	b.WriteString(`</p>`)
	return b.String()
}

var pageTemplate = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
  <title>{{ .Title }}</title>
  <link rel="stylesheet" href="{{ .AssetRoot }}/style.css">
</head>
<body>
  <main>
    <div class="layout">
      <aside class="toc" aria-label="Table of contents">
        <div class="toc-inner">
          <p class="toc-title">Contents</p>
          {{ range .GlobalTOC }}
          <section class="toc-spec{{ if .Current }} current{{ end }}">
            {{ if .Current }}<span class="toc-spec-title {{ .Status }}">{{ .Title }}</span>
            {{ else }}<a class="toc-spec-title {{ .Status }}" href="{{ .Href }}">{{ .Title }}</a>
            {{ end }}
            {{ if .Snippet }}<p class="toc-snippet">{{ .Snippet }}</p>{{ end }}
            {{ if and .Current .Children }}
            <ul class="toc-list">
              {{ range .Children }}
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
            {{ end }}
          </section>
          {{ end }}
        </div>
      </aside>

      <div class="content">
        <div class="content-header">
          {{ .Meta }}
        </div>
        <div class="content-body">
          <article class="spec">
            <section class="spec-body">{{ .Body }}</section>
          </article>
        </div>
      </div>
      {{ if .TraceContext }}
      <aside class="trace-context" aria-label="Traceability">
        <div class="trace-ctx-inner">{{ .TraceContext }}</div>
      </aside>
      {{ end }}
      <div class="mobile-title" aria-hidden="true">
        <h1>{{ .Title }}</h1>
      </div>
    </div>
  </main>
  <footer class="site-footer">
    <hr>
    <p><a href="https://github.com/corca-ai/specdown">github.com/corca-ai/specdown</a> · written by ak@corca.ai</p>
  </footer>
  <script src="{{ .AssetRoot }}/script.js"></script>
</body>
</html>
`))
