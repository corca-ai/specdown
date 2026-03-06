package core

import (
	"strconv"
	"strings"
	"time"
	"unicode"
)

type Status string

const (
	StatusPassed Status = "passed"
	StatusFailed Status = "failed"
)

type SpecID struct {
	File        string
	HeadingPath []string
	Ordinal     int
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

type EventType string

const (
	EventCaseStarted EventType = "caseStarted"
	EventCasePassed  EventType = "casePassed"
	EventCaseFailed  EventType = "caseFailed"
)

type Event struct {
	Type    EventType
	ID      SpecID
	Label   string
	Message string
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
	Info   string
	Source string
	Raw    string
	ID     *SpecID
}

func (CodeBlockNode) isNode() {}

func (n CodeBlockNode) Markdown() string {
	return n.Raw
}

type Document struct {
	Path       string
	RelativeTo string
	Title      string
	Markdown   string
	Nodes      []Node
}

type CaseResult struct {
	ID      SpecID
	Info    string
	Label   string
	Source  string
	Status  Status
	Message string
	Events  []Event
}

type DocumentResult struct {
	Document Document
	Status   Status
	Cases    []CaseResult
}

type Summary struct {
	SpecsTotal  int
	SpecsPassed int
	SpecsFailed int
	CasesTotal  int
	CasesPassed int
	CasesFailed int
}

type Report struct {
	SpecRoot    string
	GeneratedAt time.Time
	Results     []DocumentResult
	Summary     Summary
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
