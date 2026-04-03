package engine

import (
	"fmt"
	"os"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

func (c *caseRunContext) runHooksMatching(kind core.HookKind, shouldRun func(core.HookSpec) bool) {
	for i := range c.hooks {
		hook := c.hooks[i]
		if hook.Kind != kind || !shouldRun(hook) {
			continue
		}
		visible := c.bindings.VisibleAt(hook.HeadingPath)
		if err := runHook(hook, c.registry, c.sessions, visible, c.timeoutMs); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s hook failed: %v\n", hook.Kind, err)
		}
	}
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

func runHook(hook core.HookSpec, registry adapterRegistry, sm *sessionManager, visible []core.Binding, timeoutMs int) error {
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

	resp, err := session.Exec(prepared.Code.Template, timeoutMs)
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf("%s hook failed: %s", hook.Kind, resp.Error)
	}
	return nil
}
