package core

import "time"

type Binding struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
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

type DoctestStep struct {
	Command  string `json:"command"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Status   Status `json:"status"`
}

type CaseResult struct {
	// Common (always set)
	ID              SpecID        `json:"id"`
	Kind            CaseKind      `json:"kind"`
	Status          Status        `json:"status"`
	Label           string        `json:"label"`
	DurationMs      int           `json:"durationMs,omitempty"`
	ExpectFail      bool          `json:"expectFail,omitempty"`
	Message         string        `json:"message,omitempty"`
	Expected        string        `json:"expected,omitempty"`
	Actual          string        `json:"actual,omitempty"`
	Bindings        []Binding     `json:"bindings,omitempty"`
	VisibleBindings []Binding     `json:"visibleBindings,omitempty"`
	Events          []Event       `json:"events,omitempty"`

	// Kind-specific (exactly one is set, or none for InlineExpect)
	Code  *CodeResultDetail  `json:"code,omitempty"`
	Table *TableResultDetail `json:"table,omitempty"`
	Alloy *AlloyResultDetail `json:"alloy,omitempty"`
}

type CodeResultDetail struct {
	Block          string        `json:"block"`
	Template       string        `json:"template,omitempty"`
	RenderedSource string        `json:"renderedSource,omitempty"`
	Steps          []DoctestStep `json:"steps,omitempty"`
}

type TableResultDetail struct {
	Check         string   `json:"check"`
	Columns       []string `json:"columns,omitempty"`
	TemplateCells []string `json:"templateCells,omitempty"`
	RenderedCells []string `json:"renderedCells,omitempty"`
	RowNumber     int      `json:"rowNumber,omitempty"`
}

type AlloyResultDetail struct {
	Model              string `json:"model"`
	Assertion          string `json:"assertion"`
	Scope              string `json:"scope"`
	BundlePath         string `json:"bundlePath,omitempty"`
	SourceMapPath      string `json:"sourceMapPath,omitempty"`
	BundleLine         int    `json:"bundleLine,omitempty"`
	SourceRef          string `json:"sourceRef,omitempty"`
	CounterexamplePath string `json:"counterexamplePath,omitempty"`
}

type DocumentResult struct {
	Document Document     `json:"document"`
	Status   Status       `json:"status"`
	Cases    []CaseResult `json:"cases,omitempty"`
}

type Summary struct {
	SpecsTotal        int `json:"specsTotal"`
	SpecsPassed       int `json:"specsPassed"`
	SpecsFailed       int `json:"specsFailed"`
	CasesTotal        int `json:"casesTotal"`
	CasesPassed       int `json:"casesPassed"`
	CasesFailed       int `json:"casesFailed"`
	CasesExpectedFail int `json:"casesExpectedFail"`
	TraceErrorCount   int `json:"traceErrorCount,omitempty"`
}

// TraceGraphData holds the trace graph for report rendering.
type TraceGraphData struct {
	Documents       []TraceDocument `json:"documents"`
	Edges           []TraceEdge     `json:"edges"`
	TransitiveEdges []TraceEdge     `json:"transitiveEdges,omitempty"`
}

// TraceDocument is a document node in the trace graph.
type TraceDocument struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

// TraceEdge is a directed edge in the trace graph.
type TraceEdge struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	EdgeName string `json:"edgeName"`
}

// ModelRunner runs model verification cases for a document plan.
type ModelRunner interface {
	RunDocument(plan DocumentPlan) ([]CaseResult, error)
}

type Report struct {
	SchemaVersion int              `json:"schemaVersion"`
	Title         string           `json:"title"`
	GeneratedAt   time.Time        `json:"generatedAt"`
	Results       []DocumentResult `json:"results"`
	Summary       Summary          `json:"summary"`
	TraceErrors   []string         `json:"traceErrors,omitempty"`
	TraceGraph    *TraceGraphData  `json:"traceGraph,omitempty"`
}
