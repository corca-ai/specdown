package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

func (c *caseRunContext) runHooksMatching(
	kind core.HookKind,
	currentPath core.HeadingPath,
	shouldRun func(core.HookSpec) bool,
) *core.HookSpec {
	return c.runHooksMatchingWith(c.ctx, c.sessions, kind, currentPath, shouldRun)
}

//nolint:gocognit // records ordered hook execution, failure, and teardown continuation
func (c *caseRunContext) runHooksMatchingWith(
	ctx context.Context,
	sessions sessionProvider,
	kind core.HookKind,
	currentPath core.HeadingPath,
	shouldRun func(core.HookSpec) bool,
) *core.HookSpec {
	var firstFailure *core.HookSpec
	for step := range len(c.hooks) {
		i := step
		if kind == core.HookTeardown {
			i = len(c.hooks) - 1 - step
		}
		hook := c.hooks[i]
		if hook.Kind != kind || !shouldRun(hook) {
			continue
		}
		activationKey := ""
		if kind == core.HookTeardown {
			activationKey = teardownActivationKey(i, hookExecutionScope(hook, currentPath))
			if _, active := c.activeTeardowns[activationKey]; !active {
				continue
			}
		}
		visible := c.bindings.VisibleAt(hook.HeadingPath)
		startedAt := time.Now()
		event := core.LifecycleEvent{
			Scope:       core.LifecycleScopeSection,
			Phase:       hook.Kind,
			Status:      core.StatusPassed,
			File:        c.spec,
			HeadingPath: hookExecutionScope(hook, currentPath),
			Each:        hook.Each,
		}
		if err := runHook(ctx, hook, c.registry, sessions, visible, c.timeoutMs); err != nil {
			event.Status = core.StatusFailed
			event.Message = err.Error()
		}
		event.DurationMs = int(time.Since(startedAt).Milliseconds())
		c.lifecycleEvents = append(c.lifecycleEvents, event)
		if activationKey != "" {
			delete(c.activeTeardowns, activationKey)
		}
		if event.Status == core.StatusFailed {
			if firstFailure == nil {
				failed := hook
				firstFailure = &failed
			}
			if kind == core.HookSetup {
				return firstFailure
			}
		}
	}
	return firstFailure
}

func shouldRunHook(hook core.HookSpec, prevPath, currPath core.HeadingPath) bool {
	if !hook.HeadingPath.IsPrefix(currPath) {
		return false
	}
	if !hook.Each {
		return !hook.HeadingPath.IsPrefix(prevPath)
	}
	depth := len(hook.HeadingPath)
	if len(currPath) <= depth {
		return false
	}
	if !hook.HeadingPath.IsPrefix(prevPath) || len(prevPath) <= depth {
		return true
	}
	return currPath[depth] != prevPath[depth]
}

func shouldRunTeardownHook(hook core.HookSpec, currPath, nextPath core.HeadingPath) bool {
	if !hook.HeadingPath.IsPrefix(currPath) {
		return false
	}
	if !hook.Each {
		return !hook.HeadingPath.IsPrefix(nextPath)
	}
	depth := len(hook.HeadingPath)
	if len(currPath) <= depth {
		return false
	}
	if !hook.HeadingPath.IsPrefix(nextPath) || len(nextPath) <= depth {
		return true
	}
	return currPath[depth] != nextPath[depth]
}

func hookExecutionScope(hook core.HookSpec, currPath core.HeadingPath) core.HeadingPath {
	if !hook.Each {
		return append(core.HeadingPath(nil), hook.HeadingPath...)
	}
	depth := len(hook.HeadingPath) + 1
	if depth > len(currPath) {
		depth = len(currPath)
	}
	return append(core.HeadingPath(nil), currPath[:depth]...)
}

func runHook(ctx context.Context, hook core.HookSpec, registry adapterRegistry, sm sessionProvider, visible []core.Binding, timeoutMs int) error {
	synthetic := core.CaseSpec{
		ID: core.SpecID{
			File:        "_hook",
			HeadingPath: hook.HeadingPath,
		},
		Kind: core.CaseKindCode,
		Code: &core.CodeCaseSpec{
			Block:    hook.Block,
			Template: hook.Source,
		},
	}

	adapter, err := registry.adapterFor(synthetic)
	if err != nil {
		return err
	}

	prepared, err := prepareCase(synthetic, visible)
	if err != nil {
		return err
	}

	session, err := sm.For(adapter.Config)
	if err != nil {
		return err
	}

	resp, err := session.ExecContext(ctx, prepared.Code.Template, timeoutMs)
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf("%s hook failed: %s", hook.Kind, resp.Error)
	}
	return nil
}
