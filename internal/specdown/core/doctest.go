package core

import (
	"encoding/json"
	"strings"
)

// DoctestCommand is a parsed command+expected pair from a doctest block.
type DoctestCommand struct {
	Command  string
	Expected string
}

// ParseDoctestSource parses a doctest block (lines starting with "$ ")
// into a sequence of command/expected pairs.
func ParseDoctestSource(source string) []DoctestCommand {
	lines := strings.Split(source, "\n")
	var steps []DoctestCommand
	var current *DoctestCommand
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
			current = &DoctestCommand{Command: strings.TrimPrefix(line, "$ ")}
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
	for _, line := range expectedLines {
		if line == "..." {
			return matchWildcardLines(strings.Split(actual, "\n"), expectedLines, 0, 0)
		}
	}
	return actual == expected
}

func matchWildcardLines(actual, expected []string, ai, ei int) bool {
	for ei < len(expected) {
		if expected[ei] != "..." {
			if ai >= len(actual) || actual[ai] != expected[ei] {
				return false
			}
			ai++
			ei++
			continue
		}
		return matchWildcardSkip(actual, expected, ai, ei)
	}
	return ai >= len(actual)
}

func matchWildcardSkip(actual, expected []string, ai, ei int) bool {
	for ei < len(expected) && expected[ei] == "..." {
		ei++
	}
	if ei >= len(expected) {
		return true
	}
	for ai <= len(actual) {
		if matchWildcardLines(actual, expected, ai, ei) {
			return true
		}
		ai++
	}
	return false
}

// ExecResponseToString extracts a string from a JSON-encoded exec response output.
func ExecResponseToString(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return string(raw)
	}
	return s
}
