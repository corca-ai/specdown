package core

import (
	"fmt"
	"time"
)

func Run(specRoot string) (Report, error) {
	docs, err := Discover(specRoot)
	if err != nil {
		return Report{}, err
	}
	if len(docs) == 0 {
		return Report{}, fmt.Errorf("no .spec.md files found under %s", specRoot)
	}

	results := make([]DocumentResult, 0, len(docs))
	summary := Summary{SpecsTotal: len(docs)}

	for _, doc := range docs {
		result := runDocument(doc)
		results = append(results, result)

		if result.Status == StatusPassed {
			summary.SpecsPassed++
		} else {
			summary.SpecsFailed++
		}

		summary.CasesTotal += len(result.Cases)
		for _, caseResult := range result.Cases {
			if caseResult.Status == StatusPassed {
				summary.CasesPassed++
			} else {
				summary.CasesFailed++
			}
		}
	}

	return Report{
		SpecRoot:    specRoot,
		GeneratedAt: time.Now(),
		Results:     results,
		Summary:     summary,
	}, nil
}

func runDocument(doc Document) DocumentResult {
	runtime := newBoardRuntime()
	cases := make([]CaseResult, 0)
	status := StatusPassed

	for _, node := range doc.Nodes {
		code, ok := node.(CodeBlockNode)
		if !ok || code.ID == nil {
			continue
		}

		result := executeCodeBlock(runtime, code)
		cases = append(cases, result)
		if result.Status == StatusFailed {
			status = StatusFailed
		}
	}

	return DocumentResult{
		Document: doc,
		Status:   status,
		Cases:    cases,
	}
}

func executeCodeBlock(runtime *boardRuntime, node CodeBlockNode) CaseResult {
	id := *node.ID
	label := caseLabel(node)

	result := CaseResult{
		ID:     id,
		Info:   node.Block.String(),
		Label:  label,
		Source: node.Source,
		Status: StatusPassed,
		Events: []Event{{
			Type:  EventCaseStarted,
			ID:    id,
			Label: label,
		}},
	}

	if err := executeBoardBlock(runtime, node); err != nil {
		result.Status = StatusFailed
		result.Message = err.Error()
		result.Events = append(result.Events, Event{
			Type:    EventCaseFailed,
			ID:      id,
			Label:   label,
			Message: err.Error(),
		})
		return result
	}

	result.Events = append(result.Events, Event{
		Type:  EventCasePassed,
		ID:    id,
		Label: label,
	})
	return result
}

func caseLabel(node CodeBlockNode) string {
	if node.ID == nil || len(node.ID.HeadingPath) == 0 {
		return node.Block.String()
	}
	return node.Block.String() + " @ " + node.ID.HeadingPath[len(node.ID.HeadingPath)-1]
}

func executeBoardBlock(runtime *boardRuntime, node CodeBlockNode) error {
	switch node.Block.Kind {
	case BlockKindRun:
		return runtime.Run(node.Source)
	case BlockKindVerify:
		return runtime.Verify(node.Source)
	default:
		return fmt.Errorf("unsupported executable block %q", node.Block.String())
	}
}
