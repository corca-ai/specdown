package engine

import (
	"sync"
	"sync/atomic"
)

// failureBudget owns the cross-document unexpected-failure limit.
// Its reached channel closes exactly once so schedulers and in-flight workers
// can stop without polling.
type failureBudget struct {
	maximum  int
	failures atomic.Int32
	hit      atomic.Bool
	reached  chan struct{}
	once     sync.Once
}

func newFailureBudget(maximum int) *failureBudget {
	return &failureBudget{
		maximum: maximum,
		reached: make(chan struct{}),
	}
}

func (budget *failureBudget) recordUnexpectedFailure() bool {
	if budget == nil || budget.maximum <= 0 {
		return false
	}
	if int(budget.failures.Add(1)) < budget.maximum {
		return false
	}
	budget.hit.Store(true)
	budget.once.Do(func() {
		close(budget.reached)
	})
	return true
}

func (budget *failureBudget) wasHit() bool {
	return budget != nil && budget.hit.Load()
}

func (budget *failureBudget) reachedSignal() <-chan struct{} {
	if budget == nil {
		return nil
	}
	return budget.reached
}
