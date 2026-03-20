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

	"github.com/corca-ai/specdown/internal/specdown/adapterprotocol"
	"github.com/corca-ai/specdown/internal/specdown/config"
	"github.com/corca-ai/specdown/internal/specdown/jqadapter"
	"github.com/corca-ai/specdown/internal/specdown/shelladapter"
)

type Host struct {
	BaseDir string
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
	nextID      int
	builtin     bool
	done        chan struct{} // signals builtin goroutine completion
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

func (h Host) StartBuiltinShellSession(adapter config.AdapterConfig) (*Session, error) {
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	done := make(chan struct{})
	go func() {
		defer close(done)
		builtinShellLoop(stdinReader, stdoutWriter)
		_ = stdoutWriter.Close()
	}()

	scanner := bufio.NewScanner(stdoutReader)
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	return &Session{
		adapter: adapter,
		stdin:   stdinWriter,
		scanner: scanner,
		encoder: json.NewEncoder(stdinWriter),
		stderr:  &bytes.Buffer{},
		builtin: true,
		done:    done,
	}, nil
}

func (h Host) StartBuiltinJQSession(adapter config.AdapterConfig) (*Session, error) {
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	done := make(chan struct{})
	go func() {
		defer close(done)
		builtinJQLoop(stdinReader, stdoutWriter)
		_ = stdoutWriter.Close()
	}()

	scanner := bufio.NewScanner(stdoutReader)
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	return &Session{
		adapter: adapter,
		stdin:   stdinWriter,
		scanner: scanner,
		encoder: json.NewEncoder(stdinWriter),
		stderr:  &bytes.Buffer{},
		builtin: true,
		done:    done,
	}, nil
}

func builtinJQLoop(reader io.Reader, writer io.Writer) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	encoder := json.NewEncoder(writer)

	for scanner.Scan() {
		var req adapterprotocol.AssertRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			return
		}
		if err := encoder.Encode(jqadapter.Assert(req.ID, &req)); err != nil {
			return
		}
	}
}

func builtinShellLoop(reader io.Reader, writer io.Writer) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	encoder := json.NewEncoder(writer)

	for scanner.Scan() {
		if err := handleBuiltinMessage(scanner.Bytes(), encoder); err != nil {
			return
		}
	}
}

func handleBuiltinMessage(raw []byte, encoder *json.Encoder) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}

	typeRaw, ok := fields["type"]
	if !ok {
		return fmt.Errorf("adapter response missing \"type\" field (expected \"exec\")")
	}
	var msgType string
	if err := json.Unmarshal(typeRaw, &msgType); err != nil {
		return err
	}

	switch msgType {
	case "exec":
		var req adapterprotocol.ExecRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return err
		}
		return encoder.Encode(shelladapter.Exec(req.ID, req.Source))
	default:
		return fmt.Errorf("unknown type %q", msgType)
	}
}

func (s *Session) Exec(source string, timeoutMs int) (adapterprotocol.ExecResponse, error) {
	s.nextID++
	seqID := s.nextID

	request := adapterprotocol.ExecRequest{
		Type:   "exec",
		ID:     seqID,
		Source: source,
	}
	if err := s.encoder.Encode(request); err != nil {
		return adapterprotocol.ExecResponse{}, fmt.Errorf("write exec to adapter %q: %w", s.adapter.Name, err)
	}

	type result struct {
		resp adapterprotocol.ExecResponse
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		raw, err := s.readRawResponse()
		if err != nil {
			ch <- result{err: err}
			return
		}
		resp, err := adapterprotocol.ParseExecResponse(raw)
		if err != nil {
			ch <- result{err: fmt.Errorf("adapter %q: %w", s.adapter.Name, err)}
			return
		}
		if resp.ID != seqID {
			ch <- result{err: fmt.Errorf("adapter %q: response referenced unexpected id %d (expected %d)", s.adapter.Name, resp.ID, seqID)}
			return
		}
		ch <- result{resp: resp}
	}()

	if timeoutMs > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
		defer cancel()
		select {
		case r := <-ch:
			return r.resp, r.err
		case <-ctx.Done():
			return adapterprotocol.ExecResponse{ID: seqID, Error: fmt.Sprintf("timeout after %dms (exec: %q)", timeoutMs, truncate(source, 80))}, nil
		}
	}

	r := <-ch
	return r.resp, r.err
}

func (s *Session) Assert(check string, params map[string]string, columns, cells []string, timeoutMs int) (adapterprotocol.AssertResponse, error) {
	s.nextID++
	seqID := s.nextID

	request := adapterprotocol.AssertRequest{
		Type:        "assert",
		ID:          seqID,
		Check:       check,
		CheckParams: params,
		Columns:     columns,
		Cells:       cells,
	}
	if err := s.encoder.Encode(request); err != nil {
		return adapterprotocol.AssertResponse{}, fmt.Errorf("write assert to adapter %q: %w", s.adapter.Name, err)
	}

	type result struct {
		resp adapterprotocol.AssertResponse
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		raw, err := s.readRawResponse()
		if err != nil {
			ch <- result{err: err}
			return
		}
		var resp adapterprotocol.AssertResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			ch <- result{err: fmt.Errorf("adapter %q: decode assert response: %w", s.adapter.Name, err)}
			return
		}
		if resp.ID != seqID {
			ch <- result{err: fmt.Errorf("adapter %q: response referenced unexpected id %d (expected %d)", s.adapter.Name, resp.ID, seqID)}
			return
		}
		ch <- result{resp: resp}
	}()

	if timeoutMs > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
		defer cancel()
		select {
		case r := <-ch:
			return r.resp, r.err
		case <-ctx.Done():
			return adapterprotocol.AssertResponse{ID: seqID, Type: "failed", Message: fmt.Sprintf("timeout after %dms (assert: check %q)", timeoutMs, truncate(check, 80))}, nil
		}
	}

	r := <-ch
	return r.resp, r.err
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
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

func (s *Session) readRawResponse() ([]byte, error) {
	if s.scanner.Scan() {
		return append([]byte(nil), s.scanner.Bytes()...), nil
	}
	if err := s.scanner.Err(); err != nil {
		if err.Error() == "bufio.Scanner: token too long" {
			return nil, fmt.Errorf("adapter %q response exceeded buffer limit (1 MB); consider reducing output size", s.adapter.Name)
		}
		return nil, fmt.Errorf("read adapter %q response: %w", s.adapter.Name, err)
	}
	if err := s.wait(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

func (s *Session) wait() error {
	if s.waited {
		return nil
	}
	s.waited = true
	if s.builtin {
		<-s.done
		return nil
	}
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
