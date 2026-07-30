package engine

import (
	"context"
	"sync"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

type documentRunner func(context.Context, core.DocumentPlan) (core.DocumentResult, error)

// documentScheduler owns worker cancellation, task admission, and result slots.
type documentScheduler struct {
	ctx         context.Context
	cancel      context.CancelFunc
	budget      *failureBudget
	runDocument documentRunner
}

func newDocumentScheduler(
	parent context.Context,
	budget *failureBudget,
	runDocument documentRunner,
) *documentScheduler {
	ctx, cancel := context.WithCancel(parent)
	return &documentScheduler{
		ctx:         ctx,
		cancel:      cancel,
		budget:      budget,
		runDocument: runDocument,
	}
}

func (scheduler *documentScheduler) close() {
	scheduler.cancel()
}

func (scheduler *documentScheduler) execute(
	documents []core.DocumentPlan,
	jobs int,
) ([]core.DocumentResult, error) {
	results := make([]core.DocumentResult, len(documents))
	if len(documents) == 0 {
		return results, nil
	}

	jobs = effectiveJobs(jobs, len(documents))
	tasks := make(chan int)
	var workers sync.WaitGroup
	var firstErrorOnce sync.Once
	var firstError error

	recordError := func(err error) {
		firstErrorOnce.Do(func() {
			firstError = err
			scheduler.cancel()
		})
	}

	for range jobs {
		workers.Add(1)
		go scheduler.runWorker(documents, results, tasks, recordError, &workers)
	}

	scheduler.sendTasks(tasks, len(documents))
	workers.Wait()

	switch {
	case scheduler.budget.wasHit():
		return results, errMaxFailures
	case firstError != nil:
		return results, firstError
	case scheduler.ctx.Err() != nil:
		return results, scheduler.ctx.Err()
	default:
		return results, nil
	}
}

func effectiveJobs(jobs, documentCount int) int {
	if jobs < 1 {
		jobs = 1
	}
	if jobs > documentCount {
		return documentCount
	}
	return jobs
}

func (scheduler *documentScheduler) runWorker(
	documents []core.DocumentPlan,
	results []core.DocumentResult,
	tasks <-chan int,
	recordError func(error),
	workers *sync.WaitGroup,
) {
	defer workers.Done()
	for {
		select {
		case <-scheduler.ctx.Done():
			return
		case <-scheduler.budget.reachedSignal():
			return
		case index, ok := <-tasks:
			if !ok || scheduler.ctx.Err() != nil || scheduler.budget.wasHit() {
				return
			}
			result, err := scheduler.runDocument(scheduler.ctx, documents[index])
			results[index] = result
			if err != nil {
				recordError(err)
				return
			}
		}
	}
}

func (scheduler *documentScheduler) sendTasks(tasks chan<- int, count int) {
	defer close(tasks)
	for index := range count {
		select {
		case tasks <- index:
		case <-scheduler.ctx.Done():
			return
		case <-scheduler.budget.reachedSignal():
			return
		}
	}
}
