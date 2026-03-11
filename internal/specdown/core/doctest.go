package core

import "strings"

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
