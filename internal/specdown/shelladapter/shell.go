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

// ExecResponseToString delegates to core.ExecResponseToString.
func ExecResponseToString(raw json.RawMessage) string {
	return core.ExecResponseToString(raw)
}
