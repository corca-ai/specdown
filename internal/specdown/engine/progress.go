package engine

import (
	"sync/atomic"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

type progressTracker struct {
	callback   ProgressFunc
	casesTotal int
	caseCount  atomic.Int32
}

func newProgressTracker(callback ProgressFunc, documents []core.DocumentPlan) *progressTracker {
	if callback == nil {
		callback = func(ProgressEvent) {}
	}
	var casesTotal int
	for i := range documents {
		casesTotal += len(documents[i].Cases)
	}
	return &progressTracker{
		callback:   callback,
		casesTotal: casesTotal,
	}
}

func (tracker *progressTracker) documentStarted(path string) {
	tracker.callback(ProgressEvent{Kind: "spec", Spec: path})
}

func (tracker *progressTracker) caseFinished(spec string, result core.CaseResult) {
	caseNumber := int(tracker.caseCount.Add(1))
	tracker.callback(ProgressEvent{
		Kind:       "case",
		Spec:       spec,
		Case:       &result,
		CaseNum:    caseNumber,
		CasesTotal: tracker.casesTotal,
	})
}
