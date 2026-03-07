package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"specdown/internal/specdown/adapterprotocol"
)

var fixturesDir string

func main() {
	flag.StringVar(&fixturesDir, "fixtures-dir", "./fixtures", "Directory containing fixture scripts")
	flag.Parse()

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	encoder := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		var request adapterprotocol.Request
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			fmt.Fprintf(os.Stderr, "shell-adapter: decode request: %v\n", err)
			os.Exit(1)
		}

		switch request.Type {
		case "setup", "teardown":
			continue
		case "runCase":
			result := runCase(request)
			if err := encoder.Encode(result); err != nil {
				fmt.Fprintf(os.Stderr, "shell-adapter: encode response: %v\n", err)
				os.Exit(1)
			}
		default:
			fmt.Fprintf(os.Stderr, "shell-adapter: unknown request type %q\n", request.Type)
			os.Exit(1)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "shell-adapter: read stdin: %v\n", err)
		os.Exit(1)
	}
}

func runCase(request adapterprotocol.Request) adapterprotocol.Response {
	c := request.Case
	if c == nil {
		return adapterprotocol.Response{
			ID:      request.ID,
			Type:    "failed",
			Message: "missing case payload",
		}
	}

	switch c.Kind {
	case "code":
		return runCodeCase(request.ID, c)
	case "tableRow":
		return runTableRowCase(request.ID, c)
	default:
		return adapterprotocol.Response{
			ID:      request.ID,
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

	block := c.Block
	isVerify := strings.HasPrefix(block, "verify:")

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

	if isVerify {
		// For verify blocks, stdout is the assertion output.
		// Non-zero exit already handled above as failure.
		return adapterprotocol.Response{
			ID:   id,
			Type: "passed",
		}
	}

	// For run blocks, capture stdout into bindings if capture names are specified.
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

type doctestStep struct {
	Command  string
	Expected string
}

func parseDoctestSource(source string) []doctestStep {
	lines := strings.Split(source, "\n")
	var steps []doctestStep
	var current *doctestStep
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
			current = &doctestStep{Command: strings.TrimPrefix(line, "$ ")}
			expectedLines = nil
		} else if current != nil {
			expectedLines = append(expectedLines, line)
		}
	}
	flush()
	return steps
}

func runDoctestCase(id int, c *adapterprotocol.Case) adapterprotocol.Response {
	steps := parseDoctestSource(c.Source)
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
		if actual != expected {
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
	if actual == expected {
		return "passed"
	}
	return "failed"
}

func runTableRowCase(id int, c *adapterprotocol.Case) adapterprotocol.Response {
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

	return adapterprotocol.Response{
		ID:   id,
		Type: "passed",
	}
}
