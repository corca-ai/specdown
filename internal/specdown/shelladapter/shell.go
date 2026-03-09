package shelladapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/corca-ai/specdown/internal/specdown/adapterprotocol"
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

// Assert runs a check script and returns an AssertResponse.
func Assert(id int, req *adapterprotocol.AssertRequest, checksDir string) adapterprotocol.AssertResponse {
	if req == nil {
		return adapterprotocol.AssertResponse{
			ID:      id,
			Type:    "failed",
			Message: "missing assert request",
		}
	}

	// Build environment from check params and cells.
	env := os.Environ()
	if req.CheckParams != nil {
		for k, v := range req.CheckParams {
			env = append(env, fmt.Sprintf("CHECK_PARAM_%s=%s", strings.ToUpper(k), v))
		}
	}
	for i, col := range req.Columns {
		value := ""
		if i < len(req.Cells) {
			value = req.Cells[i]
		}
		env = append(env, fmt.Sprintf("COL_%s=%s", strings.ToUpper(strings.ReplaceAll(col, "-", "_")), value))
	}
	env = append(env, fmt.Sprintf("CHECK=%s", req.Check))

	script := filepath.Join(checksDir, req.Check+".sh")
	cmd := exec.Command("sh", script)
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message == "" {
			message = err.Error()
		}
		return adapterprotocol.AssertResponse{
			ID:      id,
			Type:    "failed",
			Message: message,
		}
	}

	resp := adapterprotocol.AssertResponse{
		ID:   id,
		Type: "passed",
	}
	if actual := strings.TrimSpace(stdout.String()); actual != "" {
		resp.Actual = actual
	}
	return resp
}

// DoctestStep represents a single command/expected-output pair in a doctest block.
type DoctestStep struct {
	Command  string
	Expected string
}

// IsDoctestContent returns true if the source starts with a `$ ` line.
func IsDoctestContent(source string) bool {
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		return strings.HasPrefix(line, "$ ")
	}
	return false
}

func ParseDoctestSource(source string) []DoctestStep {
	lines := strings.Split(source, "\n")
	var steps []DoctestStep
	var current *DoctestStep
	var expectedLines []string

	flush := func() {
		if current != nil {
			current.Expected = strings.Join(expectedLines, "\n")
			steps = append(steps, *current)
			current = nil
			expectedLines = nil
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "$ ") {
			flush()
			current = &DoctestStep{Command: strings.TrimPrefix(line, "$ ")}
			expectedLines = nil
		} else if current != nil {
			expectedLines = append(expectedLines, line)
		}
	}
	flush()
	return steps
}

// MatchWithWildcard checks if actual matches expected, where a line
// containing exactly "..." in expected matches zero or more lines in actual.
func MatchWithWildcard(actual, expected string) bool {
	expectedLines := strings.Split(expected, "\n")
	if !hasWildcardLine(expectedLines) {
		return actual == expected
	}
	actualLines := strings.Split(actual, "\n")
	return matchLines(actualLines, expectedLines, 0, 0)
}

func hasWildcardLine(lines []string) bool {
	for _, line := range lines {
		if line == "..." {
			return true
		}
	}
	return false
}

func skipWildcards(expected []string, ei int) int {
	for ei < len(expected) && expected[ei] == "..." {
		ei++
	}
	return ei
}

func matchLines(actual, expected []string, ai, ei int) bool {
	for ei < len(expected) {
		if expected[ei] != "..." {
			if ai >= len(actual) || actual[ai] != expected[ei] {
				return false
			}
			ai++
			ei++
			continue
		}
		ei = skipWildcards(expected, ei)
		if ei >= len(expected) {
			return true
		}
		for ai <= len(actual) {
			if matchLines(actual, expected, ai, ei) {
				return true
			}
			ai++
		}
		return false
	}
	return ai >= len(actual)
}

// StepStatus returns "passed" or "failed" for a doctest step.
func StepStatus(actual, expected string) string {
	if MatchWithWildcard(actual, expected) {
		return "passed"
	}
	return "failed"
}

// ExecForDoctest runs a single command and returns stdout/stderr.
func ExecForDoctest(command string) (stdout string, errMsg string, ok bool) {
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

// ExecResponseToString extracts a string from an ExecResponse output field.
func ExecResponseToString(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		// Not a string — return raw JSON
		return string(raw)
	}
	return s
}
