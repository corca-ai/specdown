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

func TestRunUsesSessionAdapterVariableBindingsAndFixtureRows(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "pocket-board.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := strings.Join([]string{
		"# Pocket Board",
		"",
		"## 변수 흐름",
		"",
		"```run:board -> $boardName",
		"create-board",
		"```",
		"",
		"### 생성한 보드 확인",
		"",
		"```verify:board",
		"board \"${boardName}\" should exist",
		"```",
		"",
		"### 표 기반 확인",
		"",
		"<!-- fixture:board-exists -->",
		"| board | exists |",
		"| --- | --- |",
		"| ${boardName} | 예 |",
		"| ${boardName}-archive | 예 |",
		"",
	}, "\n")
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
	if report.Summary.CasesPassed != 3 {
		t.Fatalf("unexpected summary %+v", report.Summary)
	}
	if got := report.Results[0].Cases[0].Bindings; len(got) != 1 || got[0].Name != "boardName" || got[0].Value != "board-1" {
		t.Fatalf("unexpected bindings %#v", got)
	}
	if got := report.Results[0].Cases[1].RenderedSource; got != "board \"board-1\" should exist" {
		t.Fatalf("unexpected rendered source %q", got)
	}
	if got := report.Results[0].Cases[2].RenderedCells; len(got) != 2 || got[0] != "board-1" || got[1] != "예" {
		t.Fatalf("unexpected rendered cells %#v", got)
	}
	if got := report.Results[0].Cases[3].Message; got != "expected board \"board-1-archive\" to exist; actual boards: [\"board-1\"]" {
		t.Fatalf("unexpected failure message %q", got)
	}
	if got := report.Results[0].Cases[3].Expected; got != "board \"board-1-archive\" exists" {
		t.Fatalf("unexpected expected diff %q", got)
	}
	if got := report.Results[0].Cases[3].Actual; got != "boards: [\"board-1\"]" {
		t.Fatalf("unexpected actual diff %q", got)
	}
}

func TestRunFailsWhenRuntimeBindingWasNotProducedForFixtureRow(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "pocket-board.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := strings.Join([]string{
		"# Pocket Board",
		"",
		"## 변수 흐름",
		"",
		"```run:board -> $boardName",
		"create-board board-1",
		"```",
		"",
		"### 표 기반 확인",
		"",
		"<!-- fixture:board-exists -->",
		"| board | exists |",
		"| --- | --- |",
		"| ${boardName} | 예 |",
		"",
	}, "\n")
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

func TestRunFailsWhenNoAdapterSupportsFixture(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "pocket-board.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := strings.Join([]string{
		"# Pocket Board",
		"",
		"## 표 기반 확인",
		"",
		"<!-- fixture:unknown -->",
		"| board | exists |",
		"| --- | --- |",
		"| demo | yes |",
		"",
	}, "\n")
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	_, err := Run(root, helperAdapterConfig())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no adapter supports fixture") {
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
				Fixtures: []string{"board-exists"},
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
	if item.Kind == "tableRow" {
		label = "fixture:" + item.Fixture
	}
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
		helperErr, ok := err.(*helperError)
		if !ok {
			helperErr = &helperError{message: err.Error()}
		}
		encoder.Encode(adapterprotocol.Response{
			Type:     "caseFailed",
			ID:       &item.ID,
			Label:    label,
			Message:  helperErr.message,
			Expected: helperErr.expected,
			Actual:   helperErr.actual,
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
	if item.Kind == "tableRow" {
		return executeHelperFixtureRow(state, item)
	}

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
		return nil, helperBoardExistsFailure(name, true)
	default:
		return nil, &helperError{message: "unsupported case " + item.Block}
	}
}

func executeHelperFixtureRow(state *helperState, item adapterprotocol.Case) ([]adapterprotocol.Binding, error) {
	if item.Fixture != "board-exists" {
		return nil, &helperError{message: "unsupported fixture " + strconvQuote(item.Fixture)}
	}
	if len(item.Columns) != len(item.Cells) {
		return nil, &helperError{message: "fixture row shape mismatch"}
	}

	values := make(map[string]string, len(item.Columns))
	for index, column := range item.Columns {
		values[column] = item.Cells[index]
	}
	name := values["board"]
	if _, exists := state.boards[name]; exists {
		return nil, nil
	}
	return nil, helperBoardExistsFailure(name, true)
}

type helperError struct {
	message  string
	expected string
	actual   string
}

func (e *helperError) Error() string {
	return e.message
}

func helperBoardExistsFailure(name string, shouldExist bool) *helperError {
	actual := "boards: [\"board-1\"]"
	if shouldExist {
		return &helperError{
			message:  "expected board " + strconvQuote(name) + " to exist; actual boards: [\"board-1\"]",
			expected: "board " + strconvQuote(name) + " exists",
			actual:   actual,
		}
	}
	return &helperError{
		message:  "expected board " + strconvQuote(name) + " not to exist; actual boards: [\"board-1\"]",
		expected: "board " + strconvQuote(name) + " absent",
		actual:   actual,
	}
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
