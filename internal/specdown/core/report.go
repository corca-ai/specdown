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
	ID              SpecID        `json:"id"`
	Kind            CaseKind      `json:"kind"`
	Block           string        `json:"block,omitempty"`
	Check           string        `json:"check,omitempty"`
	Label           string        `json:"label"`
	Template        string        `json:"template,omitempty"`
	RenderedSource  string        `json:"renderedSource,omitempty"`
	Columns         []string      `json:"columns,omitempty"`
	TemplateCells   []string      `json:"templateCells,omitempty"`
	RenderedCells   []string      `json:"renderedCells,omitempty"`
	RowNumber       int           `json:"rowNumber,omitempty"`
	Status          Status        `json:"status"`
	ExpectFail      bool          `json:"expectFail,omitempty"`
	Message         string        `json:"message,omitempty"`
	Expected        string        `json:"expected,omitempty"`
	Actual          string        `json:"actual,omitempty"`
	Bindings        []Binding     `json:"bindings,omitempty"`
	VisibleBindings []Binding     `json:"visibleBindings,omitempty"`
	DurationMs      int           `json:"durationMs,omitempty"`
	Steps           []DoctestStep `json:"steps,omitempty"`
	Events          []Event       `json:"events,omitempty"`
	// Alloy-specific fields (only set when Kind == CaseKindAlloy)
	Model              string `json:"model,omitempty"`
	Assertion          string `json:"assertion,omitempty"`
	Scope              string `json:"scope,omitempty"`
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
	Title       string           `json:"title"`
	GeneratedAt time.Time        `json:"generatedAt"`
	Results     []DocumentResult `json:"results"`
	Summary     Summary          `json:"summary"`
	TraceErrors []string         `json:"traceErrors,omitempty"`
	TraceGraph  *TraceGraphData  `json:"traceGraph,omitempty"`
}
