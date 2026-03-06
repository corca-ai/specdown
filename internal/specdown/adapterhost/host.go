package adapterhost

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"specdown/internal/specdown/adapterprotocol"
	"specdown/internal/specdown/config"
	"specdown/internal/specdown/core"
)

type Host struct {
	BaseDir string
}

type Capabilities struct {
	Blocks   []string
	Fixtures []string
}

func (h Host) Describe(adapter config.AdapterConfig) (Capabilities, error) {
	responses, err := h.request(adapter, adapterprotocol.Request{
		Type: "describe",
	})
	if err != nil {
		return Capabilities{}, err
	}
	if len(responses) != 1 || responses[0].Type != "capabilities" {
		return Capabilities{}, fmt.Errorf("adapter %q returned invalid describe response", adapter.Name)
	}
	return Capabilities{
		Blocks:   responses[0].Blocks,
		Fixtures: responses[0].Fixtures,
	}, nil
}

func (h Host) RunCases(adapter config.AdapterConfig, cases []core.CodeBlockNode) (map[string]core.CaseResult, error) {
	requestCases := make([]adapterprotocol.Case, 0, len(cases))
	results := make(map[string]core.CaseResult, len(cases))
	for _, node := range cases {
		if node.ID == nil {
			return nil, fmt.Errorf("adapter %q received non-executable block", adapter.Name)
		}
		requestCases = append(requestCases, adapterprotocol.Case{
			Kind:   "code",
			Info:   node.Block.String(),
			Source: node.Source,
			ID:     *node.ID,
		})
		results[node.ID.Key()] = core.CaseResult{
			ID:     *node.ID,
			Info:   node.Block.String(),
			Label:  defaultLabel(node),
			Source: node.Source,
		}
	}

	responses, err := h.request(adapter, adapterprotocol.Request{
		Type:  "run",
		Cases: requestCases,
	})
	if err != nil {
		return nil, err
	}

	terminal := make(map[string]bool, len(cases))
	for _, response := range responses {
		if err := applyResponse(results, terminal, response); err != nil {
			return nil, fmt.Errorf("adapter %q: %w", adapter.Name, err)
		}
	}

	for _, node := range cases {
		key := node.ID.Key()
		if !terminal[key] {
			return nil, fmt.Errorf("adapter %q did not emit a terminal result for %s", adapter.Name, key)
		}
	}

	return results, nil
}

func (h Host) request(adapter config.AdapterConfig, request adapterprotocol.Request) ([]adapterprotocol.Response, error) {
	command := resolveCommand(h.BaseDir, adapter.Command)

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = h.BaseDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("prepare stdout for adapter %q: %w", adapter.Name, err)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("prepare stdin for adapter %q: %w", adapter.Name, err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start adapter %q: %w", adapter.Name, err)
	}

	encoder := json.NewEncoder(stdin)
	if err := encoder.Encode(request); err != nil {
		stdin.Close()
		return nil, fmt.Errorf("write request to adapter %q: %w", adapter.Name, err)
	}
	if err := stdin.Close(); err != nil {
		return nil, fmt.Errorf("close stdin for adapter %q: %w", adapter.Name, err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	var responses []adapterprotocol.Response
	for scanner.Scan() {
		var response adapterprotocol.Response
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
			return nil, fmt.Errorf("decode adapter %q response: %w", adapter.Name, err)
		}
		responses = append(responses, response)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read adapter %q response: %w", adapter.Name, err)
	}

	if err := cmd.Wait(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("adapter %q infrastructure failure: %s", adapter.Name, message)
	}

	return responses, nil
}

func resolveCommand(baseDir string, command []string) []string {
	resolved := append([]string(nil), command...)
	for i, part := range resolved {
		if filepath.IsAbs(part) {
			continue
		}
		if i == 0 {
			if strings.HasPrefix(part, ".") || strings.Contains(part, string(filepath.Separator)) {
				resolved[i] = filepath.Clean(filepath.Join(baseDir, part))
			}
			continue
		}
		if strings.HasPrefix(part, ".") {
			resolved[i] = filepath.Clean(filepath.Join(baseDir, part))
		}
	}
	return resolved
}

func applyResponse(results map[string]core.CaseResult, terminal map[string]bool, response adapterprotocol.Response) error {
	switch response.Type {
	case "caseStarted":
		result, key, err := resultFor(results, response)
		if err != nil {
			return err
		}
		if response.Label != "" {
			result.Label = response.Label
		}
		result.Events = append(result.Events, core.Event{
			Type:  core.EventCaseStarted,
			ID:    result.ID,
			Label: result.Label,
		})
		results[key] = result
		return nil
	case "casePassed":
		result, key, err := resultFor(results, response)
		if err != nil {
			return err
		}
		if response.Label != "" {
			result.Label = response.Label
		}
		result.Status = core.StatusPassed
		result.Events = append(result.Events, core.Event{
			Type:  core.EventCasePassed,
			ID:    result.ID,
			Label: result.Label,
		})
		results[key] = result
		terminal[key] = true
		return nil
	case "caseFailed":
		result, key, err := resultFor(results, response)
		if err != nil {
			return err
		}
		if response.Label != "" {
			result.Label = response.Label
		}
		result.Status = core.StatusFailed
		result.Message = response.Message
		result.Events = append(result.Events, core.Event{
			Type:    core.EventCaseFailed,
			ID:      result.ID,
			Label:   result.Label,
			Message: result.Message,
		})
		results[key] = result
		terminal[key] = true
		return nil
	default:
		return fmt.Errorf("unexpected response type %q", response.Type)
	}
}

func resultFor(results map[string]core.CaseResult, response adapterprotocol.Response) (core.CaseResult, string, error) {
	if response.ID == nil {
		return core.CaseResult{}, "", fmt.Errorf("response %q missing case id", response.Type)
	}
	key := response.ID.Key()
	result, ok := results[key]
	if !ok {
		return core.CaseResult{}, "", fmt.Errorf("response %q referenced unknown case %s", response.Type, key)
	}
	return result, key, nil
}

func defaultLabel(node core.CodeBlockNode) string {
	if node.ID == nil || len(node.ID.HeadingPath) == 0 {
		return node.Block.String()
	}
	return node.Block.String() + " @ " + node.ID.HeadingPath[len(node.ID.HeadingPath)-1]
}
