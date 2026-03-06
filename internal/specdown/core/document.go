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
	Level int    `json:"level"`
	Text  string `json:"text"`
	Raw   string `json:"raw"`
}

func (HeadingNode) isNode() {}

func (n HeadingNode) Markdown() string {
	return n.Raw
}

type ProseNode struct {
	Raw string `json:"raw"`
}

func (ProseNode) isNode() {}

func (n ProseNode) Markdown() string {
	return n.Raw
}

type CodeBlockNode struct {
	Block  BlockSpec `json:"block"`
	Source string    `json:"source"`
	Raw    string    `json:"raw"`
	ID     *SpecID   `json:"id,omitempty"`
}

func (CodeBlockNode) isNode() {}

func (n CodeBlockNode) Markdown() string {
	return n.Raw
}

type AlloyModelNode struct {
	Model       string   `json:"model"`
	Source      string   `json:"source"`
	Raw         string   `json:"raw"`
	HeadingPath []string `json:"headingPath,omitempty"`
}

func (AlloyModelNode) isNode() {}

func (n AlloyModelNode) Markdown() string {
	return n.Raw
}

type AlloyRefNode struct {
	Model       string   `json:"model"`
	Assertion   string   `json:"assertion"`
	Scope       string   `json:"scope"`
	Raw         string   `json:"raw"`
	HeadingPath []string `json:"headingPath,omitempty"`
	ID          *SpecID  `json:"id,omitempty"`
}

func (AlloyRefNode) isNode() {}

func (n AlloyRefNode) Markdown() string {
	return n.Raw
}

type TableRowNode struct {
	Cells []string `json:"cells"`
	Raw   string   `json:"raw"`
	ID    *SpecID  `json:"id,omitempty"`
}

type TableNode struct {
	Fixture string         `json:"fixture"`
	Columns []string       `json:"columns"`
	Rows    []TableRowNode `json:"rows"`
	Raw     string         `json:"raw"`
}

func (TableNode) isNode() {}

func (n TableNode) Markdown() string {
	return n.Raw
}

type Document struct {
	RelativeTo string `json:"relativeTo"`
	Title      string `json:"title"`
	Markdown   string `json:"markdown"`
	Nodes      []Node `json:"nodes"`
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
