package html

import (
	"fmt"
	"html/template"
	"regexp"
	"strings"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

// rewriteTraceLinks transforms trace links (edge::Display text) into ruby-annotated spans.
// Input:  <a href="...">edgeName::Display Text</a>
// Output: <a href="..." class="trace-link">Display Text<span class="annotation">edgeName</span></a>
var traceLinkPattern = regexp.MustCompile(`(<a\s+href="[^"]*")>(([a-z][a-z0-9_]*)::([^<]+))</a>`)

func rewriteTraceLinks(html string) string {
	return traceLinkPattern.ReplaceAllStringFunc(html, func(match string) string {
		parts := traceLinkPattern.FindStringSubmatch(match)
		if len(parts) < 5 {
			return match
		}
		anchor := parts[1]    // <a href="..."
		edgeName := parts[3]  // e.g. "depends"
		display := parts[4]   // e.g. "Spec Syntax"
		return anchor + ` class="trace-link">` +
			template.HTMLEscapeString(display) +
			`<span class="annotation">` +
			template.HTMLEscapeString(edgeName) +
			`</span></a>`
	})
}

// renderPageTraceContext builds a right-side traceability panel showing parents → current → children.
func renderPageTraceContext(docPath, docTitle string, tg *core.TraceGraphData, entryDir string) string {
	type link struct {
		edge     string
		doc      string
		docType  string
		href     string
	}

	// Build type lookup.
	typeOf := make(map[string]string, len(tg.Documents))
	for _, d := range tg.Documents {
		typeOf[d.Path] = d.Type
	}

	var incoming, outgoing []link
	for _, e := range tg.Edges {
		if e.Target == docPath {
			incoming = append(incoming, link{
				edge: e.EdgeName, doc: e.Source,
				docType: typeOf[e.Source], href: docToHTMLPath(e.Source, entryDir),
			})
		}
		if e.Source == docPath {
			outgoing = append(outgoing, link{
				edge: e.EdgeName, doc: e.Target,
				docType: typeOf[e.Target], href: docToHTMLPath(e.Target, entryDir),
			})
		}
	}
	if len(incoming) == 0 && len(outgoing) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(`<p class="trace-ctx-title">Traceability</p>`)

	// Write type tag.
	writeTag := func(t string) {
		if t == "" {
			return
		}
		fmt.Fprintf(&b, `<span class="trace-tag" style="--type-hue:%d">%s</span> `,
			typeHue(t), template.HTMLEscapeString(t))
	}

	// Parents: "[tag] Title *edge*" on a single line, linking into current.
	for _, l := range incoming {
		b.WriteString(`<div class="trace-ctx-parent">`)
		b.WriteString(`<a href="`)
		b.WriteString(template.HTMLEscapeString(l.href))
		b.WriteString(`">`)
		writeTag(l.docType)
		b.WriteString(template.HTMLEscapeString(titleFromPath(l.doc)))
		b.WriteString(`</a>`)
		fmt.Fprintf(&b, ` <span class="trace-ctx-edge">%s</span>`,
			template.HTMLEscapeString(l.edge))
		b.WriteString(`</div>`)
	}

	// Current document — only indent if it has parents.
	curTitle := docTitle
	if curTitle == "" {
		curTitle = titleFromPath(docPath)
	}
	curClass := "trace-ctx-current"
	if len(incoming) == 0 {
		curClass = "trace-ctx-current trace-ctx-root"
	}
	fmt.Fprintf(&b, `<div class="%s">`, curClass)
	writeTag(typeOf[docPath])
	b.WriteString(template.HTMLEscapeString(curTitle))
	b.WriteString(`</div>`)

	// Children: "*edge* [tag] Title" on a single line.
	childClass := "trace-ctx-child"
	if len(incoming) == 0 {
		childClass = "trace-ctx-child trace-ctx-child-root"
	}
	for _, l := range outgoing {
		fmt.Fprintf(&b, `<div class="%s">`, childClass)
		fmt.Fprintf(&b, `<span class="trace-ctx-edge">%s</span> `,
			template.HTMLEscapeString(l.edge))
		b.WriteString(`<a href="`)
		b.WriteString(template.HTMLEscapeString(l.href))
		b.WriteString(`">`)
		writeTag(l.docType)
		b.WriteString(template.HTMLEscapeString(titleFromPath(l.doc)))
		b.WriteString(`</a>`)
		b.WriteString(`</div>`)
	}

	return b.String()
}

// typeHue returns a stable hue (0–359) for a type string.
func typeHue(t string) int {
	var h uint32
	for _, c := range t {
		h = h*31 + uint32(c)
	}
	return int(h % 360)
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
	case core.CheckCallNode:
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
