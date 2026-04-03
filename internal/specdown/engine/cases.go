package engine

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/corca-ai/specdown/internal/specdown/adapterhost"
	"github.com/corca-ai/specdown/internal/specdown/adapterprotocol"
	"github.com/corca-ai/specdown/internal/specdown/core"
)

func runSingleCase(specCase core.CaseSpec, registry adapterRegistry, sm *sessionManager, visible []core.Binding, timeoutMs int) (core.CaseResult, error) {
	start := time.Now()

	if specCase.Kind == core.CaseKindInlineExpect {
		prepared, err := prepareCase(specCase, visible)
		if err != nil {
			return variableFailure(specCase, err), nil
		}
		result := runInlineExpect(prepared, visible)
		if specCase.InlineExpect.ExpectFail {
			result = applyExpectFail(result)
		}
		result.DurationMs = int(time.Since(start).Milliseconds())
		return result, nil
	}

	adapter, err := registry.adapterFor(specCase)
	if err != nil {
		return core.CaseResult{}, err
	}

	prepared, err := prepareCase(specCase, visible)
	if err != nil {
		return variableFailure(specCase, err), nil
	}

	session, err := sm.For(adapter.Config)
	if err != nil {
		return core.CaseResult{}, err
	}

	var result core.CaseResult
	switch specCase.Kind {
	case core.CaseKindCode:
		result, err = runCodeCase(specCase, prepared, session, timeoutMs)
	case core.CaseKindTableRow:
		result, err = runTableRowCase(specCase, prepared, session, timeoutMs)
	default:
		return core.CaseResult{}, fmt.Errorf("unsupported case kind %q", specCase.Kind)
	}
	if err != nil {
		return result, fmt.Errorf("%s: %s: %w", specCase.ID.File, specCase.ID.Key(), err)
	}
	result.VisibleBindings = visible

	if specCase.Code != nil && specCase.Code.Block.ExpectFail {
		result = applyExpectFail(result)
	}

	result.DurationMs = int(time.Since(start).Milliseconds())
	return result, nil
}

func runCodeCase(specCase, prepared core.CaseSpec, session *adapterhost.Session, timeoutMs int) (core.CaseResult, error) {
	code := specCase.Code
	result := core.CaseResult{
		ID:    specCase.ID,
		Kind:  specCase.Kind,
		Label: specCase.DefaultLabel(),
		Code: &core.CodeResultDetail{
			Block:          code.Block.Descriptor(),
			Template:       code.Template,
			RenderedSource: prepared.Code.Template,
		},
	}

	result.Events = append(result.Events, core.Event{
		Type:  core.EventCaseStarted,
		ID:    result.ID,
		Label: result.Label,
	})

	if core.IsDoctestContent(prepared.Code.Template) {
		return runDoctestCase(specCase, prepared, session, result, timeoutMs)
	}

	resp, err := session.Exec(prepared.Code.Template, timeoutMs)
	if err != nil {
		return result, err
	}

	result.Code.ExitCode = resp.ExitCode
	if resp.Stderr != "" {
		result.Code.Stderr = resp.Stderr
	}

	if resp.Error != "" {
		result.Status = core.StatusFailed
		result.Message = resp.Error
		result.Events = append(result.Events, core.Event{
			Type:    core.EventCaseFailed,
			ID:      result.ID,
			Label:   result.Label,
			Message: resp.Error,
		})
		return result, nil
	}

	result.Status = core.StatusPassed

	// Extract captures from output
	if resp.HasOutput && len(code.Block.CaptureNames) > 0 {
		result.Bindings = captureBindings(resp.Output, code.Block.CaptureNames)
	}

	result.Events = append(result.Events, core.Event{
		Type:     core.EventCasePassed,
		ID:       result.ID,
		Label:    result.Label,
		Bindings: result.Bindings,
	})

	return result, nil
}

func runDoctestCase(_, prepared core.CaseSpec, session *adapterhost.Session, result core.CaseResult, timeoutMs int) (core.CaseResult, error) {
	steps := core.ParseDoctestSource(prepared.Code.Template)
	result.Status = core.StatusPassed

	for _, step := range steps {
		resp, err := session.Exec(step.Command, timeoutMs)
		if err != nil {
			return result, err
		}

		actual, stepStatus := evalDoctestStep(resp, step.Expected)
		result.Code.Steps = append(result.Code.Steps, core.DoctestStep{
			Command:  step.Command,
			Expected: step.Expected,
			Actual:   actual,
			Status:   stepStatus,
		})

		if stepStatus == core.StatusFailed {
			result.Status = core.StatusFailed
			result.Code.ExitCode = resp.ExitCode
			if resp.Stderr != "" {
				result.Code.Stderr = resp.Stderr
			}
		}
	}

	if result.Status == core.StatusFailed {
		result.Events = append(result.Events, core.Event{
			Type:  core.EventCaseFailed,
			ID:    result.ID,
			Label: result.Label,
		})
	} else {
		result.Events = append(result.Events, core.Event{
			Type:  core.EventCasePassed,
			ID:    result.ID,
			Label: result.Label,
		})
	}

	return result, nil
}

func evalDoctestStep(resp adapterprotocol.ExecResponse, expected string) (string, core.Status) {
	switch {
	case resp.Error != "":
		if expected == "" || !core.MatchWithWildcard(resp.Error, expected) {
			return resp.Error, core.StatusFailed
		}
		return resp.Error, core.StatusPassed
	case resp.HasOutput:
		actual := core.ExecResponseToString(resp.Output)
		if expected != "" && !core.MatchWithWildcard(actual, expected) {
			return actual, core.StatusFailed
		}
		return actual, core.StatusPassed
	default:
		if expected != "" {
			return "", core.StatusFailed
		}
		return "", core.StatusPassed
	}
}

func runTableRowCase(specCase, prepared core.CaseSpec, session *adapterhost.Session, timeoutMs int) (core.CaseResult, error) {
	tr := specCase.TableRow
	pr := prepared.TableRow
	result := core.CaseResult{
		ID:    specCase.ID,
		Kind:  specCase.Kind,
		Label: specCase.DefaultLabel(),
		Table: &core.TableResultDetail{
			Check:         tr.Check,
			Columns:       append([]string(nil), tr.Columns...),
			TemplateCells: append([]string(nil), tr.Cells...),
			RenderedCells: append([]string(nil), pr.Cells...),
			RowNumber:     tr.RowNumber,
		},
	}

	result.Events = append(result.Events, core.Event{
		Type:  core.EventCaseStarted,
		ID:    result.ID,
		Label: result.Label,
	})

	resp, err := session.Assert(pr.Check, pr.CheckParams, pr.Columns, pr.Cells, timeoutMs)
	if err != nil {
		return result, err
	}

	switch resp.Type {
	case "passed":
		result.Status = core.StatusPassed
		if resp.Actual != "" {
			result.Actual = resp.Actual
		}
		result.Events = append(result.Events, core.Event{
			Type:  core.EventCasePassed,
			ID:    result.ID,
			Label: result.Label,
		})
	case "failed":
		result.Status = core.StatusFailed
		result.Message = resp.Message
		result.Expected = resp.Expected
		result.Actual = resp.Actual
		if resp.Label != "" {
			result.Label = resp.Label
		}
		result.Events = append(result.Events, core.Event{
			Type:     core.EventCaseFailed,
			ID:       result.ID,
			Label:    result.Label,
			Message:  resp.Message,
			Expected: resp.Expected,
			Actual:   resp.Actual,
		})
	default:
		return result, fmt.Errorf("unexpected assert response type %q", resp.Type)
	}

	return result, nil
}

func captureBindings(rawOutput json.RawMessage, captureNames []string) []core.Binding {
	// Try to parse as string first
	var strValue string
	if err := json.Unmarshal(rawOutput, &strValue); err == nil {
		// String output — split by newlines for captures
		lines := strings.Split(strValue, "\n")
		var bindings []core.Binding
		for i, name := range captureNames {
			var value any = ""
			if i < len(lines) {
				value = lines[i]
			}
			bindings = append(bindings, core.Binding{Name: name, Value: value})
		}
		return bindings
	}

	// Non-string output (object, array, number, etc.) — store as structured value
	if len(captureNames) == 1 {
		var parsed interface{}
		if err := json.Unmarshal(rawOutput, &parsed); err == nil {
			return []core.Binding{{Name: captureNames[0], Value: parsed}}
		}
	}

	// Fallback: store raw JSON string
	var bindings []core.Binding
	for i, name := range captureNames {
		if i == 0 {
			bindings = append(bindings, core.Binding{Name: name, Value: string(rawOutput)})
		} else {
			bindings = append(bindings, core.Binding{Name: name, Value: ""})
		}
	}
	return bindings
}

func runInlineExpect(prepared core.CaseSpec, visible []core.Binding) core.CaseResult {
	ie := prepared.InlineExpect
	result := core.CaseResult{
		ID:              prepared.ID,
		Kind:            prepared.Kind,
		Label:           prepared.DefaultLabel(),
		Expected:        ie.ExpectValue,
		Actual:          ie.Template,
		VisibleBindings: visible,
	}

	result.Events = append(result.Events, core.Event{
		Type:  core.EventCaseStarted,
		ID:    result.ID,
		Label: result.Label,
	})

	if ie.Template == ie.ExpectValue {
		result.Status = core.StatusPassed
		result.Events = append(result.Events, core.Event{
			Type:  core.EventCasePassed,
			ID:    result.ID,
			Label: result.Label,
		})
	} else {
		result.Status = core.StatusFailed
		result.Message = fmt.Sprintf("expected %q, got %q", ie.ExpectValue, ie.Template)
		result.Events = append(result.Events, core.Event{
			Type:     core.EventCaseFailed,
			ID:       result.ID,
			Label:    result.Label,
			Message:  result.Message,
			Expected: result.Expected,
			Actual:   result.Actual,
		})
	}
	return result
}

func applyExpectFail(result core.CaseResult) core.CaseResult {
	result.ExpectFail = true
	if result.Status == core.StatusPassed {
		// Unexpected success — this is a real failure
		result.ExpectFail = false
		result.Status = core.StatusFailed
		result.Message = "expected failure but case passed"
		result.Events = []core.Event{
			{Type: core.EventCaseStarted, ID: result.ID, Label: result.Label},
			{Type: core.EventCaseFailed, ID: result.ID, Label: result.Label, Message: result.Message},
		}
	}
	// When failed: keep status as failed, keep all failure details, just mark ExpectFail
	return result
}
