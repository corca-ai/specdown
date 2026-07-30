package shelladapter

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/corca-ai/specdown/internal/specdown/core"
	"github.com/corca-ai/specdown/internal/specdown/subprocess"
)

// ExecRawResponse is the raw JSON map returned by Exec, suitable for encoding as a line.
type ExecRawResponse map[string]interface{}

// Exec runs source via sh -c and returns a raw JSON response with "output" or "error" key.
func Exec(id int, source, workdir string) ExecRawResponse {
	return ExecContext(context.Background(), id, source, workdir)
}

// ExecContext runs source via sh -c and terminates the process tree when ctx
// is canceled.
func ExecContext(ctx context.Context, id int, source, workdir string) ExecRawResponse {
	cmd := subprocess.CommandContext(ctx, "sh", "-c", source)
	if workdir != "" {
		cmd.Dir = workdir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		resp := ExecRawResponse{"id": id, "error": message}
		if cmd.ProcessState != nil {
			resp["exitCode"] = cmd.ProcessState.ExitCode()
		}
		if stderrStr := strings.TrimSpace(stderr.String()); stderrStr != "" {
			resp["stderr"] = stderrStr
		}
		return resp
	}

	output := strings.TrimRight(stdout.String(), "\n")
	resp := ExecRawResponse{"id": id, "output": output}
	if stderrStr := strings.TrimSpace(stderr.String()); stderrStr != "" {
		resp["stderr"] = stderrStr
	}
	return resp
}

// DoctestStep is an alias for core.DoctestCommand for backward compatibility.
type DoctestStep = core.DoctestCommand

// IsDoctestContent delegates to core.IsDoctestContent.
func IsDoctestContent(source string) bool {
	return core.IsDoctestContent(source)
}

// ParseDoctestSource delegates to core.ParseDoctestSource.
func ParseDoctestSource(source string) []DoctestStep {
	return core.ParseDoctestSource(source)
}

// MatchWithWildcard delegates to core.MatchWithWildcard.
func MatchWithWildcard(actual, expected string) bool {
	return core.MatchWithWildcard(actual, expected)
}

// StepStatus returns "passed" or "failed" for a doctest step.
func StepStatus(actual, expected string) string {
	if core.MatchWithWildcard(actual, expected) {
		return "passed"
	}
	return "failed"
}

// ExecForDoctest runs a single command and returns stdout/stderr.
func ExecForDoctest(command string) (stdout, errMsg string, ok bool) {
	return ExecForDoctestContext(context.Background(), command)
}

// ExecForDoctestContext runs one doctest command with caller cancellation.
func ExecForDoctestContext(ctx context.Context, command string) (stdout, errMsg string, ok bool) {
	cmd := subprocess.CommandContext(ctx, "sh", "-c", command)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(errBuf.String())
		if message == "" {
			message = err.Error()
		}
		return strings.TrimRight(outBuf.String(), "\n"), message, false
	}
	return strings.TrimRight(outBuf.String(), "\n"), "", true
}

// ExecResponseToString delegates to core.ExecResponseToString.
func ExecResponseToString(raw json.RawMessage) string {
	return core.ExecResponseToString(raw)
}
