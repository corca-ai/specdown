package engine

import (
	"strings"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

type caseFilter interface {
	matches(c core.CaseSpec) bool
}

type typeFilter struct{ kind string }

func (f typeFilter) matches(c core.CaseSpec) bool {
	switch f.kind {
	case "code":
		return c.Kind == core.CaseKindCode
	case "table":
		return c.Kind == core.CaseKindTableRow
	case "expect":
		return c.Kind == core.CaseKindInlineExpect
	case "alloy":
		return c.Kind == core.CaseKindAlloy
	default:
		return false
	}
}

type blockFilter struct{ target string }

func (f blockFilter) matches(c core.CaseSpec) bool {
	return c.Code != nil && c.Code.Block.Target == f.target
}

type checkFilter struct{ name string }

func (f checkFilter) matches(c core.CaseSpec) bool {
	return c.TableRow != nil && c.TableRow.Check == f.name
}

type headingFilter struct{ substring string }

func (f headingFilter) matches(c core.CaseSpec) bool {
	path := c.ID.HeadingPath.Join(" > ")
	return strings.Contains(path, f.substring)
}

func parseFilter(filter string) caseFilter {
	if strings.HasPrefix(filter, "type:") {
		return typeFilter{kind: filter[5:]}
	}
	if strings.HasPrefix(filter, "block:") {
		return blockFilter{target: filter[6:]}
	}
	if strings.HasPrefix(filter, "check:") {
		return checkFilter{name: filter[6:]}
	}
	return headingFilter{substring: filter}
}
