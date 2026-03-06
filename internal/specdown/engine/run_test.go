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

func TestRunSupportsBoardAndCardLifecycleFixtures(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "pocket-board.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := strings.Join([]string{
		"# Pocket Board",
		"",
		"## 보드 생성",
		"",
		"```run:board -> $boardName",
		"create-board",
		"```",
		"",
		"### 생성한 보드는 즉시 존재해야 한다",
		"",
		"```verify:board",
		"board \"${boardName}\" should exist",
		"```",
		"",
		"### 생성하지 않은 보드는 존재하지 않아야 한다",
		"",
		"```verify:board",
		"board \"${boardName}-archive\" should not exist",
		"```",
		"",
		"### 카드 수명 주기",
		"",
		"```run:board -> $cardId",
		"create-card \"${boardName}\" \"명세 쓰기\"",
		"```",
		"",
		"#### 생성한 카드는 보드에 속해야 한다",
		"",
		"<!-- fixture:card-exists -->",
		"| board | card | exists |",
		"| --- | --- | --- |",
		"| ${boardName} | ${cardId} | 예 |",
		"",
		"#### 새 카드는 todo에 있어야 한다",
		"",
		"<!-- fixture:card-column -->",
		"| board | card | column |",
		"| --- | --- | --- |",
		"| ${boardName} | ${cardId} | todo |",
		"",
		"#### 카드는 다른 컬럼으로 이동할 수 있다",
		"",
		"```run:board",
		"move-card \"${boardName}\" \"${cardId}\" doing",
		"```",
		"",
		"##### 이동한 카드는 doing에 있어야 한다",
		"",
		"<!-- fixture:card-column -->",
		"| board | card | column |",
		"| --- | --- | --- |",
		"| ${boardName} | ${cardId} | doing |",
		"",
	}, "\n")
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	report, err := Run(root, helperAdapterConfig())
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if report.Summary.SpecsPassed != 1 || report.Summary.CasesFailed != 0 {
		t.Fatalf("unexpected summary %+v", report.Summary)
	}
	if report.Summary.CasesPassed != 8 {
		t.Fatalf("unexpected summary %+v", report.Summary)
	}
	if got := report.Results[0].Cases[0].Bindings; len(got) != 1 || got[0].Name != "boardName" || got[0].Value != "board-1" {
		t.Fatalf("unexpected board binding %#v", got)
	}
	if got := report.Results[0].Cases[3].Bindings; len(got) != 1 || got[0].Name != "cardId" || got[0].Value != "card-1" {
		t.Fatalf("unexpected card binding %#v", got)
	}
	if got := report.Results[0].Cases[4].RenderedCells; len(got) != 3 || got[0] != "board-1" || got[1] != "card-1" || got[2] != "예" {
		t.Fatalf("unexpected card exists row %#v", got)
	}
	if got := report.Results[0].Cases[7].RenderedCells; len(got) != 3 || got[2] != "doing" {
		t.Fatalf("unexpected moved card row %#v", got)
	}
}

func TestRunFailsWhenCardColumnFixtureMismatches(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "pocket-board.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := strings.Join([]string{
		"# Pocket Board",
		"",
		"## 보드 생성",
		"",
		"```run:board -> $boardName",
		"create-board",
		"```",
		"",
		"### 카드 수명 주기",
		"",
		"```run:board -> $cardId",
		"create-card \"${boardName}\" \"명세 쓰기\"",
		"```",
		"",
		"#### 카드 컬럼 확인",
		"",
		"<!-- fixture:card-column -->",
		"| board | card | column |",
		"| --- | --- | --- |",
		"| ${boardName} | ${cardId} | doing |",
		"",
	}, "\n")
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	report, err := Run(root, helperAdapterConfig())
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if report.Summary.CasesFailed != 1 {
		t.Fatalf("unexpected summary %+v", report.Summary)
	}
	if got := report.Results[0].Cases[2].Message; got != "expected card \"card-1\" in board \"board-1\" to be in column \"doing\"; actual column: \"todo\"" {
		t.Fatalf("unexpected failure message %q", got)
	}
	if got := report.Results[0].Cases[2].Expected; got != "card \"card-1\" in board \"board-1\" at column \"doing\"" {
		t.Fatalf("unexpected expected diff %q", got)
	}
	if got := report.Results[0].Cases[2].Actual; got != "column: \"todo\"" {
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
		"## 보드 생성",
		"",
		"```run:board -> $boardName",
		"create-board board-1",
		"```",
		"",
		"### 보드 존재 규칙",
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
		boards:      make(map[string]*helperBoard),
		nextBoardID: 1,
		nextCardID:  1,
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
				Fixtures: []string{"board-exists", "card-exists", "card-column"},
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
	boards      map[string]*helperBoard
	nextBoardID int
	nextCardID  int
	emitBinding bool
}

type helperBoard struct {
	cards map[string]*helperCard
}

type helperCard struct {
	column string
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

	bindings, err := executeHelperCase(state, item)
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

func executeHelperCase(state *helperState, item adapterprotocol.Case) ([]adapterprotocol.Binding, error) {
	if item.Kind == "tableRow" {
		return executeHelperFixtureRow(state, item)
	}

	parts, err := parseHelperCommand(item.Source)
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		return nil, &helperError{message: "empty command"}
	}

	switch parts[0] {
	case "create-board":
		return executeHelperCreateBoard(state, item, parts)
	case "create-card":
		return executeHelperCreateCard(state, item, parts)
	case "move-card":
		return executeHelperMoveCard(state, parts)
	case "board":
		return executeHelperVerifyBoard(state, item.Source)
	default:
		return nil, &helperError{message: "unsupported case " + item.Block}
	}
}

func executeHelperCreateBoard(state *helperState, item adapterprotocol.Case, parts []string) ([]adapterprotocol.Binding, error) {
	var name string
	if len(parts) == 1 {
		name = "board-" + strconv.Itoa(state.nextBoardID)
		state.nextBoardID++
	} else if len(parts) == 2 {
		name = parts[1]
	} else {
		return nil, &helperError{message: "unsupported board command " + strconvQuote(item.Source)}
	}
	if _, exists := state.boards[name]; exists {
		return nil, &helperError{message: "board " + strconvQuote(name) + " already exists"}
	}
	state.boards[name] = &helperBoard{cards: make(map[string]*helperCard)}
	if !state.emitBinding || len(item.CaptureNames) == 0 {
		return nil, nil
	}
	return []adapterprotocol.Binding{{Name: item.CaptureNames[0], Value: name}}, nil
}

func executeHelperCreateCard(state *helperState, item adapterprotocol.Case, parts []string) ([]adapterprotocol.Binding, error) {
	if len(parts) != 3 {
		return nil, &helperError{message: "unsupported board command " + strconvQuote(item.Source)}
	}
	board, err := helperBoardFor(state, parts[1])
	if err != nil {
		return nil, err
	}
	cardID := "card-" + strconv.Itoa(state.nextCardID)
	state.nextCardID++
	board.cards[cardID] = &helperCard{column: "todo"}
	if !state.emitBinding || len(item.CaptureNames) == 0 {
		return nil, nil
	}
	return []adapterprotocol.Binding{{Name: item.CaptureNames[0], Value: cardID}}, nil
}

func executeHelperMoveCard(state *helperState, parts []string) ([]adapterprotocol.Binding, error) {
	if len(parts) != 4 {
		return nil, &helperError{message: "unsupported board command"}
	}
	board, err := helperBoardFor(state, parts[1])
	if err != nil {
		return nil, err
	}
	card := board.cards[parts[2]]
	if card == nil {
		return nil, helperCardExistsFailure(parts[1], parts[2], true)
	}
	card.column = parts[3]
	return nil, nil
}

func executeHelperVerifyBoard(state *helperState, source string) ([]adapterprotocol.Binding, error) {
	name, shouldExist, err := parseHelperVerifySource(source)
	if err != nil {
		return nil, err
	}
	_, exists := state.boards[name]
	if shouldExist == exists {
		return nil, nil
	}
	return nil, helperBoardExistsFailure(name, shouldExist)
}

func executeHelperFixtureRow(state *helperState, item adapterprotocol.Case) ([]adapterprotocol.Binding, error) {
	if len(item.Columns) != len(item.Cells) {
		return nil, &helperError{message: "fixture row shape mismatch"}
	}

	values := make(map[string]string, len(item.Columns))
	for index, column := range item.Columns {
		values[column] = item.Cells[index]
	}

	switch item.Fixture {
	case "board-exists":
		name := values["board"]
		shouldExist := parseHelperExists(values["exists"])
		_, exists := state.boards[name]
		if shouldExist == exists {
			return nil, nil
		}
		return nil, helperBoardExistsFailure(name, shouldExist)
	case "card-exists":
		boardName := values["board"]
		cardName := values["card"]
		shouldExist := parseHelperExists(values["exists"])
		board, err := helperBoardFor(state, boardName)
		if err != nil {
			return nil, err
		}
		_, exists := board.cards[cardName]
		if shouldExist == exists {
			return nil, nil
		}
		return nil, helperCardExistsFailure(boardName, cardName, shouldExist)
	case "card-column":
		boardName := values["board"]
		cardName := values["card"]
		expectedColumn := values["column"]
		board, err := helperBoardFor(state, boardName)
		if err != nil {
			return nil, err
		}
		card := board.cards[cardName]
		if card == nil {
			return nil, helperCardExistsFailure(boardName, cardName, true)
		}
		if card.column == expectedColumn {
			return nil, nil
		}
		return nil, &helperError{
			message:  "expected card " + strconvQuote(cardName) + " in board " + strconvQuote(boardName) + " to be in column " + strconvQuote(expectedColumn) + "; actual column: " + strconvQuote(card.column),
			expected: "card " + strconvQuote(cardName) + " in board " + strconvQuote(boardName) + " at column " + strconvQuote(expectedColumn),
			actual:   "column: " + strconvQuote(card.column),
		}
	default:
		return nil, &helperError{message: "unsupported fixture " + strconvQuote(item.Fixture)}
	}
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

func helperCardExistsFailure(boardName string, cardName string, shouldExist bool) *helperError {
	actual := "cards: [\"card-1\"]"
	if shouldExist {
		return &helperError{
			message:  "expected card " + strconvQuote(cardName) + " to exist in board " + strconvQuote(boardName) + "; actual cards: [\"card-1\"]",
			expected: "card " + strconvQuote(cardName) + " in board " + strconvQuote(boardName) + " exists",
			actual:   actual,
		}
	}
	return &helperError{
		message:  "expected card " + strconvQuote(cardName) + " not to exist in board " + strconvQuote(boardName) + "; actual cards: [\"card-1\"]",
		expected: "card " + strconvQuote(cardName) + " in board " + strconvQuote(boardName) + " absent",
		actual:   actual,
	}
}

func helperBoardFor(state *helperState, boardName string) (*helperBoard, error) {
	board := state.boards[boardName]
	if board == nil {
		return nil, helperBoardExistsFailure(boardName, true)
	}
	return board, nil
}

func strconvQuote(value string) string {
	return strconv.Quote(value)
}

func parseHelperCommand(source string) ([]string, error) {
	var (
		parts   []string
		current strings.Builder
		inQuote bool
	)
	for _, r := range source {
		switch r {
		case '"':
			inQuote = !inQuote
		case ' ', '\t':
			if inQuote {
				current.WriteRune(r)
				continue
			}
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if inQuote {
		return nil, &helperError{message: "invalid command source " + strconvQuote(source)}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts, nil
}

func parseHelperVerifySource(source string) (string, bool, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(source, "board"))
	if strings.HasSuffix(trimmed, "should exist") {
		namePart := strings.TrimSpace(strings.TrimSuffix(trimmed, "should exist"))
		name, err := parseHelperCommandArg(namePart)
		return name, true, err
	}
	if strings.HasSuffix(trimmed, "should not exist") {
		namePart := strings.TrimSpace(strings.TrimSuffix(trimmed, "should not exist"))
		name, err := parseHelperCommandArg(namePart)
		return name, false, err
	}
	return "", false, &helperError{message: "invalid verify source " + strconvQuote(source)}
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

func parseHelperExists(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "예", "yes", "true":
		return true
	default:
		return false
	}
}
