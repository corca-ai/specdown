package engine

import (
	"sort"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

type scopedBinding struct {
	Binding     core.Binding
	HeadingPath core.HeadingPath
	Order       int
}

type bindingsManager struct {
	bindings []scopedBinding
}

func newBindingsManager() *bindingsManager {
	return &bindingsManager{
		bindings: make([]scopedBinding, 0),
	}
}

// Add records new bindings at the given heading path.
func (m *bindingsManager) Add(bindings []core.Binding, path core.HeadingPath) {
	for _, b := range bindings {
		m.bindings = append(m.bindings, scopedBinding{
			Binding:     b,
			HeadingPath: append(core.HeadingPath(nil), path...),
			Order:       len(m.bindings),
		})
	}
}

// VisibleAt returns bindings visible at the given heading path,
// sorted alphabetically by name.
func (m *bindingsManager) VisibleAt(path core.HeadingPath) []core.Binding {
	selected := make(map[string]scopedBinding)
	for _, binding := range m.bindings {
		if !bindingReachable(binding.HeadingPath, path) {
			continue
		}
		current, ok := selected[binding.Binding.Name]
		if !ok || binding.Order >= current.Order {
			selected[binding.Binding.Name] = binding
		}
	}

	items := make([]core.Binding, 0, len(selected))
	names := make([]string, 0, len(selected))
	for name := range selected {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		items = append(items, selected[name].Binding)
	}
	return items
}

func bindingReachable(bp core.HeadingPath, current core.HeadingPath) bool {
	return bp.Reachable(current)
}
