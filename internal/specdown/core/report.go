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
	Bindings []Binding
}

type CaseResult struct {
	ID             SpecID
	Block          string
	Label          string
	Template       string
	RenderedSource string
	Status         Status
	Message        string
	Bindings       []Binding
	Events         []Event
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
	GeneratedAt time.Time
	Results     []DocumentResult
	Summary     Summary
}
