package core

import (
	"strconv"
	"strings"
	"unicode"
)

type Status string

const (
	StatusPassed Status = "passed"
	StatusFailed Status = "failed"
)

type SpecID struct {
	File        string   `json:"file"`
	HeadingPath []string `json:"headingPath"`
	Ordinal     int      `json:"ordinal"`
}

func (id SpecID) Key() string {
	key := id.File
	for _, part := range id.HeadingPath {
		key += "|" + part
	}
	return key + "|" + strconv.Itoa(id.Ordinal)
}

func (id SpecID) Anchor() string {
	var parts []string
	parts = append(parts, id.File)
	parts = append(parts, id.HeadingPath...)
	parts = append(parts, strconv.Itoa(id.Ordinal))
	return "case-" + slug(strings.Join(parts, "-"))
}

type Node interface {
	isNode()
	Markdown() string
}

type HeadingNode struct {
	Level int
	Text  string
	Raw   string
}

func (HeadingNode) isNode() {}

func (n HeadingNode) Markdown() string {
	return n.Raw
}

type ProseNode struct {
	Raw string
}

func (ProseNode) isNode() {}

func (n ProseNode) Markdown() string {
	return n.Raw
}

type CodeBlockNode struct {
	Block  BlockSpec
	Source string
	Raw    string
	ID     *SpecID
}

func (CodeBlockNode) isNode() {}

func (n CodeBlockNode) Markdown() string {
	return n.Raw
}

type TableRowNode struct {
	Cells []string
	Raw   string
	ID    *SpecID
}

type TableNode struct {
	Fixture string
	Columns []string
	Rows    []TableRowNode
	Raw     string
}

func (TableNode) isNode() {}

func (n TableNode) Markdown() string {
	return n.Raw
}

type Document struct {
	RelativeTo string
	Title      string
	Markdown   string
	Nodes      []Node
}

func slug(input string) string {
	var out strings.Builder
	lastDash := false

	for _, r := range strings.ToLower(input) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			out.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			out.WriteByte('-')
			lastDash = true
		}
	}

	result := strings.Trim(out.String(), "-")
	if result == "" {
		return "spec"
	}
	return result
}
