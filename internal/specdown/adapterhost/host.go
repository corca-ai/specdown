package adapterhost

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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

type Session struct {
	adapter     config.AdapterConfig
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	scanner     *bufio.Scanner
	encoder     *json.Encoder
	stderr      *bytes.Buffer
	waited      bool
	closed      bool
	stdinClosed bool
	setupDone   bool
}

func (h Host) Describe(adapter config.AdapterConfig) (Capabilities, error) {
	request := adapterprotocol.Request{
		Type:     "describe",
		Protocol: adapterprotocol.Version,
	}
	responses, err := h.request(adapter, request)
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

func (h Host) StartSession(adapter config.AdapterConfig) (*Session, error) {
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

	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start adapter %q: %w", adapter.Name, err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	return &Session{
		adapter: adapter,
		cmd:     cmd,
		stdin:   stdin,
		scanner: scanner,
		encoder: json.NewEncoder(stdin),
		stderr:  stderr,
	}, nil
}

func (s *Session) Setup() error {
	if s.setupDone {
		return nil
	}
	s.setupDone = true

	request := adapterprotocol.Request{
		Type:     "setup",
		Protocol: adapterprotocol.Version,
	}
	if err := s.encoder.Encode(request); err != nil {
		return fmt.Errorf("write setup to adapter %q: %w", s.adapter.Name, err)
	}

	// Read optional setupDone response; adapter may ignore setup
	// We don't block — the adapter will respond inline with runCase
	return nil
}

func (s *Session) Teardown() error {
	request := adapterprotocol.Request{
		Type:     "teardown",
		Protocol: adapterprotocol.Version,
	}
	if err := s.encoder.Encode(request); err != nil {
		// Adapter may have already exited, that's fine
		return nil
	}
	return nil
}

func (s *Session) RunCase(original core.CaseSpec, prepared core.CaseSpec, visibleBindings []core.Binding, timeoutMs int) (core.CaseResult, error) {
	result := core.CaseResult{
		ID:        original.ID,
		Kind:      original.Kind,
		Block:     original.Block.Descriptor(),
		Fixture:   original.Fixture,
		Label:     defaultLabel(original),
		Columns:   append([]string(nil), original.Columns...),
		RowNumber: original.RowNumber,
	}
	if original.Kind == core.CaseKindCode {
		result.Template = original.Template
		result.RenderedSource = prepared.Template
	} else {
		result.TemplateCells = append([]string(nil), original.Cells...)
		result.RenderedCells = append([]string(nil), prepared.Cells...)
	}

	request := adapterprotocol.Request{
		Type:     "runCase",
		Protocol: adapterprotocol.Version,
		Case: &adapterprotocol.Case{
			ID:           protocolID(prepared.ID),
			Kind:         string(prepared.Kind),
			Block:        prepared.Block.Descriptor(),
			Source:       prepared.Template,
			Fixture:      prepared.Fixture,
			Columns:      append([]string(nil), prepared.Columns...),
			Cells:        append([]string(nil), prepared.Cells...),
			CaptureNames: append([]string(nil), prepared.Block.CaptureNames...),
			Bindings:     protocolBindings(visibleBindings),
		},
	}
	if err := s.encoder.Encode(request); err != nil {
		return core.CaseResult{}, fmt.Errorf("write request to adapter %q: %w", s.adapter.Name, err)
	}

	readResult := make(chan readResultMsg, 1)

	go func() {
		for {
			response, err := s.readResponse()
			if err != nil {
				readResult <- readResultMsg{err: err}
				return
			}
			if err := applyResponse(&result, response); err != nil {
				readResult <- readResultMsg{err: fmt.Errorf("adapter %q: %w", s.adapter.Name, err)}
				return
			}
			if result.Status == core.StatusPassed || result.Status == core.StatusFailed {
				readResult <- readResultMsg{result: result}
				return
			}
		}
	}()

	if timeoutMs > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
		defer cancel()
		select {
		case msg := <-readResult:
			return msg.result, msg.err
		case <-ctx.Done():
			result.Status = core.StatusFailed
			result.Message = fmt.Sprintf("timeout after %dms", timeoutMs)
			return result, nil
		}
	}

	msg := <-readResult
	return msg.result, msg.err
}

type readResultMsg struct {
	result core.CaseResult
	err    error
}

func (s *Session) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true

	if !s.stdinClosed {
		if err := s.stdin.Close(); err != nil {
			return fmt.Errorf("close stdin for adapter %q: %w", s.adapter.Name, err)
		}
		s.stdinClosed = true
	}
	if s.waited {
		return nil
	}
	return s.wait()
}

func (h Host) request(adapter config.AdapterConfig, request adapterprotocol.Request) ([]adapterprotocol.Response, error) {
	session, err := h.StartSession(adapter)
	if err != nil {
		return nil, err
	}

	if err := session.encoder.Encode(request); err != nil {
		session.Close()
		return nil, fmt.Errorf("write request to adapter %q: %w", adapter.Name, err)
	}
	if err := session.stdin.Close(); err != nil {
		session.Close()
		return nil, fmt.Errorf("close stdin for adapter %q: %w", adapter.Name, err)
	}
	session.stdinClosed = true

	var responses []adapterprotocol.Response
	for {
		response, err := session.readResponse()
		if err == io.EOF {
			break
		}
		if err != nil {
			session.Close()
			return nil, err
		}
		responses = append(responses, response)
	}
	if err := session.Close(); err != nil {
		return nil, err
	}
	return responses, nil
}

func (s *Session) readResponse() (adapterprotocol.Response, error) {
	if s.scanner.Scan() {
		var response adapterprotocol.Response
		if err := json.Unmarshal(s.scanner.Bytes(), &response); err != nil {
			return adapterprotocol.Response{}, fmt.Errorf("decode adapter %q response: %w", s.adapter.Name, err)
		}
		return response, nil
	}
	if err := s.scanner.Err(); err != nil {
		return adapterprotocol.Response{}, fmt.Errorf("read adapter %q response: %w", s.adapter.Name, err)
	}
	if err := s.wait(); err != nil {
		return adapterprotocol.Response{}, err
	}
	return adapterprotocol.Response{}, io.EOF
}

func (s *Session) wait() error {
	if s.waited {
		return nil
	}
	s.waited = true
	if err := s.cmd.Wait(); err != nil {
		message := strings.TrimSpace(s.stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("adapter %q infrastructure failure: %s", s.adapter.Name, message)
	}
	return nil
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

func applyResponse(result *core.CaseResult, response adapterprotocol.Response) error {
	if response.Type == "caseStarted" {
		if err := expectResponseID(result.ID, response); err != nil {
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
		return nil
	}

	switch response.Type {
	case "casePassed":
		if err := expectResponseID(result.ID, response); err != nil {
			return err
		}
		if response.Label != "" {
			result.Label = response.Label
		}
		result.Status = core.StatusPassed
		result.Bindings = coreBindings(response.Bindings)
		result.Stderr = response.Stderr
		result.Events = append(result.Events, core.Event{
			Type:     core.EventCasePassed,
			ID:       result.ID,
			Label:    result.Label,
			Bindings: result.Bindings,
		})
		return nil
	case "caseFailed":
		if err := expectResponseID(result.ID, response); err != nil {
			return err
		}
		if response.Label != "" {
			result.Label = response.Label
		}
		result.Status = core.StatusFailed
		result.Message = response.Message
		result.Expected = response.Expected
		result.Actual = response.Actual
		result.Stderr = response.Stderr
		result.Events = append(result.Events, core.Event{
			Type:     core.EventCaseFailed,
			ID:       result.ID,
			Label:    result.Label,
			Message:  result.Message,
			Expected: result.Expected,
			Actual:   result.Actual,
		})
		return nil
	default:
		return fmt.Errorf("unexpected response type %q", response.Type)
	}
}

func expectResponseID(expected core.SpecID, response adapterprotocol.Response) error {
	if response.ID == nil {
		return fmt.Errorf("response %q missing case id", response.Type)
	}
	if coreID(*response.ID).Key() != expected.Key() {
		return fmt.Errorf("response %q referenced unexpected case %s", response.Type, coreID(*response.ID).Key())
	}
	return nil
}

func defaultLabel(specCase core.CaseSpec) string {
	if len(specCase.ID.HeadingPath) == 0 {
		return specCase.DisplayKind()
	}
	label := specCase.DisplayKind() + " @ " + specCase.ID.HeadingPath[len(specCase.ID.HeadingPath)-1]
	if specCase.Kind == core.CaseKindTableRow {
		return label + " row " + fmt.Sprintf("%d", specCase.RowNumber)
	}
	return label
}

func protocolID(id core.SpecID) adapterprotocol.SpecID {
	return adapterprotocol.SpecID{
		File:        id.File,
		HeadingPath: append([]string(nil), id.HeadingPath...),
		Ordinal:     id.Ordinal,
	}
}

func coreID(id adapterprotocol.SpecID) core.SpecID {
	return core.SpecID{
		File:        id.File,
		HeadingPath: append([]string(nil), id.HeadingPath...),
		Ordinal:     id.Ordinal,
	}
}

func protocolBindings(bindings []core.Binding) []adapterprotocol.Binding {
	items := make([]adapterprotocol.Binding, 0, len(bindings))
	for _, binding := range bindings {
		items = append(items, adapterprotocol.Binding{
			Name:  binding.Name,
			Value: binding.Value,
		})
	}
	return items
}

func coreBindings(bindings []adapterprotocol.Binding) []core.Binding {
	items := make([]core.Binding, 0, len(bindings))
	for _, binding := range bindings {
		items = append(items, core.Binding{
			Name:  binding.Name,
			Value: binding.Value,
		})
	}
	return items
}
