package shelladapter

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/corca-ai/specdown/internal/specdown/adapterprotocol"
)

// RunCase executes a single shell adapter case and returns the response.
// fixturesDir is the directory containing fixture scripts (used for tableRow cases).
func RunCase(id int, c *adapterprotocol.Case, fixturesDir string) adapterprotocol.Response {
	if c == nil {
		return adapterprotocol.Response{
			ID:      id,
			Type:    "failed",
			Message: "missing case payload",
		}
	}

	switch c.Kind {
	case "code":
		return runCodeCase(id, c)
	case "tableRow":
		return runTableRowCase(id, c, fixturesDir)
	default:
		return adapterprotocol.Response{
			ID:      id,
			Type:    "failed",
			Message: fmt.Sprintf("unsupported case kind %q", c.Kind),
		}
	}
}

func runCodeCase(id int, c *adapterprotocol.Case) adapterprotocol.Response {
	if strings.HasPrefix(c.Block, "doctest:") {
		return runDoctestCase(id, c)
	}

	source := c.Source

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
		return adapterprotocol.Response{
			ID:      id,
			Type:    "failed",
			Message: message,
		}
	}

	// Capture stdout into bindings if capture names are specified.
	var bindings []adapterprotocol.Binding
	if len(c.CaptureNames) > 0 {
		output := strings.TrimRight(stdout.String(), "\n")
		lines := strings.SplitN(output, "\n", len(c.CaptureNames))
		for i, name := range c.CaptureNames {
			value := ""
			if i < len(lines) {
				value = lines[i]
			}
			bindings = append(bindings, adapterprotocol.Binding{
				Name:  name,
				Value: value,
			})
		}
	}

	return adapterprotocol.Response{
		ID:       id,
		Type:     "passed",
		Bindings: bindings,
	}
}

// DoctestStep represents a single command/expected-output pair in a doctest block.
type DoctestStep struct {
	Command  string
	Expected string
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

func runDoctestCase(id int, c *adapterprotocol.Case) adapterprotocol.Response {
	steps := ParseDoctestSource(c.Source)
	if len(steps) == 0 {
		return adapterprotocol.Response{
			ID:      id,
			Type:    "failed",
			Message: "doctest block contains no $ commands",
		}
	}

	var resultSteps []adapterprotocol.DoctestStep
	for _, step := range steps {
		cmd := exec.Command("sh", "-c", step.Command)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			message := strings.TrimSpace(stderr.String())
			if message == "" {
				message = err.Error()
			}
			resultSteps = append(resultSteps, adapterprotocol.DoctestStep{
				Command:  step.Command,
				Expected: step.Expected,
				Actual:   message,
				Status:   "failed",
			})
			return adapterprotocol.Response{
				ID:    id,
				Type:  "failed",
				Steps: resultSteps,
			}
		}

		actual := strings.TrimRight(stdout.String(), "\n")
		expected := step.Expected
		resultSteps = append(resultSteps, adapterprotocol.DoctestStep{
			Command:  step.Command,
			Expected: expected,
			Actual:   actual,
			Status:   stepStatus(actual, expected),
		})
		if !MatchWithWildcard(actual, expected) {
			return adapterprotocol.Response{
				ID:    id,
				Type:  "failed",
				Steps: resultSteps,
			}
		}
	}

	return adapterprotocol.Response{
		ID:    id,
		Type:  "passed",
		Steps: resultSteps,
	}
}

func stepStatus(actual, expected string) string {
	if MatchWithWildcard(actual, expected) {
		return "passed"
	}
	return "failed"
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

func runTableRowCase(id int, c *adapterprotocol.Case, fixturesDir string) adapterprotocol.Response {
	// Build environment from fixture params and cells.
	env := os.Environ()
	if c.FixtureParams != nil {
		for k, v := range c.FixtureParams {
			env = append(env, fmt.Sprintf("FIXTURE_PARAM_%s=%s", strings.ToUpper(k), v))
		}
	}
	for i, col := range c.Columns {
		value := ""
		if i < len(c.Cells) {
			value = c.Cells[i]
		}
		env = append(env, fmt.Sprintf("COL_%s=%s", strings.ToUpper(strings.ReplaceAll(col, "-", "_")), value))
	}
	env = append(env, fmt.Sprintf("FIXTURE=%s", c.Fixture))

	script := filepath.Join(fixturesDir, c.Fixture+".sh")
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
		return adapterprotocol.Response{
			ID:      id,
			Type:    "failed",
			Message: message,
		}
	}

	resp := adapterprotocol.Response{
		ID:   id,
		Type: "passed",
	}
	if actual := strings.TrimSpace(stdout.String()); actual != "" {
		resp.Actual = actual
	}
	return resp
}
