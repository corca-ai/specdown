package core

import (
	"fmt"
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

func (r *boardRuntime) Execute(source string) error {
	lines := strings.Split(source, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if err := r.executeLine(trimmed); err != nil {
			return err
		}
	}
	return nil
}

func (r *boardRuntime) executeLine(line string) error {
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
