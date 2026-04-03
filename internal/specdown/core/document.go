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
	File        string      `json:"file"`
	HeadingPath HeadingPath `json:"headingPath"`
	Ordinal     int         `json:"ordinal"`
	Line        int         `json:"line,omitempty"`
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
	return "case-" + Slug(strings.Join(parts, "-"))
}

func HeadingAnchor(file string, headingPath HeadingPath) string {
	parts := append([]string{file}, headingPath...)
	return "section-" + Slug(strings.Join(parts, "-"))
}

type Node interface {
	isNode()
	Markdown() string
}

type HeadingNode struct {
	Level       int         `json:"level"`
	Text        string      `json:"text"`
	Raw         string      `json:"raw"`
	HeadingPath HeadingPath `json:"headingPath,omitempty"`
}

func (HeadingNode) isNode() {}

func (n HeadingNode) Markdown() string {
	return n.Raw
}

type InlineKind string

const (
	InlineExpect InlineKind = "expect"
	InlineCheck  InlineKind = "check"
)

type InlineElement struct {
	Kind        InlineKind        `json:"kind"`
	Raw         string            `json:"raw"`
	ExpectExpr  string            `json:"expectExpr,omitempty"`
	ExpectValue string            `json:"expectValue,omitempty"`
	ExpectFail  bool              `json:"expectFail,omitempty"`
	Check       string            `json:"check,omitempty"`
	CheckParams map[string]string `json:"checkParams,omitempty"`
	ID          *SpecID           `json:"id,omitempty"`
}

type ProseNode struct {
	Raw         string          `json:"raw"`
	Inlines     []InlineElement `json:"inlines,omitempty"`
	HeadingPath HeadingPath     `json:"headingPath,omitempty"`
}

func (ProseNode) isNode() {}

func (n ProseNode) Markdown() string {
	return n.Raw
}

type CodeBlockNode struct {
	Block   BlockSpec `json:"block"`
	Source  string    `json:"source"`
	Raw     string    `json:"raw"`
	Summary string    `json:"summary,omitempty"`
	ID      *SpecID   `json:"id,omitempty"`
}

func (CodeBlockNode) isNode() {}

func (n CodeBlockNode) Markdown() string {
	return n.Raw
}

type AlloyModelNode struct {
	Model       string      `json:"model"`
	Source      string      `json:"source"`
	Raw         string      `json:"raw"`
	HeadingPath HeadingPath `json:"headingPath,omitempty"`
}

func (AlloyModelNode) isNode() {}

func (n AlloyModelNode) Markdown() string {
	return n.Raw
}

type AlloyRefNode struct {
	Model       string      `json:"model"`
	Assertion   string      `json:"assertion"`
	Scope       string      `json:"scope"`
	Raw         string      `json:"raw"`
	HeadingPath HeadingPath `json:"headingPath,omitempty"`
	ID          *SpecID     `json:"id,omitempty"`
}

func (AlloyRefNode) isNode() {}

func (n AlloyRefNode) Markdown() string {
	return n.Raw
}

type TableRowNode struct {
	Cells []string `json:"cells"`
	Raw   string   `json:"raw"`
	Line  int      `json:"line,omitempty"` // 1-based source line number
	ID    *SpecID  `json:"id,omitempty"`
}

type TableNode struct {
	Check       string            `json:"check"`
	CheckParams map[string]string `json:"checkParams,omitempty"`
	Columns     []string          `json:"columns"`
	Rows        []TableRowNode    `json:"rows"`
	Raw         string            `json:"raw"`
}

func (TableNode) isNode() {}

func (n TableNode) Markdown() string {
	return n.Raw
}

type HookKind string

const (
	HookSetup    HookKind = "setup"
	HookTeardown HookKind = "teardown"
)

type HookNode struct {
	Hook        HookKind    `json:"hook"`
	Each        bool        `json:"each"`
	Block       BlockSpec   `json:"block"`
	Source      string      `json:"source"`
	Raw         string      `json:"raw"`
	Summary     string      `json:"summary,omitempty"`
	HeadingPath HeadingPath `json:"headingPath,omitempty"`
}

func (HookNode) isNode()            {}
func (n HookNode) Markdown() string { return n.Raw }

type CheckCallNode struct {
	Check       string            `json:"check"`
	CheckParams map[string]string `json:"checkParams"`
	Raw         string            `json:"raw"`
	HeadingPath HeadingPath       `json:"headingPath,omitempty"`
	ID          *SpecID           `json:"id,omitempty"`
}

func (CheckCallNode) isNode()            {}
func (n CheckCallNode) Markdown() string { return n.Raw }

// CheckDirectiveNode is an intermediate parse node representing a check directive
// (> check:name or > check:name(params)). It is paired with a following TableNode
// during compilation in CompileDocument().
type CheckDirectiveNode struct {
	Check       string            `json:"check"`
	CheckParams map[string]string `json:"checkParams,omitempty"`
	Raw         string            `json:"raw"`
	Line        int               `json:"line,omitempty"`
	HeadingPath HeadingPath       `json:"headingPath,omitempty"`
}

func (CheckDirectiveNode) isNode()            {}
func (n CheckDirectiveNode) Markdown() string { return n.Raw }

type Frontmatter struct {
	Timeout int    `json:"timeout,omitempty"` // milliseconds, 0 = no limit
	Type    string `json:"type,omitempty"`    // trace node type (e.g. "goal", "feature", "test")
	Workdir string `json:"workdir,omitempty"` // working directory relative to spec file
}

type Document struct {
	RelativeTo  string      `json:"relativeTo"`
	Title       string      `json:"title"`
	Markdown    string      `json:"markdown"`
	Nodes       []Node      `json:"nodes"`
	Frontmatter Frontmatter `json:"frontmatter,omitempty"`
	Warnings    []string    `json:"warnings,omitempty"`
}

// Slug converts input to a URL-friendly slug (lowercase, alphanumeric, dashes).
func Slug(input string) string {
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
