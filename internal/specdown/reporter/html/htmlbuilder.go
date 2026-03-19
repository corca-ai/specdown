package html

import (
	"html/template"
	"strings"
)

// htmlBuilder wraps strings.Builder with HTML-aware convenience methods.
type htmlBuilder struct {
	strings.Builder
}

// text writes HTML-escaped text.
func (b *htmlBuilder) text(s string) {
	b.WriteString(template.HTMLEscapeString(s))
}

// raw writes a raw HTML string without escaping.
func (b *htmlBuilder) raw(s string) {
	b.WriteString(s)
}

// open writes an opening tag with an optional class attribute.
// Pass an empty class to omit the attribute.
func (b *htmlBuilder) open(tag, class string) {
	b.WriteString(`<`)
	b.WriteString(tag)
	if class != "" {
		b.WriteString(` class="`)
		b.WriteString(class)
		b.WriteString(`"`)
	}
	b.WriteString(`>`)
}

// close writes a closing tag.
func (b *htmlBuilder) close(tag string) {
	b.WriteString(`</`)
	b.WriteString(tag)
	b.WriteString(`>`)
}
