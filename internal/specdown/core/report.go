package core

import "time"

type Binding struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type EventType string

const (
	EventCaseStarted EventType = "caseStarted"
	EventCasePassed  EventType = "casePassed"
	EventCaseFailed  EventType = "caseFailed"
)

type Event struct {
	Type     EventType `json:"type"`
	ID       SpecID    `json:"id"`
	Label    string    `json:"label,omitempty"`
	Message  string    `json:"message,omitempty"`
	Expected string    `json:"expected,omitempty"`
	Actual   string    `json:"actual,omitempty"`
	Bindings []Binding `json:"bindings,omitempty"`
}

type CaseResult struct {
	ID             SpecID    `json:"id"`
	Kind           CaseKind  `json:"kind"`
	Block          string    `json:"block,omitempty"`
	Fixture        string    `json:"fixture,omitempty"`
	Label          string    `json:"label"`
	Template       string    `json:"template,omitempty"`
	RenderedSource string    `json:"renderedSource,omitempty"`
	Columns        []string  `json:"columns,omitempty"`
	TemplateCells  []string  `json:"templateCells,omitempty"`
	RenderedCells  []string  `json:"renderedCells,omitempty"`
	RowNumber      int       `json:"rowNumber,omitempty"`
	Status         Status    `json:"status"`
	ExpectFail     bool      `json:"expectFail,omitempty"`
	Message        string    `json:"message,omitempty"`
	Expected       string    `json:"expected,omitempty"`
	Actual         string    `json:"actual,omitempty"`
	Bindings       []Binding `json:"bindings,omitempty"`
	Events         []Event   `json:"events,omitempty"`
}

type AlloyCheckResult struct {
	ID                 SpecID `json:"id"`
	Model              string `json:"model"`
	Assertion          string `json:"assertion"`
	Scope              string `json:"scope"`
	Label              string `json:"label"`
	Status             Status `json:"status"`
	Message            string `json:"message,omitempty"`
	BundlePath         string `json:"bundlePath,omitempty"`
	SourceMapPath      string `json:"sourceMapPath,omitempty"`
	BundleLine         int    `json:"bundleLine,omitempty"`
	SourceRef          string `json:"sourceRef,omitempty"`
	CounterexamplePath string `json:"counterexamplePath,omitempty"`
}

type DocumentResult struct {
	Document    Document            `json:"document"`
	Status      Status              `json:"status"`
	Cases       []CaseResult        `json:"cases,omitempty"`
	AlloyChecks []AlloyCheckResult  `json:"alloyChecks,omitempty"`
}

type Summary struct {
	SpecsTotal        int `json:"specsTotal"`
	SpecsPassed       int `json:"specsPassed"`
	SpecsFailed       int `json:"specsFailed"`
	CasesTotal        int `json:"casesTotal"`
	CasesPassed       int `json:"casesPassed"`
	CasesFailed       int `json:"casesFailed"`
	AlloyChecksTotal  int `json:"alloyChecksTotal"`
	AlloyChecksPassed int `json:"alloyChecksPassed"`
	AlloyChecksFailed int `json:"alloyChecksFailed"`
}

type Report struct {
	Title       string           `json:"title"`
	GeneratedAt time.Time        `json:"generatedAt"`
	Results     []DocumentResult `json:"results"`
	Summary     Summary          `json:"summary"`
}
