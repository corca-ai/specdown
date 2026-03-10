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
	ID             SpecID    `json:"id"`
	Kind           CaseKind  `json:"kind"`
	Block          string    `json:"block,omitempty"`
	Check          string    `json:"check,omitempty"`
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
	Bindings       []Binding      `json:"bindings,omitempty"`
	VisibleBindings []Binding     `json:"visibleBindings,omitempty"`
	Steps          []DoctestStep  `json:"steps,omitempty"`
	Events         []Event        `json:"events,omitempty"`
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
	SpecsTotal           int `json:"specsTotal"`
	SpecsPassed          int `json:"specsPassed"`
	SpecsFailed          int `json:"specsFailed"`
	CasesTotal           int `json:"casesTotal"`
	CasesPassed          int `json:"casesPassed"`
	CasesFailed          int `json:"casesFailed"`
	CasesExpectedFail    int `json:"casesExpectedFail"`
	AlloyChecksTotal     int `json:"alloyChecksTotal"`
	AlloyChecksPassed    int `json:"alloyChecksPassed"`
	AlloyChecksFailed    int `json:"alloyChecksFailed"`
	TraceErrorCount      int `json:"traceErrorCount,omitempty"`
}

// TraceGraphData holds the trace graph and its classification for report rendering.
type TraceGraphData struct {
	Documents       []TraceDocument `json:"documents"`
	Edges           []TraceEdge     `json:"edges"`
	TransitiveEdges []TraceEdge     `json:"transitiveEdges,omitempty"`
	Class           string          `json:"class"`
	Layout          string          `json:"layout"`
	Layers          []string        `json:"layers,omitempty"`
}

// TraceDocument is a document node in the trace graph.
type TraceDocument struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

// TraceEdge is a directed edge in the trace graph.
type TraceEdge struct {
	Source   string `json:"source"`
	Target  string `json:"target"`
	EdgeName string `json:"edgeName"`
}

type Report struct {
	Title       string           `json:"title"`
	GeneratedAt time.Time        `json:"generatedAt"`
	Results     []DocumentResult `json:"results"`
	Summary     Summary          `json:"summary"`
	TraceErrors []string         `json:"traceErrors,omitempty"`
	TraceGraph  *TraceGraphData  `json:"traceGraph,omitempty"`
}
