package shelladapter

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

// ExecRawResponse is the raw JSON map returned by Exec, suitable for encoding as a line.
type ExecRawResponse map[string]interface{}

// Exec runs source via sh -c and returns a raw JSON response with "output" or "error" key.
func Exec(id int, source string) ExecRawResponse {
	cmd := exec.Command("sh", "-c", source)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return ExecRawResponse{"id": id, "error": message}
	}

	output := strings.TrimRight(stdout.String(), "\n")
	return ExecRawResponse{"id": id, "output": output}
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
	cmd := exec.Command("sh", "-c", command)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(errBuf.String())
		if message == "" {
			message = err.Error()
		}
		return "", message, false
	}
	return strings.TrimRight(outBuf.String(), "\n"), "", true
}

// ExecResponseToString delegates to core.ExecResponseToString.
func ExecResponseToString(raw json.RawMessage) string {
	return core.ExecResponseToString(raw)
}
