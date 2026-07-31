package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/corca-ai/specdown/internal/specdown/adapterhost"
	"github.com/corca-ai/specdown/internal/specdown/core"
)

// documentRun owns every resource whose lifetime is one document.
type documentRun struct {
	ctx             context.Context
	executor        *documentExecutor
	plan            core.DocumentPlan
	sessions        *sessionManager
	cleanupSessions *sessionManager
}

func (executor *documentExecutor) runDocument(
	ctx context.Context,
	plan core.DocumentPlan,
) (core.DocumentResult, error) {
	if err := ctx.Err(); err != nil {
		return core.DocumentResult{}, err
	}
	executor.progress.documentStarted(plan.Document.RelativeTo)

	if len(plan.Cases) == 0 {
		return core.DocumentResult{
			Document: plan.Document,
			Status:   core.StatusPassed,
		}, nil
	}

	run, err := newDocumentRun(ctx, executor, plan)
	if err != nil {
		return core.DocumentResult{}, err
	}
	return run.execute()
}

func newDocumentRun(
	ctx context.Context,
	executor *documentExecutor,
	plan core.DocumentPlan,
) (*documentRun, error) {
	host := executor.host
	if workdir := plan.Document.Frontmatter.Workdir; workdir != "" {
		documentDir := filepath.Dir(filepath.FromSlash(plan.Document.RelativeTo))
		resolved := filepath.Join(executor.host.BaseDir, documentDir, filepath.FromSlash(workdir))
		if err := os.MkdirAll(resolved, 0o755); err != nil {
			return nil, fmt.Errorf("create workdir %q: %w", workdir, err)
		}
		host = adapterhost.Host{BaseDir: resolved}
	}
	sessionContext := context.WithoutCancel(ctx)
	return &documentRun{
		ctx:             ctx,
		executor:        executor,
		plan:            plan,
		sessions:        newSessionManager(sessionContext, host),
		cleanupSessions: newSessionManager(sessionContext, host),
	}, nil
}

func (run *documentRun) execute() (core.DocumentResult, error) {
	precomputed, lazyAlloy, err := run.prepareAlloy()
	if err != nil {
		return run.finish(core.DocumentResult{}, err)
	}

	cases, lifecycleEvents, runErr := run.executor.runDocumentCases(
		run.ctx,
		run.plan,
		run.sessions,
		run.cleanupSessions,
		precomputed,
		lazyAlloy,
	)
	result := assembleDocumentResult(run.plan.Document, cases, lifecycleEvents, runErr)
	return run.finish(result, runErr)
}

func (run *documentRun) prepareAlloy() (
	precomputed map[string]core.CaseResult,
	lazy bool,
	err error,
) {
	// Documents without hooks can verify Alloy checks in one batch. Documents
	// with hooks defer each Alloy check until its setup scope has succeeded.
	lazyAlloy := len(run.plan.Hooks) > 0
	if lazyAlloy {
		return nil, true, nil
	}
	results, err := run.executor.alloyRunner.RunDocument(run.ctx, run.plan)
	if err != nil {
		return nil, false, err
	}
	return indexResultsByKey(results), false, nil
}

func assembleDocumentResult(
	document core.Document,
	cases []core.CaseResult,
	lifecycleEvents []core.LifecycleEvent,
	runErr error,
) core.DocumentResult {
	hitLimit := errors.Is(runErr, errMaxFailures)
	result := core.DocumentResult{
		Document:        document,
		Status:          documentStatus(cases, lifecycleEvents),
		Cases:           cases,
		LifecycleEvents: lifecycleEvents,
	}
	if runErr == nil || hitLimit || result.Status != core.StatusPassed {
		return result
	}
	if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
		result.Status = core.StatusSkipped
	} else {
		result.Status = core.StatusFailed
	}
	return result
}

func (run *documentRun) finish(
	result core.DocumentResult,
	runErr error,
) (core.DocumentResult, error) {
	hitLimit := errors.Is(runErr, errMaxFailures)
	sessionErr := run.sessions.CloseAll()
	cleanupSessionErr := run.cleanupSessions.CloseAll()

	if runErr == nil && !hitLimit {
		if sessionErr != nil {
			return result, sessionErr
		}
		if cleanupSessionErr != nil {
			return result, cleanupSessionErr
		}
	} else {
		warnSessionClose("adapter sessions", sessionErr)
		warnSessionClose("cleanup adapter sessions", cleanupSessionErr)
	}

	if hitLimit {
		return result, errMaxFailures
	}
	return result, runErr
}

func warnSessionClose(label string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: closing %s: %v\n", label, err)
	}
}
