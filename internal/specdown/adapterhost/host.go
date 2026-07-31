package adapterhost

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/corca-ai/specdown/internal/specdown/adapterprotocol"
	"github.com/corca-ai/specdown/internal/specdown/config"
	"github.com/corca-ai/specdown/internal/specdown/jqadapter"
	"github.com/corca-ai/specdown/internal/specdown/shelladapter"
	"github.com/corca-ai/specdown/internal/specdown/subprocess"
)

const (
	adapterExitGrace         = 2 * time.Second
	adapterResponseLineLimit = 1024 * 1024
)

type Host struct {
	BaseDir string
}

type Session struct {
	adapter      config.AdapterConfig
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	stdout       io.ReadCloser
	scanner      *bufio.Scanner
	encoder      *json.Encoder
	stderr       *bytes.Buffer
	closed       bool
	stdinClosed  bool
	poisoned     atomic.Bool
	nextID       int
	baseDir      string
	builtinShell bool
	builtinJQ    bool
	done         chan struct{}
	waitErr      error
}

// Usable reports whether the session can accept another request.
func (s *Session) Usable() bool {
	return !s.poisoned.Load() && !s.closed
}

func (h Host) StartSession(adapter config.AdapterConfig) (*Session, error) {
	return h.StartSessionContext(context.Background(), adapter)
}

func (h Host) StartSessionContext(ctx context.Context, adapter config.AdapterConfig) (*Session, error) {
	command := resolveCommand(h.BaseDir, adapter.Command)
	cmd := subprocess.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = h.BaseDir

	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("prepare stdin for adapter %q: %w", adapter.Name, err)
	}
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		_ = stdinReader.Close()
		_ = stdinWriter.Close()
		return nil, fmt.Errorf("prepare stdout for adapter %q: %w", adapter.Name, err)
	}
	cmd.Stdin = stdinReader
	cmd.Stdout = stdoutWriter

	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		_ = stdinReader.Close()
		_ = stdinWriter.Close()
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		return nil, fmt.Errorf("start adapter %q: %w", adapter.Name, err)
	}
	_ = stdinReader.Close()
	_ = stdoutWriter.Close()

	scanner := bufio.NewScanner(stdoutReader)
	// Scanner counts the trailing newline while enforcing its token buffer.
	// Keep one extra byte so a response whose JSON payload is exactly at the
	// documented line limit remains valid.
	scanner.Buffer(make([]byte, 1024), adapterResponseLineLimit+1)

	session := &Session{
		adapter: adapter,
		cmd:     cmd,
		stdin:   stdinWriter,
		stdout:  stdoutReader,
		scanner: scanner,
		encoder: json.NewEncoder(stdinWriter),
		stderr:  stderr,
		done:    make(chan struct{}),
	}
	go session.collectWait()
	return session, nil
}

func (h Host) StartBuiltinShellSession(adapter config.AdapterConfig) (*Session, error) {
	return &Session{
		adapter:      adapter,
		baseDir:      h.BaseDir,
		stderr:       &bytes.Buffer{},
		builtinShell: true,
	}, nil
}

func (h Host) StartBuiltinJQSession(adapter config.AdapterConfig) (*Session, error) {
	return &Session{
		adapter:   adapter,
		stderr:    &bytes.Buffer{},
		builtinJQ: true,
	}, nil
}

func (s *Session) Exec(source string, timeoutMs int) (adapterprotocol.ExecResponse, error) {
	return s.ExecContext(context.Background(), source, timeoutMs)
}

func (s *Session) ExecContext(parent context.Context, source string, timeoutMs int) (adapterprotocol.ExecResponse, error) {
	if s.poisoned.Load() {
		return adapterprotocol.ExecResponse{}, fmt.Errorf("adapter %q session is unusable after a previous timeout", s.adapter.Name)
	}
	if err := parent.Err(); err != nil {
		return adapterprotocol.ExecResponse{}, err
	}

	s.nextID++
	seqID := s.nextID
	requestCtx, cancel := requestContext(parent, timeoutMs)
	defer cancel()

	if s.builtinShell {
		return s.execBuiltinShell(parent, requestCtx, seqID, source, timeoutMs)
	}
	return s.execExternal(parent, requestCtx, seqID, source, timeoutMs)
}

func (s *Session) execBuiltinShell(parent, requestCtx context.Context, seqID int, source string, timeoutMs int) (adapterprotocol.ExecResponse, error) {
	raw, err := json.Marshal(shelladapter.ExecContext(requestCtx, seqID, source, s.baseDir))
	if requestCtx.Err() != nil {
		s.poison()
		if parent.Err() != nil {
			return adapterprotocol.ExecResponse{}, parent.Err()
		}
		return execTimeoutResponse(seqID, source, timeoutMs), nil
	}
	if err != nil {
		return adapterprotocol.ExecResponse{}, fmt.Errorf("encode builtin shell response: %w", err)
	}
	resp, err := adapterprotocol.ParseExecResponse(raw)
	if err != nil {
		return adapterprotocol.ExecResponse{}, fmt.Errorf("adapter %q: %w", s.adapter.Name, err)
	}
	return resp, nil
}

func (s *Session) execExternal(parent, requestCtx context.Context, seqID int, source string, timeoutMs int) (adapterprotocol.ExecResponse, error) {
	request := adapterprotocol.ExecRequest{
		Type:   "exec",
		ID:     seqID,
		Source: source,
	}

	type result struct {
		resp adapterprotocol.ExecResponse
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		if err := s.encoder.Encode(request); err != nil {
			ch <- result{err: fmt.Errorf("write exec to adapter %q: %w", s.adapter.Name, err)}
			return
		}
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
		if resp.ID != request.ID {
			ch <- result{err: fmt.Errorf("adapter %q: response referenced unexpected id %d (expected %d)", s.adapter.Name, resp.ID, seqID)}
			return
		}
		ch <- result{resp: resp}
	}()

	select {
	case r := <-ch:
		return r.resp, r.err
	case <-requestCtx.Done():
		s.poison()
		if parent.Err() != nil {
			return adapterprotocol.ExecResponse{}, parent.Err()
		}
		return execTimeoutResponse(seqID, source, timeoutMs), nil
	}
}

func (s *Session) Assert(check string, params map[string]string, columns, cells []string, timeoutMs int) (adapterprotocol.AssertResponse, error) {
	return s.AssertContext(context.Background(), check, params, columns, cells, timeoutMs)
}

func (s *Session) AssertContext(parent context.Context, check string, params map[string]string, columns, cells []string, timeoutMs int) (adapterprotocol.AssertResponse, error) {
	if s.poisoned.Load() {
		return adapterprotocol.AssertResponse{}, fmt.Errorf("adapter %q session is unusable after a previous timeout", s.adapter.Name)
	}
	if err := parent.Err(); err != nil {
		return adapterprotocol.AssertResponse{}, err
	}

	s.nextID++
	seqID := s.nextID
	requestCtx, cancel := requestContext(parent, timeoutMs)
	defer cancel()

	request := adapterprotocol.AssertRequest{
		Type:        "assert",
		ID:          seqID,
		Check:       check,
		CheckParams: params,
		Columns:     columns,
		Cells:       cells,
	}

	if s.builtinJQ {
		return s.assertBuiltinJQ(parent, requestCtx, &request, timeoutMs)
	}
	return s.assertExternal(parent, requestCtx, request, timeoutMs)
}

func (s *Session) assertBuiltinJQ(parent, requestCtx context.Context, request *adapterprotocol.AssertRequest, timeoutMs int) (adapterprotocol.AssertResponse, error) {
	resp := jqadapter.AssertContext(requestCtx, request.ID, request)
	if requestCtx.Err() != nil {
		s.poison()
		if parent.Err() != nil {
			return adapterprotocol.AssertResponse{}, parent.Err()
		}
		return assertTimeoutResponse(request.ID, request.Check, timeoutMs), nil
	}
	return resp, nil
}

func (s *Session) assertExternal(parent, requestCtx context.Context, request adapterprotocol.AssertRequest, timeoutMs int) (adapterprotocol.AssertResponse, error) {
	type result struct {
		resp adapterprotocol.AssertResponse
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		if err := s.encoder.Encode(request); err != nil {
			ch <- result{err: fmt.Errorf("write assert to adapter %q: %w", s.adapter.Name, err)}
			return
		}
		raw, err := s.readRawResponse()
		if err != nil {
			ch <- result{err: err}
			return
		}
		resp, err := adapterprotocol.ParseAssertResponse(raw)
		if err != nil {
			ch <- result{err: fmt.Errorf("adapter %q: %w", s.adapter.Name, err)}
			return
		}
		if resp.ID != request.ID {
			ch <- result{err: fmt.Errorf("adapter %q: response referenced unexpected id %d (expected %d)", s.adapter.Name, resp.ID, request.ID)}
			return
		}
		ch <- result{resp: resp}
	}()

	select {
	case r := <-ch:
		return r.resp, r.err
	case <-requestCtx.Done():
		s.poison()
		if parent.Err() != nil {
			return adapterprotocol.AssertResponse{}, parent.Err()
		}
		return assertTimeoutResponse(request.ID, request.Check, timeoutMs), nil
	}
}

func requestContext(parent context.Context, timeoutMs int) (context.Context, context.CancelFunc) {
	if timeoutMs > 0 {
		return context.WithTimeout(parent, time.Duration(timeoutMs)*time.Millisecond)
	}
	return context.WithCancel(parent)
}

func execTimeoutResponse(id int, source string, timeoutMs int) adapterprotocol.ExecResponse {
	return adapterprotocol.ExecResponse{
		ID:    id,
		Error: fmt.Sprintf("timeout after %dms (exec: %q)", timeoutMs, truncate(source, 80)),
	}
}

func assertTimeoutResponse(id int, check string, timeoutMs int) adapterprotocol.AssertResponse {
	return adapterprotocol.AssertResponse{
		ID:      id,
		Type:    adapterprotocol.AssertResponseFailed,
		Message: fmt.Sprintf("timeout after %dms (assert: check %q)", timeoutMs, truncate(check, 80)),
	}
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}

// poison marks the session unusable, closes its input, and terminates any
// external adapter process so a timed-out request cannot keep the run alive.
func (s *Session) poison() {
	s.poisoned.Store(true)
	_ = s.closeStdin()
	if s.cmd != nil {
		_ = subprocess.Terminate(s.cmd)
		_, _ = s.waitFor(adapterExitGrace)
	}
	_ = s.closeStdout()
}

func (s *Session) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true

	if s.builtinShell || s.builtinJQ {
		return nil
	}

	stdinErr := s.closeStdin()
	completed, waitErr := s.waitFor(adapterExitGrace)
	if completed {
		return errors.Join(stdinErr, s.closeStdout(), waitErr)
	}

	s.poisoned.Store(true)
	terminateErr := subprocess.Terminate(s.cmd)
	completed, waitErr = s.waitFor(adapterExitGrace)
	if !completed {
		waitErr = fmt.Errorf("adapter %q did not exit after termination", s.adapter.Name)
	} else if waitErr == nil {
		waitErr = fmt.Errorf("adapter %q did not exit after stdin closed; terminated", s.adapter.Name)
	}
	return errors.Join(stdinErr, s.closeStdout(), terminateErr, waitErr)
}

func (s *Session) closeStdin() error {
	if s.stdin == nil || s.stdinClosed {
		return nil
	}
	s.stdinClosed = true
	if err := s.stdin.Close(); err != nil {
		return fmt.Errorf("close stdin for adapter %q: %w", s.adapter.Name, err)
	}
	return nil
}

func (s *Session) closeStdout() error {
	if s.stdout == nil {
		return nil
	}
	err := s.stdout.Close()
	s.stdout = nil
	if err != nil {
		return fmt.Errorf("close stdout for adapter %q: %w", s.adapter.Name, err)
	}
	return nil
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
	if s.done == nil {
		return nil
	}
	<-s.done
	if s.poisoned.Load() {
		return nil
	}
	return s.waitErr
}

func (s *Session) waitFor(timeout time.Duration) (bool, error) {
	if s.done == nil {
		return true, nil
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-s.done:
		if s.poisoned.Load() {
			return true, nil
		}
		return true, s.waitErr
	case <-timer.C:
		return false, nil
	}
}

func (s *Session) collectWait() {
	defer close(s.done)
	if err := s.cmd.Wait(); err != nil {
		message := strings.TrimSpace(s.stderr.String())
		if message == "" {
			message = err.Error()
		}
		s.waitErr = fmt.Errorf("adapter %q infrastructure failure: %s", s.adapter.Name, message)
	}
}

func resolveCommand(baseDir string, command []string) []string {
	resolved := append([]string(nil), command...)
	for i, part := range resolved {
		pathPart := filepath.FromSlash(part)
		if filepath.IsAbs(pathPart) {
			continue
		}
		if i == 0 {
			if isExplicitRelativePath(pathPart) || strings.ContainsRune(pathPart, filepath.Separator) {
				resolved[i] = filepath.Clean(filepath.Join(baseDir, pathPart))
			}
			continue
		}
		if isExplicitRelativePath(pathPart) {
			resolved[i] = filepath.Clean(filepath.Join(baseDir, pathPart))
		}
	}
	return resolved
}

func isExplicitRelativePath(value string) bool {
	separator := string(filepath.Separator)
	return value == "." ||
		value == ".." ||
		strings.HasPrefix(value, "."+separator) ||
		strings.HasPrefix(value, ".."+separator)
}
