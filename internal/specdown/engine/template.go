package engine

import (
	"errors"
	"fmt"
	"strings"

	"github.com/corca-ai/specdown/internal/specdown/binding"
	"github.com/corca-ai/specdown/internal/specdown/core"
)

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
	if code.Block.Literal {
		return &codeCopy, nil
	}
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
	resolver := binding.New(bindings)
	return binding.ReplaceReferences(tmpl, func(reference string) (string, error) {
		value, err := resolver.Resolve(reference)
		if err != nil {
			var undefined *binding.UndefinedError
			if errors.As(err, &undefined) {
				return "", undefinedVariableError(undefined.Name, resolver.Names())
			}
			return "", fmt.Errorf("cannot resolve %q: %w", reference, err)
		}
		return binding.Format(value), nil
	})
}

func undefinedVariableError(name string, names []string) error {
	available := make([]string, len(names))
	for index, availableName := range names {
		available[index] = "$" + availableName
	}
	if len(available) > 0 {
		return fmt.Errorf("variable $%s is not defined; available bindings: %s", name, strings.Join(available, ", "))
	}
	return fmt.Errorf("variable $%s is not defined; no bindings are available in this scope", name)
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
