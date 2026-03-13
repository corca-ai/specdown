package html

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/corca-ai/specdown/internal/specdown/config"
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
			docType:  results[i].Document.Frontmatter.Type,
			relPath:  results[i].Document.RelativeTo,
		}
	}
	return docs
}

func buildTocEntry(d docTOC, isCurrent bool, assetRoot string) globalTocEntry {
	href := ""
	if !isCurrent {
		href = assetRoot + "/" + d.htmlPath
	}
	var children []tocItemView
	if isCurrent {
		children = d.headings
	}
	return globalTocEntry{
		Title:    d.title,
		Snippet:  d.snippet,
		Href:     href,
		Status:   d.status,
		Current:  isCurrent,
		Children: children,
		DocType:  d.docType,
	}
}

// flatTOC returns each document as its own standalone section (no grouping).
func flatTOC(docs []docTOC, currentIdx int, assetRoot string) globalTOCView {
	sections := make(globalTOCView, len(docs))
	for j, d := range docs {
		sections[j] = tocSection{
			Entries: []globalTocEntry{buildTocEntry(d, j == currentIdx, assetRoot)},
		}
	}
	return sections
}

// buildGroupedTOC organizes documents into an ordered list of sections.
// Config entries are placed in order; unclaimed documents are auto-grouped by directory.
// entryDir is the directory of the entry document, used as the root for auto-grouping.
func buildGroupedTOC(docs []docTOC, currentIdx int, assetRoot string, tocConfig []config.TOCEntry, entryDir string) (sections globalTOCView, warnings []string) {
	pathIndex := make(map[string]int, len(docs))
	for i, d := range docs {
		pathIndex[d.relPath] = i
	}
	claimed := make(map[int]bool)

	// Phase 1: process explicit TOC config entries in order.
	for _, entry := range tocConfig {
		if entry.Doc != "" {
			idx, ok := pathIndex[entry.Doc]
			if !ok {
				warnings = append(warnings, fmt.Sprintf("toc: %q not found in discovered documents", entry.Doc))
				continue
			}
			claimed[idx] = true
			sections = append(sections, tocSection{
				Entries: []globalTocEntry{buildTocEntry(docs[idx], idx == currentIdx, assetRoot)},
			})
		} else {
			sec, w := buildExplicitGroup(entry, docs, pathIndex, currentIdx, assetRoot, claimed)
			sections = append(sections, sec)
			warnings = append(warnings, w...)
		}
	}

	// Phase 2: auto-group unclaimed documents by directory.
	sections = append(sections, autoGroupUnclaimed(docs, claimed, currentIdx, assetRoot, entryDir)...)

	return
}

func buildExplicitGroup(entry config.TOCEntry, docs []docTOC, pathIndex map[string]int, currentIdx int, assetRoot string, claimed map[int]bool) (section tocSection, warnings []string) {
	var entries []globalTocEntry
	hasCurrent := false
	for _, docPath := range entry.Docs {
		idx, ok := pathIndex[docPath]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("toc: group %q references %q which was not found in discovered documents", entry.Group, docPath))
			continue
		}
		claimed[idx] = true
		if idx == currentIdx {
			hasCurrent = true
		}
		entries = append(entries, buildTocEntry(docs[idx], idx == currentIdx, assetRoot))
	}
	section = tocSection{
		Name:     entry.Group,
		Status:   groupStatus(entries),
		Expanded: hasCurrent,
		Entries:  entries,
	}
	return
}

func autoGroupUnclaimed(docs []docTOC, claimed map[int]bool, currentIdx int, assetRoot, entryDir string) []tocSection {
	dirBuckets, rootIndices := bucketByDirectory(docs, claimed, entryDir)

	var sections []tocSection

	// Root-level docs become standalone sections.
	for _, idx := range rootIndices {
		sections = append(sections, tocSection{
			Entries: []globalTocEntry{buildTocEntry(docs[idx], idx == currentIdx, assetRoot)},
		})
	}

	// Subdirectory docs become named groups.
	for _, bucket := range dirBuckets {
		var entries []globalTocEntry
		hasCurrent := false
		for _, idx := range bucket.indices {
			if idx == currentIdx {
				hasCurrent = true
			}
			entries = append(entries, buildTocEntry(docs[idx], idx == currentIdx, assetRoot))
		}
		sections = append(sections, tocSection{
			Name:     dirGroupName(bucket.dir),
			Status:   groupStatus(entries),
			Expanded: hasCurrent,
			Entries:  entries,
		})
	}

	return sections
}

type dirBucket struct {
	dir     string
	indices []int
}

// bucketByDirectory partitions unclaimed docs into directory buckets and root-level indices.
// entryDir is the canonical root directory; docs in this directory become standalone,
// docs in subdirectories are grouped.
func bucketByDirectory(docs []docTOC, claimed map[int]bool, entryDir string) (buckets []dirBucket, rootIndices []int) {
	dirMap := make(map[string]int)

	for i, d := range docs {
		if claimed[i] {
			continue
		}
		dir := path.Dir(d.relPath)
		if dir == entryDir {
			rootIndices = append(rootIndices, i)
			continue
		}
		if gi, ok := dirMap[dir]; ok {
			buckets[gi].indices = append(buckets[gi].indices, i)
		} else {
			dirMap[dir] = len(buckets)
			buckets = append(buckets, dirBucket{dir: dir, indices: []int{i}})
		}
	}
	return buckets, rootIndices
}

// dirGroupName derives a human-readable group name from a directory path.
func dirGroupName(dir string) string {
	base := path.Base(dir)
	words := strings.Split(base, "-")
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// groupStatus returns the worst status among entries in a group.
func groupStatus(entries []globalTocEntry) string {
	hasXFail := false
	for _, e := range entries {
		if e.Status == "failed" {
			return "failed"
		}
		if e.Status == "expected-fail" {
			hasXFail = true
		}
	}
	if hasXFail {
		return "expected-fail"
	}
	return ""
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
