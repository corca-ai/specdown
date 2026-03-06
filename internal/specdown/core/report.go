package core

import "time"

type Binding struct {
	Name  string
	Value string
}

type EventType string

const (
	EventCaseStarted EventType = "caseStarted"
	EventCasePassed  EventType = "casePassed"
	EventCaseFailed  EventType = "caseFailed"
)

type Event struct {
	Type     EventType
	ID       SpecID
	Label    string
	Message  string
	Expected string
	Actual   string
	Bindings []Binding
}

type CaseResult struct {
	ID             SpecID
	Kind           CaseKind
	Block          string
	Fixture        string
	Label          string
	Template       string
	RenderedSource string
	Columns        []string
	TemplateCells  []string
	RenderedCells  []string
	RowNumber      int
	Status         Status
	Message        string
	Expected       string
	Actual         string
	Bindings       []Binding
	Events         []Event
}

type AlloyCheckResult struct {
	ID                 SpecID
	Model              string
	Assertion          string
	Scope              string
	Label              string
	Status             Status
	Message            string
	Expected           string
	Actual             string
	BundlePath         string
	CounterexamplePath string
}

type DocumentResult struct {
	Document    Document
	Status      Status
	Cases       []CaseResult
	AlloyChecks []AlloyCheckResult
}

type Summary struct {
	SpecsTotal        int
	SpecsPassed       int
	SpecsFailed       int
	CasesTotal        int
	CasesPassed       int
	CasesFailed       int
	AlloyChecksTotal  int
	AlloyChecksPassed int
	AlloyChecksFailed int
}

type Report struct {
	GeneratedAt time.Time
	Results     []DocumentResult
	Summary     Summary
}
