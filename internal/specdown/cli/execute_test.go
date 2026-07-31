package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type executeResult struct {
	status int
	stdout string
	stderr string
}

type failingWriter struct {
	err error
}

func (writer failingWriter) Write(_ []byte) (int, error) {
	return 0, writer.err
}

func executeForTest(t *testing.T, root string, args ...string) executeResult {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := Execute(
		args,
		strings.NewReader(""),
		&stdout,
		&stderr,
		Options{
			Version:       "test-version",
			SkillSpecdown: "# test skill\n",
			WorkingDir:    root,
		},
	)
	return executeResult{status: status, stdout: stdout.String(), stderr: stderr.String()}
}

func writeCLIFile(t *testing.T, root, relativePath, content string) {
	t.Helper()

	path := filepath.Join(root, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent directory for %s: %v", relativePath, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", relativePath, err)
	}
}

func writeMinimalCLIProject(t *testing.T, root string) {
	t.Helper()

	writeCLIFile(t, root, "specdown.json", `{"entry":"spec.md"}`)
	writeCLIFile(t, root, "spec.md", "# Example\n\nA prose-only executable specification.\n")
}

func TestExecuteTopLevelDispatch(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name       string
		args       []string
		wantStatus int
		wantStdout string
		wantStderr string
	}{
		{name: "missing command", wantStatus: 2, wantStderr: "Commands:"},
		{name: "help", args: []string{"--help"}, wantStatus: 0, wantStderr: "Commands:"},
		{name: "version", args: []string{"version"}, wantStatus: 0, wantStdout: "test-version\n"},
		{name: "version flag", args: []string{"--version"}, wantStatus: 0, wantStdout: "test-version\n"},
		{name: "unknown command", args: []string{"unknown"}, wantStatus: 2, wantStderr: `unknown command "unknown"`},
		{
			name:       "legacy version flag after unknown command",
			args:       []string{"unknown", "--version"},
			wantStatus: 0,
			wantStdout: "test-version\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executeForTest(t, root, tt.args...)
			if result.status != tt.wantStatus {
				t.Fatalf("status = %d, want %d; stdout=%q stderr=%q", result.status, tt.wantStatus, result.stdout, result.stderr)
			}
			if tt.wantStdout != "" && !strings.Contains(result.stdout, tt.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", result.stdout, tt.wantStdout)
			}
			if tt.wantStderr != "" && !strings.Contains(result.stderr, tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", result.stderr, tt.wantStderr)
			}
		})
	}
}

func TestExecuteCommandHelp(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "init", args: []string{"init", "--help"}, want: "Usage: specdown init"},
		{name: "run", args: []string{"run", "--help"}, want: "Usage: specdown run"},
		{name: "trace", args: []string{"trace", "--help"}, want: "Usage: specdown trace"},
		{name: "alloy", args: []string{"alloy", "--help"}, want: "Usage: specdown alloy"},
		{name: "alloy explore", args: []string{"alloy", "explore", "--help"}, want: "Usage: specdown alloy explore"},
		{name: "alloy dump", args: []string{"alloy", "dump", "--help"}, want: "Usage: specdown alloy dump"},
		{name: "install skills", args: []string{"install", "skills", "--help"}, want: "Usage: specdown install skills"},
		{name: "version", args: []string{"version", "--help"}, want: "Usage: specdown version"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executeForTest(t, root, tt.args...)
			if result.status != 0 {
				t.Fatalf("status = %d, want 0; stdout=%q stderr=%q", result.status, result.stdout, result.stderr)
			}
			if !strings.Contains(result.stderr, tt.want) {
				t.Errorf("stderr = %q, want it to contain %q", result.stderr, tt.want)
			}
		})
	}
}

func TestExecuteRejectsInvalidFlags(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name string
		args []string
	}{
		{name: "init", args: []string{"init", "--invalid"}},
		{name: "run", args: []string{"run", "--invalid"}},
		{name: "trace", args: []string{"trace", "--invalid"}},
		{name: "alloy explore", args: []string{"alloy", "explore", "--invalid"}},
		{name: "alloy dump", args: []string{"alloy", "dump", "--invalid"}},
		{name: "install skills", args: []string{"install", "skills", "--invalid"}},
		{name: "version", args: []string{"version", "--invalid"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executeForTest(t, root, tt.args...)
			if result.status != 1 {
				t.Fatalf("status = %d, want 1; stdout=%q stderr=%q", result.status, result.stdout, result.stderr)
			}
			if !strings.Contains(result.stderr, "flag provided but not defined") {
				t.Errorf("stderr = %q, want invalid-flag diagnostic", result.stderr)
			}
		})
	}
}

func TestExecuteInitSuccessAndUserError(t *testing.T) {
	root := t.TempDir()

	result := executeForTest(t, root, "init")
	if result.status != 0 {
		t.Fatalf("init status = %d, want 0; stderr=%q", result.status, result.stderr)
	}
	for _, relativePath := range []string{"specdown.json", "specs/index.md", "specs/example.md"} {
		if _, err := os.Stat(filepath.Join(root, relativePath)); err != nil {
			t.Errorf("expected %s: %v", relativePath, err)
		}
	}

	result = executeForTest(t, root, "init")
	if result.status != 1 || !strings.Contains(result.stderr, "specdown.json already exists") {
		t.Fatalf("second init = %+v, want status 1 and already-exists diagnostic", result)
	}
}

func TestExecuteRunSuccessAndUserError(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		root := t.TempDir()
		writeMinimalCLIProject(t, root)

		result := executeForTest(t, root, "run", "--config", "specdown.json", "--dry-run")
		if result.status != 0 {
			t.Fatalf("status = %d, want 0; stdout=%q stderr=%q", result.status, result.stdout, result.stderr)
		}
		if !strings.Contains(result.stdout, "total: 1 spec(s), 0 case(s)") {
			t.Errorf("stdout = %q, want dry-run summary", result.stdout)
		}
	})

	t.Run("quiet keeps final summary", func(t *testing.T) {
		root := t.TempDir()
		writeMinimalCLIProject(t, root)

		result := executeForTest(t, root, "run", "--config", "specdown.json", "--quiet", "--no-report")
		if result.status != 0 {
			t.Fatalf("status = %d, want 0; stdout=%q stderr=%q", result.status, result.stdout, result.stderr)
		}
		if !strings.Contains(result.stdout, "PASS 1 spec(s), 0 case(s)") {
			t.Errorf("stdout = %q, want final summary", result.stdout)
		}
	})

	t.Run("user error", func(t *testing.T) {
		root := t.TempDir()
		result := executeForTest(t, root, "run", "--config", "missing.json")
		if result.status != 1 || !strings.Contains(result.stderr, "missing.json") {
			t.Fatalf("result = %+v, want status 1 and missing-config diagnostic", result)
		}
	})
}

func TestExecuteRunUsesInjectedWorkingDirectoryForDefaults(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, root, "specs/index.md", "# Index\n\n- [Example](example.md)\n")
	writeCLIFile(t, root, "specs/example.md", "# Example\n\nProse.\n")

	result := executeForTest(t, root, "run", "--dry-run")
	if result.status != 0 {
		t.Fatalf("status = %d, want 0; stdout=%q stderr=%q", result.status, result.stdout, result.stderr)
	}
	if !strings.Contains(result.stdout, "total: 2 spec(s), 0 case(s)") {
		t.Errorf("stdout = %q, want default discovery rooted in injected working directory", result.stdout)
	}
}

func TestExecuteTraceSuccessAndUserError(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		root := t.TempDir()
		writeCLIFile(t, root, "specdown.json", `{
  "entry": "typed.md",
  "trace": {
    "types": ["spec", "goal"],
    "edges": {
      "covers": {"from": "spec", "to": "goal"}
    }
  }
}`)
		writeCLIFile(t, root, "typed.md", "---\ntype: spec\n---\n# Typed\n")

		result := executeForTest(t, root, "trace", "--config", "specdown.json")
		if result.status != 0 {
			t.Fatalf("status = %d, want 0; stdout=%q stderr=%q", result.status, result.stdout, result.stderr)
		}
		if !strings.Contains(result.stdout, `"documents"`) || !strings.Contains(result.stdout, `"type": "spec"`) {
			t.Errorf("stdout = %q, want trace JSON", result.stdout)
		}
	})

	t.Run("user error", func(t *testing.T) {
		root := t.TempDir()
		writeMinimalCLIProject(t, root)

		result := executeForTest(t, root, "trace", "--config", "specdown.json")
		if result.status != 1 || !strings.Contains(result.stderr, "no trace configuration") {
			t.Fatalf("result = %+v, want status 1 and missing-trace diagnostic", result)
		}
	})
}

func TestExecuteAlloyDumpSuccess(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, root, "specdown.json", `{"entry":"model.md"}`)
	writeCLIFile(t, root, "model.md", strings.Join([]string{
		"# Model",
		"",
		"```alloy:model(sample)",
		"module sample",
		"sig A {}",
		"```",
		"",
	}, "\n"))

	result := executeForTest(t, root, "alloy", "dump", "--config", "specdown.json")
	if result.status != 0 {
		t.Fatalf("status = %d, want 0; stdout=%q stderr=%q", result.status, result.stdout, result.stderr)
	}
	path := strings.TrimSpace(result.stdout)
	if !strings.HasSuffix(path, ".als") {
		t.Fatalf("stdout = %q, want dumped .als path", result.stdout)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("dumped Alloy model: %v", err)
	}
}

func TestExecuteAlloyExploreSuccessWithNoModels(t *testing.T) {
	root := t.TempDir()
	writeMinimalCLIProject(t, root)

	result := executeForTest(t, root, "alloy", "explore", "--config", "specdown.json")
	if result.status != 0 || !strings.Contains(result.stdout, "no Alloy models found") {
		t.Fatalf("result = %+v, want successful empty exploration", result)
	}
}

func TestExecuteAlloyCommandsReportUserErrors(t *testing.T) {
	for _, subcommand := range []string{"dump", "explore"} {
		t.Run(subcommand+" user error", func(t *testing.T) {
			root := t.TempDir()
			result := executeForTest(t, root, "alloy", subcommand, "--config", "missing.json")
			if result.status != 1 || !strings.Contains(result.stderr, "missing.json") {
				t.Fatalf("result = %+v, want status 1 and missing-config diagnostic", result)
			}
		})
	}
}

func TestExecuteAlloyRejectsUnknownSubcommand(t *testing.T) {
	result := executeForTest(t, t.TempDir(), "alloy", "unknown")
	if result.status != 1 || !strings.Contains(result.stderr, "unknown alloy subcommand") {
		t.Fatalf("result = %+v, want status 1 and unknown-subcommand diagnostic", result)
	}
}

func TestExecuteInstallSkillsSuccessAndUserError(t *testing.T) {
	root := t.TempDir()

	result := executeForTest(t, root, "install", "skills")
	if result.status != 0 {
		t.Fatalf("status = %d, want 0; stdout=%q stderr=%q", result.status, result.stdout, result.stderr)
	}
	skillPath := filepath.Join(root, ".agents", "skills", "specdown", "SKILL.md")
	body, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if string(body) != "# test skill\n" {
		t.Errorf("installed skill = %q, want injected content", body)
	}
	target, err := os.Readlink(filepath.Join(root, ".claude", "skills"))
	if err != nil {
		t.Fatalf("read compatibility symlink: %v", err)
	}
	if target != filepath.Join("..", ".agents", "skills") {
		t.Errorf("symlink target = %q, want %q", target, filepath.Join("..", ".agents", "skills"))
	}

	result = executeForTest(t, root, "install", "skills")
	if result.status != 1 || !strings.Contains(result.stderr, "already exists") {
		t.Fatalf("second install = %+v, want status 1 and already-exists diagnostic", result)
	}
}

func TestExecuteInstallSkillsPreservesConflictingDirectories(t *testing.T) {
	root := t.TempDir()
	legacyPath := filepath.Join(root, ".claude", "skills", "legacy.txt")
	currentPath := filepath.Join(root, ".agents", "skills", "current.txt")
	writeCLIFile(t, root, filepath.Join(".claude", "skills", "legacy.txt"), "legacy")
	writeCLIFile(t, root, filepath.Join(".agents", "skills", "current.txt"), "current")

	result := executeForTest(t, root, "install", "skills", "--overwrite")

	for _, want := range []string{
		"both .claude/skills and .agents/skills exist",
		"merge or remove one directory and retry",
	} {
		if !strings.Contains(result.stderr, want) {
			t.Fatalf("result = %+v, want diagnostic %q", result, want)
		}
	}
	if result.status != 1 {
		t.Fatalf("result = %+v, want a conflict error", result)
	}
	for path, want := range map[string]string{
		legacyPath:  "legacy",
		currentPath: "current",
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read preserved file %s: %v", path, err)
		}
		if string(body) != want {
			t.Fatalf("preserved file %s = %q, want %q", path, body, want)
		}
	}
}

func TestExecuteVersionRejectsArguments(t *testing.T) {
	result := executeForTest(t, t.TempDir(), "version", "extra")
	if result.status != 1 || !strings.Contains(result.stderr, "does not accept positional arguments") {
		t.Fatalf("result = %+v, want status 1 and positional-argument diagnostic", result)
	}
}

func TestExecuteReportsInjectedWriterFailures(t *testing.T) {
	writeErr := errors.New("write failed")

	t.Run("dry-run stdout", func(t *testing.T) {
		root := t.TempDir()
		writeMinimalCLIProject(t, root)
		var stderr bytes.Buffer

		status := Execute(
			[]string{"run", "--config", "specdown.json", "--dry-run"},
			strings.NewReader(""),
			failingWriter{err: writeErr},
			&stderr,
			Options{WorkingDir: root},
		)
		if status != 1 {
			t.Fatalf("status = %d, want 1 for stdout failure; stderr=%q", status, stderr.String())
		}
	})

	t.Run("help stderr", func(t *testing.T) {
		var stdout bytes.Buffer
		status := Execute(
			[]string{"help"},
			strings.NewReader(""),
			&stdout,
			failingWriter{err: writeErr},
			Options{WorkingDir: t.TempDir()},
		)
		if status != 1 {
			t.Fatalf("status = %d, want 1 for stderr failure; stdout=%q", status, stdout.String())
		}
	})
}
