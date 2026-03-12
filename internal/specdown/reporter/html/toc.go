package html

import (
	"path"
	"regexp"
	"strings"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

func collectDocTOCs(results []core.DocumentResult, entryDir string) []docTOC {
	docs := make([]docTOC, len(results))
	for i := range results {
		htmlPath := docToHTMLPath(results[i].Document.RelativeTo, entryDir)
		title := results[i].Document.Title
		if title == "" {
			title = titleFromPath(results[i].Document.RelativeTo)
		}
		headings := collectHeadings(results[i])
		snippet := ""
		if len(headings) == 0 {
			snippet = extractSnippet(results[i].Document)
		}
		docs[i] = docTOC{
			title:    title,
			htmlPath: htmlPath,
			status:   docStatusClass(results[i]),
			snippet:  snippet,
			headings: headings,
		}
	}
	return docs
}

func buildGlobalTOC(docs []docTOC, currentIdx int, assetRoot string) []globalTocEntry {
	toc := make([]globalTocEntry, len(docs))
	for j, d := range docs {
		isCurrent := j == currentIdx
		href := ""
		if !isCurrent {
			href = assetRoot + "/" + d.htmlPath
		}
		var children []tocItemView
		if isCurrent {
			children = d.headings
		}
		toc[j] = globalTocEntry{
			Title:    d.title,
			Snippet:  d.snippet,
			Href:     href,
			Status:   d.status,
			Current:  isCurrent,
			Children: children,
		}
	}
	return toc
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
			Status: statuses[headingPathKey(heading.HeadingPath)],
		}
		if heading.Level == 2 {
			roots = append(roots, item)
		} else if len(roots) > 0 {
			roots[len(roots)-1].Children = append(roots[len(roots)-1].Children, item)
		}
	}
	return roots
}

// collectHeadingStatuses returns a CSS class per heading path.
// Priority: "failed" > "expected-fail" > "" (passed/empty).
func collectHeadingStatuses(result core.DocumentResult) map[string]string {
	statuses := make(map[string]string)
	mark := func(path []string, status string) {
		for i := 1; i <= len(path); i++ {
			key := headingPathKey(path[:i])
			current := statuses[key]
			switch {
			case current == "failed":
				continue
			case status == "failed":
				statuses[key] = status
			case current == "expected-fail":
				continue
			case status == "expected-fail":
				statuses[key] = status
			}
		}
	}

	for i := range result.Cases {
		switch {
		case result.Cases[i].Status == core.StatusFailed && !result.Cases[i].ExpectFail:
			mark(result.Cases[i].ID.HeadingPath, "failed")
		case result.Cases[i].ExpectFail:
			mark(result.Cases[i].ID.HeadingPath, "expected-fail")
		}
	}
	return statuses
}

func headingPathKey(hp core.HeadingPath) string {
	return hp.Key()
}

// docStatusClass returns "failed", "expected-fail", or "" for a document.
func docStatusClass(result core.DocumentResult) string {
	hasXFail := false
	for i := range result.Cases {
		if result.Cases[i].Status == core.StatusFailed && !result.Cases[i].ExpectFail {
			return "failed"
		}
		if result.Cases[i].ExpectFail {
			hasXFail = true
		}
	}
	if hasXFail {
		return "expected-fail"
	}
	return ""
}

// titleFromPath derives a title from a file path when no H1 is present.
func titleFromPath(docPath string) string {
	base := path.Base(docPath)
	base = strings.TrimSuffix(base, ".spec.md")
	base = strings.TrimSuffix(base, ".md")
	words := strings.Split(base, "-")
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

var mdLinkPattern = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
var mdEmphasisPattern = regexp.MustCompile(`\*\*?([^*]+)\*\*?`)
var mdCodePattern = regexp.MustCompile("`([^`]+)`")

// extractSnippet returns the first sentence of the first prose node, for
// documents that have no headings.
func extractSnippet(doc core.Document) string {
	for _, node := range doc.Nodes {
		prose, ok := node.(core.ProseNode)
		if !ok {
			continue
		}
		text := strings.TrimSpace(prose.Raw)
		if text == "" {
			continue
		}
		return truncateSnippet(stripMarkdownInline(text))
	}
	return ""
}

func stripMarkdownInline(text string) string {
	// Take first paragraph.
	if idx := strings.Index(text, "\n\n"); idx > 0 {
		text = text[:idx]
	}
	text = mdLinkPattern.ReplaceAllString(text, "$1")
	text = mdEmphasisPattern.ReplaceAllString(text, "$1")
	text = mdCodePattern.ReplaceAllString(text, "$1")
	return strings.TrimSpace(text)
}

func truncateSnippet(text string) string {
	if text == "" {
		return ""
	}
	// Take first sentence.
	if idx := strings.Index(text, ". "); idx > 0 && idx < 80 {
		text = text[:idx+1]
	}
	const maxLen = 60
	if len(text) <= maxLen {
		return text
	}
	if idx := strings.LastIndex(text[:maxLen], " "); idx > 20 {
		return text[:idx] + "\u2026"
	}
	return text[:maxLen] + "\u2026"
}
