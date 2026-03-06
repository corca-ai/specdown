package engine

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"specdown/internal/specdown/adapterprotocol"
	"specdown/internal/specdown/config"
)

func TestRunUsesSessionAdapterAndVariableBindings(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "pocket-board.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := "# Pocket Board\n\n## Variable Flow\n\n```run:board -> $boardName\ncreate-board\n```\n\n### Verify Board\n\n```verify:board\nboard \"${boardName}\" should exist\n```\n"
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	report, err := Run(root, helperAdapterConfig())
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if report.Summary.SpecsPassed != 1 {
		t.Fatalf("unexpected summary %+v", report.Summary)
	}
	if report.Summary.CasesPassed != 2 {
		t.Fatalf("unexpected summary %+v", report.Summary)
	}
	if got := report.Results[0].Cases[0].Bindings; len(got) != 1 || got[0].Name != "boardName" || got[0].Value != "board-1" {
		t.Fatalf("unexpected bindings %#v", got)
	}
	if got := report.Results[0].Cases[1].RenderedSource; got != "board \"board-1\" should exist" {
		t.Fatalf("unexpected rendered source %q", got)
	}
}

func TestRunFailsWhenAdapterReportsVerificationFailure(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "pocket-board.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := "# Pocket Board\n\n## Variable Flow\n\n```run:board -> $boardName\ncreate-board\n```\n\n### Verify Missing Board\n\n```verify:board\nboard \"${boardName}-archive\" should exist\n```\n"
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	report, err := Run(root, helperAdapterConfig())
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if report.Summary.SpecsFailed != 1 || report.Summary.CasesFailed != 1 {
		t.Fatalf("unexpected summary %+v", report.Summary)
	}
	if got := report.Results[0].Cases[1].Message; got != "expected board \"board-1-archive\" to exist; actual boards: [\"board-1\"]" {
		t.Fatalf("unexpected failure message %q", got)
	}
}

func TestRunFailsWhenRuntimeBindingWasNotProduced(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "pocket-board.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := "# Pocket Board\n\n## Variable Flow\n\n```run:board -> $boardName\ncreate-board board-1\n```\n\n### Verify Board\n\n```verify:board\nboard \"${boardName}\" should exist\n```\n"
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	report, err := Run(root, helperNoBindingConfig())
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if report.Summary.CasesFailed != 1 {
		t.Fatalf("unexpected summary %+v", report.Summary)
	}
	if got := report.Results[0].Cases[1].Message; got != `missing runtime binding for "boardName"` {
		t.Fatalf("unexpected message %q", got)
	}
}

func TestRunFailsWhenNoAdapterSupportsBlock(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "pocket-board.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := "# Pocket Board\n\n## Shell\n\n```run:shell\necho hi\n```\n"
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	_, err := Run(root, helperAdapterConfig())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no adapter supports block") {
		t.Fatalf("unexpected error %v", err)
	}
}

func helperAdapterConfig() config.Config {
	executable, err := os.Executable()
	if err != nil {
		panic(err)
	}

	return config.Config{
		Include: []string{"specs/**/*.spec.md"},
		Adapters: []config.AdapterConfig{
			{
				Name:     "helper-board",
				Command:  []string{executable, "-test.run=TestHelperAdapterProcess", "--", "board"},
				Protocol: adapterprotocol.Version,
			},
		},
	}
}

func helperNoBindingConfig() config.Config {
	executable, err := os.Executable()
	if err != nil {
		panic(err)
	}

	return config.Config{
		Include: []string{"specs/**/*.spec.md"},
		Adapters: []config.AdapterConfig{
			{
				Name:     "helper-board",
				Command:  []string{executable, "-test.run=TestHelperAdapterProcess", "--", "board-no-bindings"},
				Protocol: adapterprotocol.Version,
			},
		},
	}
}

func TestHelperAdapterProcess(t *testing.T) {
	if len(os.Args) < 2 {
		return
	}

	mode := os.Args[len(os.Args)-1]
	if mode != "board" && mode != "board-no-bindings" {
		return
	}

	state := helperState{
		boards:      make(map[string]struct{}),
		nextBoardID: 1,
		emitBinding: mode == "board",
	}

	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var request adapterprotocol.Request
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			os.Exit(2)
		}

		switch request.Type {
		case "describe":
			encoder.Encode(adapterprotocol.Response{
				Type:     "capabilities",
				Blocks:   []string{"run:board", "verify:board"},
				Fixtures: []string{},
			})
		case "runCase":
			if request.Case == nil {
				os.Exit(3)
			}
			runHelperCase(encoder, &state, *request.Case)
		default:
			os.Exit(4)
		}
	}
	os.Exit(0)
}

type helperState struct {
	boards      map[string]struct{}
	nextBoardID int
	emitBinding bool
}

func runHelperCase(encoder *json.Encoder, state *helperState, item adapterprotocol.Case) {
	label := item.Block
	if len(item.ID.HeadingPath) > 0 {
		label += " @ " + item.ID.HeadingPath[len(item.ID.HeadingPath)-1]
	}
	encoder.Encode(adapterprotocol.Response{
		Type:  "caseStarted",
		ID:    &item.ID,
		Label: label,
	})

	bindings, err := executeHelperBoardCase(state, item)
	if err != nil {
		encoder.Encode(adapterprotocol.Response{
			Type:    "caseFailed",
			ID:      &item.ID,
			Label:   label,
			Message: err.Error(),
		})
		return
	}

	encoder.Encode(adapterprotocol.Response{
		Type:     "casePassed",
		ID:       &item.ID,
		Label:    label,
		Bindings: bindings,
	})
}

func executeHelperBoardCase(state *helperState, item adapterprotocol.Case) ([]adapterprotocol.Binding, error) {
	switch item.Block {
	case "run:board":
		var name string
		var err error
		if strings.TrimSpace(item.Source) == "create-board" {
			name = "board-" + strconv.Itoa(state.nextBoardID)
			state.nextBoardID++
		} else {
			name, err = parseHelperCommandArg(strings.TrimSpace(strings.TrimPrefix(item.Source, "create-board")))
			if err != nil {
				return nil, err
			}
		}
		if _, exists := state.boards[name]; exists {
			return nil, &helperError{message: "board " + strconvQuote(name) + " already exists"}
		}
		state.boards[name] = struct{}{}
		if !state.emitBinding || len(item.CaptureNames) == 0 {
			return nil, nil
		}
		return []adapterprotocol.Binding{{
			Name:  item.CaptureNames[0],
			Value: name,
		}}, nil
	case "verify:board":
		name, err := parseHelperVerifySource(item.Source)
		if err != nil {
			return nil, err
		}
		if _, exists := state.boards[name]; exists {
			return nil, nil
		}
		return nil, &helperError{message: "expected board " + strconvQuote(name) + " to exist; actual boards: [\"board-1\"]"}
	default:
		return nil, &helperError{message: "unsupported case " + item.Block}
	}
}

type helperError struct {
	message string
}

func (e *helperError) Error() string {
	return e.message
}

func strconvQuote(value string) string {
	return strconv.Quote(value)
}

func parseHelperCommandArg(input string) (string, error) {
	input = strings.TrimSpace(input)
	value, err := strconv.Unquote(input)
	if err == nil {
		return value, nil
	}
	if strings.ContainsAny(input, " \t") {
		return "", &helperError{message: "invalid command source " + strconvQuote(input)}
	}
	return input, nil
}

func parseHelperVerifySource(source string) (string, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(source, "board"))
	if !strings.HasSuffix(trimmed, "should exist") {
		return "", &helperError{message: "invalid verify source " + strconvQuote(source)}
	}
	namePart := strings.TrimSpace(strings.TrimSuffix(trimmed, "should exist"))
	return parseHelperCommandArg(namePart)
}
