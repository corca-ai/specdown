package core

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type boardRuntime struct {
	boards map[string]struct{}
}

func newBoardRuntime() *boardRuntime {
	return &boardRuntime{
		boards: make(map[string]struct{}),
	}
}

func (r *boardRuntime) Run(source string) error {
	lines := strings.Split(source, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if err := r.executeCommand(trimmed); err != nil {
			return err
		}
	}
	return nil
}

func (r *boardRuntime) Verify(source string) error {
	lines := strings.Split(source, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if err := r.verifyAssertion(trimmed); err != nil {
			return err
		}
	}
	return nil
}

func (r *boardRuntime) executeCommand(line string) error {
	const prefix = "create-board"

	if !strings.HasPrefix(line, prefix) {
		return fmt.Errorf("unsupported board command %q", line)
	}

	name, err := parseSingleArg(strings.TrimSpace(strings.TrimPrefix(line, prefix)))
	if err != nil {
		return err
	}
	if _, exists := r.boards[name]; exists {
		return fmt.Errorf("board %q already exists", name)
	}

	r.boards[name] = struct{}{}
	return nil
}

func (r *boardRuntime) verifyAssertion(line string) error {
	assertion, err := parseBoardAssertion(line)
	if err != nil {
		return err
	}

	_, exists := r.boards[assertion.Name]
	if assertion.ShouldExist && exists {
		return nil
	}
	if !assertion.ShouldExist && !exists {
		return nil
	}

	if assertion.ShouldExist {
		return fmt.Errorf("expected board %q to exist; actual boards: %s", assertion.Name, r.boardList())
	}
	return fmt.Errorf("expected board %q not to exist; actual boards: %s", assertion.Name, r.boardList())
}

func parseSingleArg(arg string) (string, error) {
	if arg == "" {
		return "", fmt.Errorf("missing board name")
	}

	if strings.HasPrefix(arg, "\"") {
		value, err := strconv.Unquote(arg)
		if err != nil {
			return "", fmt.Errorf("invalid quoted argument %q", arg)
		}
		if value == "" {
			return "", fmt.Errorf("board name must not be empty")
		}
		return value, nil
	}

	if strings.ContainsAny(arg, " \t") {
		return "", fmt.Errorf("board name must be a single token or quoted string")
	}
	return arg, nil
}

type boardAssertion struct {
	Name        string
	ShouldExist bool
}

func parseBoardAssertion(line string) (boardAssertion, error) {
	const prefix = "board"

	if !strings.HasPrefix(line, prefix) {
		return boardAssertion{}, fmt.Errorf("unsupported board assertion %q", line)
	}

	name, rest, err := parseLeadingArg(strings.TrimSpace(strings.TrimPrefix(line, prefix)))
	if err != nil {
		return boardAssertion{}, err
	}

	switch rest {
	case "should exist":
		return boardAssertion{Name: name, ShouldExist: true}, nil
	case "should not exist":
		return boardAssertion{Name: name, ShouldExist: false}, nil
	default:
		return boardAssertion{}, fmt.Errorf("unsupported board assertion %q", line)
	}
}

func parseLeadingArg(input string) (string, string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", fmt.Errorf("missing board name")
	}

	if strings.HasPrefix(input, "\"") {
		end := findClosingQuote(input)
		if end == -1 {
			return "", "", fmt.Errorf("invalid quoted argument %q", input)
		}

		value, err := strconv.Unquote(input[:end+1])
		if err != nil {
			return "", "", fmt.Errorf("invalid quoted argument %q", input[:end+1])
		}
		if value == "" {
			return "", "", fmt.Errorf("board name must not be empty")
		}
		return value, strings.TrimSpace(input[end+1:]), nil
	}

	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", "", fmt.Errorf("missing board name")
	}
	name := parts[0]
	rest := strings.TrimSpace(input[len(name):])
	return name, rest, nil
}

func findClosingQuote(input string) int {
	escaped := false
	for i := 1; i < len(input); i++ {
		switch {
		case escaped:
			escaped = false
		case input[i] == '\\':
			escaped = true
		case input[i] == '"':
			return i
		}
	}
	return -1
}

func (r *boardRuntime) boardList() string {
	names := make([]string, 0, len(r.boards))
	for name := range r.boards {
		names = append(names, strconv.Quote(name))
	}
	sort.Strings(names)
	return "[" + strings.Join(names, ", ") + "]"
}
