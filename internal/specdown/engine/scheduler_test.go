package engine

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

func TestFailureBudgetConcurrentLimit(t *testing.T) {
	const calls = 64
	budget := newFailureBudget(8)
	var workers sync.WaitGroup
	workers.Add(calls)
	for range calls {
		go func() {
			defer workers.Done()
			budget.recordUnexpectedFailure()
		}()
	}
	workers.Wait()

	if !budget.wasHit() {
		t.Fatal("failure budget was not marked hit")
	}
	select {
	case <-budget.reachedSignal():
	default:
		t.Fatal("failure budget did not close its reached signal")
	}
}

func TestDocumentSchedulerStopsQueuedWorkAtFailureLimit(t *testing.T) {
	documents := schedulerTestDocuments("first", "second", "third")
	budget := newFailureBudget(1)
	var started []string
	scheduler := newDocumentScheduler(
		context.Background(),
		budget,
		func(_ context.Context, plan core.DocumentPlan) (core.DocumentResult, error) {
			started = append(started, plan.Document.RelativeTo)
			budget.recordUnexpectedFailure()
			return core.DocumentResult{Document: plan.Document}, errMaxFailures
		},
	)
	defer scheduler.close()

	results, err := scheduler.execute(documents, 1)

	if !errors.Is(err, errMaxFailures) {
		t.Fatalf("scheduler error = %v, want failure limit", err)
	}
	if len(started) != 1 || started[0] != "first" {
		t.Fatalf("started documents = %v, want only first", started)
	}
	if results[0].Document.RelativeTo != "first" ||
		results[1].Document.RelativeTo != "" ||
		results[2].Document.RelativeTo != "" {
		t.Fatalf("results = %+v, want only first result populated", results)
	}
}

func TestDocumentSchedulerPreservesInFlightCleanupResult(t *testing.T) {
	documents := schedulerTestDocuments("limit", "cleanup", "queued")
	budget := newFailureBudget(1)
	cleanupStarted := make(chan struct{})
	var startedMu sync.Mutex
	var started []string
	scheduler := newDocumentScheduler(
		context.Background(),
		budget,
		func(ctx context.Context, plan core.DocumentPlan) (core.DocumentResult, error) {
			startedMu.Lock()
			started = append(started, plan.Document.RelativeTo)
			startedMu.Unlock()

			switch plan.Document.RelativeTo {
			case "limit":
				<-cleanupStarted
				budget.recordUnexpectedFailure()
				return core.DocumentResult{Document: plan.Document}, errMaxFailures
			case "cleanup":
				close(cleanupStarted)
				<-ctx.Done()
				return core.DocumentResult{
					Document: plan.Document,
					LifecycleEvents: []core.LifecycleEvent{{
						Scope:   core.LifecycleScopeSection,
						Phase:   core.HookTeardown,
						Status:  core.StatusFailed,
						Message: "cleanup failed",
					}},
				}, ctx.Err()
			default:
				return core.DocumentResult{Document: plan.Document}, nil
			}
		},
	)
	defer scheduler.close()

	results, err := scheduler.execute(documents, 2)

	if !errors.Is(err, errMaxFailures) {
		t.Fatalf("scheduler error = %v, want failure limit", err)
	}
	if results[1].Document.RelativeTo != "cleanup" ||
		len(results[1].LifecycleEvents) != 1 ||
		results[1].LifecycleEvents[0].Message != "cleanup failed" {
		t.Fatalf("cleanup result = %+v, want preserved lifecycle failure", results[1])
	}
	startedMu.Lock()
	defer startedMu.Unlock()
	for _, path := range started {
		if path == "queued" {
			t.Fatalf("queued document started after failure limit: %v", started)
		}
	}
}

func schedulerTestDocuments(paths ...string) []core.DocumentPlan {
	documents := make([]core.DocumentPlan, 0, len(paths))
	for _, path := range paths {
		documents = append(documents, core.DocumentPlan{
			Document: core.Document{RelativeTo: path},
		})
	}
	return documents
}
