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

func TestRunUsesExternalAdapter(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "pocket-board.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := "# Pocket Board\n\n## Create Board\n\n```run:board\ncreate-board \"demo\"\n```\n\n## Verify Board\n\n```verify:board\nboard \"demo\" should exist\n```\n"
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	cfg := helperAdapterConfig()
	report, err := Run(filepath.Join(root, "specs"), cfg, root)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if report.Summary.SpecsPassed != 1 {
		t.Fatalf("unexpected summary %+v", report.Summary)
	}
	if report.Summary.CasesPassed != 2 {
		t.Fatalf("unexpected summary %+v", report.Summary)
	}
	if report.Results[0].Cases[1].Info != "verify:board" {
		t.Fatalf("unexpected case %#v", report.Results[0].Cases[1])
	}
}

func TestRunFailsWhenAdapterReportsVerificationFailure(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "specs", "pocket-board.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := "# Pocket Board\n\n## Create Board\n\n```run:board\ncreate-board \"demo\"\n```\n\n## Verify Missing Board\n\n```verify:board\nboard \"archive\" should exist\n```\n"
	if err := os.WriteFile(specPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	cfg := helperAdapterConfig()
	report, err := Run(filepath.Join(root, "specs"), cfg, root)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if report.Summary.SpecsFailed != 1 || report.Summary.CasesFailed != 1 {
		t.Fatalf("unexpected summary %+v", report.Summary)
	}
	if got := report.Results[0].Cases[1].Message; got != "expected board \"archive\" to exist; actual boards: [\"demo\"]" {
		t.Fatalf("unexpected failure message %q", got)
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

	cfg := helperAdapterConfig()
	_, err := Run(filepath.Join(root, "specs"), cfg, root)
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
		Adapters: []config.AdapterConfig{
			{
				Name:     "helper-board",
				Command:  []string{executable, "-test.run=TestHelperAdapterProcess", "--", "board"},
				Protocol: adapterprotocol.Version,
			},
		},
	}
}

func TestHelperAdapterProcess(t *testing.T) {
	if len(os.Args) < 2 || os.Args[len(os.Args)-1] != "board" {
		return
	}

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		os.Exit(0)
	}

	var request adapterprotocol.Request
	if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
		os.Exit(2)
	}

	encoder := json.NewEncoder(os.Stdout)
	switch request.Type {
	case "describe":
		encoder.Encode(adapterprotocol.Response{
			Type:     "capabilities",
			Blocks:   []string{"run:board", "verify:board"},
			Fixtures: []string{},
		})
	case "run":
		boards := make(map[string]struct{})
		for _, item := range request.Cases {
			label := item.Info
			if len(item.ID.HeadingPath) > 0 {
				label += " @ " + item.ID.HeadingPath[len(item.ID.HeadingPath)-1]
			}
			encoder.Encode(adapterprotocol.Response{
				Type:  "caseStarted",
				ID:    &item.ID,
				Label: label,
			})

			err := executeHelperBoardCase(boards, item)
			if err != nil {
				encoder.Encode(adapterprotocol.Response{
					Type:    "caseFailed",
					ID:      &item.ID,
					Label:   label,
					Message: err.Error(),
				})
				continue
			}

			encoder.Encode(adapterprotocol.Response{
				Type:  "casePassed",
				ID:    &item.ID,
				Label: label,
			})
		}
	default:
		os.Exit(3)
	}
	os.Exit(0)
}

func executeHelperBoardCase(boards map[string]struct{}, item adapterprotocol.Case) error {
	switch item.Info {
	case "run:board":
		name, err := parseHelperCommandArg(strings.TrimSpace(strings.TrimPrefix(item.Source, "create-board")))
		if err != nil {
			return err
		}
		if _, exists := boards[name]; exists {
			return &helperError{message: "board " + strconvQuote(name) + " already exists"}
		}
		boards[name] = struct{}{}
		return nil
	case "verify:board":
		name, err := parseHelperVerifySource(item.Source)
		if err != nil {
			return err
		}
		if _, exists := boards[name]; exists {
			return nil
		}
		return &helperError{message: "expected board " + strconvQuote(name) + " to exist; actual boards: [\"demo\"]"}
	default:
		return &helperError{message: "unsupported case " + item.Info}
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
