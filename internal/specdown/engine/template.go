package engine

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

var variablePattern = core.VariablePattern

func prepareCase(specCase core.CaseSpec, bindings []core.Binding) (core.CaseSpec, error) {
	prepared := specCase
	switch specCase.Kind {
	case core.CaseKindCode:
		code, err := prepareCodeCase(specCase.Code, bindings)
		if err != nil {
			return core.CaseSpec{}, err
		}
		prepared.Code = code
		return prepared, nil
	case core.CaseKindInlineExpect:
		ie, err := prepareInlineExpectCase(specCase.InlineExpect, bindings)
		if err != nil {
			return core.CaseSpec{}, err
		}
		prepared.InlineExpect = ie
		return prepared, nil
	case core.CaseKindTableRow:
		tr, err := prepareTableRowCase(specCase.TableRow, bindings)
		if err != nil {
			return core.CaseSpec{}, err
		}
		prepared.TableRow = tr
		return prepared, nil
	default:
		return core.CaseSpec{}, fmt.Errorf("unsupported case kind %q", specCase.Kind)
	}
}

func prepareCodeCase(code *core.CodeCaseSpec, bindings []core.Binding) (*core.CodeCaseSpec, error) {
	codeCopy := *code
	rendered, err := renderTemplate(codeCopy.Template, bindings)
	if err != nil {
		return nil, err
	}
	codeCopy.Template = rendered
	return &codeCopy, nil
}

func prepareInlineExpectCase(ie *core.InlineExpectCaseSpec, bindings []core.Binding) (*core.InlineExpectCaseSpec, error) {
	ieCopy := *ie
	rendered, err := renderTemplate(ieCopy.Template, bindings)
	if err != nil {
		return nil, err
	}
	ieCopy.Template = rendered
	renderedExpect, err := renderTemplate(ieCopy.ExpectValue, bindings)
	if err != nil {
		return nil, err
	}
	ieCopy.ExpectValue = renderedExpect
	return &ieCopy, nil
}

func prepareTableRowCase(tr *core.TableRowCaseSpec, bindings []core.Binding) (*core.TableRowCaseSpec, error) {
	trCopy := *tr
	rendered := make([]string, 0, len(trCopy.Cells))
	for _, cell := range trCopy.Cells {
		value, err := renderTemplate(cell, bindings)
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, core.UnescapeCell(value))
	}
	trCopy.Cells = rendered
	if len(trCopy.CheckParams) > 0 {
		renderedParams := make(map[string]string, len(trCopy.CheckParams))
		for k, v := range trCopy.CheckParams {
			rv, err := renderTemplate(v, bindings)
			if err != nil {
				return nil, err
			}
			renderedParams[k] = rv
		}
		trCopy.CheckParams = renderedParams
	}
	return &trCopy, nil
}

func renderTemplate(tmpl string, bindings []core.Binding) (string, error) {
	values := make(map[string]any, len(bindings))
	for _, binding := range bindings {
		values[binding.Name] = binding.Value
	}

	var unresolved error
	rendered := variablePattern.ReplaceAllStringFunc(tmpl, func(raw string) string {
		match := variablePattern.FindStringSubmatch(raw)
		if len(match) != 3 {
			return raw
		}
		if match[1] == `\` {
			// escaped \${...} → literal ${...}
			return raw[1:]
		}
		ref := match[2]
		parts := strings.SplitN(ref, ".", 2)
		rootName := parts[0]
		rootValue, ok := values[rootName]
		if !ok {
			unresolved = undefinedVariableError(rootName, values)
			return raw
		}
		if len(parts) == 1 {
			return valueToString(rootValue)
		}
		// Dot-path access
		resolved, err := resolveValue(rootValue, strings.Split(parts[1], "."))
		if err != nil {
			unresolved = fmt.Errorf("cannot resolve %q: %w", ref, err)
			return raw
		}
		return valueToString(resolved)
	})
	if unresolved != nil {
		return "", unresolved
	}
	return rendered, nil
}

func undefinedVariableError(name string, values map[string]any) error {
	names := make([]string, 0, len(values))
	for k := range values {
		names = append(names, "$"+k)
	}
	sort.Strings(names)
	if len(names) > 0 {
		return fmt.Errorf("variable $%s is not defined; available bindings: %s", name, strings.Join(names, ", "))
	}
	return fmt.Errorf("variable $%s is not defined; no bindings are available in this scope", name)
}

func resolveValue(value any, path []string) (any, error) {
	current := value
	for _, key := range path {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("cannot access %q on non-object value", key)
		}
		next, exists := m[key]
		if !exists {
			return nil, fmt.Errorf("key %q not found", key)
		}
		current = next
	}
	return current, nil
}

func valueToString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case nil:
		return ""
	default:
		data, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(data)
	}
}

func variableFailure(specCase core.CaseSpec, err error) core.CaseResult {
	result := core.CaseResult{
		ID:      specCase.ID,
		Kind:    specCase.Kind,
		Label:   specCase.DefaultLabel(),
		Status:  core.StatusFailed,
		Message: err.Error(),
	}

	switch specCase.Kind {
	case core.CaseKindCode:
		result.Code = &core.CodeResultDetail{
			Block:          specCase.Code.Block.Descriptor(),
			Template:       specCase.Code.Template,
			RenderedSource: specCase.Code.Template,
		}
	case core.CaseKindTableRow:
		tr := specCase.TableRow
		result.Table = &core.TableResultDetail{
			Check:         tr.Check,
			Columns:       append([]string(nil), tr.Columns...),
			RowNumber:     tr.RowNumber,
			TemplateCells: append([]string(nil), tr.Cells...),
			RenderedCells: append([]string(nil), tr.Cells...),
		}
	case core.CaseKindAlloy:
		result.Alloy = &core.AlloyResultDetail{
			Model:     specCase.Alloy.Model,
			Assertion: specCase.Alloy.Assertion,
			Scope:     specCase.Alloy.Scope,
		}
	}

	result.Events = append(result.Events, core.Event{
		Type:    core.EventCaseFailed,
		ID:      specCase.ID,
		Label:   result.Label,
		Message: result.Message,
	})
	return result
}
