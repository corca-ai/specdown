package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failingSkillFilesystem struct {
	skillFilesystem
	operation string
	path      string
	err       error
}

func (filesystem failingSkillFilesystem) Lstat(path string) (os.FileInfo, error) {
	if filesystem.operation == "lstat" && path == filesystem.path {
		return nil, filesystem.err
	}
	return filesystem.skillFilesystem.Lstat(path)
}

func (filesystem failingSkillFilesystem) Stat(path string) (os.FileInfo, error) {
	if filesystem.operation == "stat" && path == filesystem.path {
		return nil, filesystem.err
	}
	return filesystem.skillFilesystem.Stat(path)
}

func (filesystem failingSkillFilesystem) Readlink(path string) (string, error) {
	if filesystem.operation == "readlink" && path == filesystem.path {
		return "", filesystem.err
	}
	return filesystem.skillFilesystem.Readlink(path)
}

func (filesystem failingSkillFilesystem) Remove(path string) error {
	if filesystem.operation == "remove" && path == filesystem.path {
		return filesystem.err
	}
	return filesystem.skillFilesystem.Remove(path)
}

func (filesystem failingSkillFilesystem) MkdirAll(path string, perm os.FileMode) error {
	if filesystem.operation == "mkdir" && path == filesystem.path {
		return filesystem.err
	}
	return filesystem.skillFilesystem.MkdirAll(path, perm)
}

func (filesystem failingSkillFilesystem) Rename(oldPath, newPath string) error {
	if filesystem.operation == "rename" && newPath == filesystem.path {
		return filesystem.err
	}
	return filesystem.skillFilesystem.Rename(oldPath, newPath)
}

func (filesystem failingSkillFilesystem) Symlink(oldPath, newPath string) error {
	if filesystem.operation == "symlink" && newPath == filesystem.path {
		return filesystem.err
	}
	return filesystem.skillFilesystem.Symlink(oldPath, newPath)
}

func TestMigrateSkillsDirReportsFilesystemErrorsWithoutMutation(t *testing.T) {
	denied := errors.New("permission denied")

	for _, test := range []struct {
		name      string
		operation string
		failPath  func(string) string
		wantPath  string
	}{
		{
			name:      "legacy lstat",
			operation: "lstat",
			failPath:  func(root string) string { return filepath.Join(root, ".claude", "skills") },
			wantPath:  filepath.Join(".claude", "skills"),
		},
		{
			name:      "destination lstat",
			operation: "lstat",
			failPath:  func(root string) string { return filepath.Join(root, ".agents", "skills") },
			wantPath:  filepath.Join(".agents", "skills"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			legacyFile := filepath.Join(root, ".claude", "skills", "legacy.txt")
			writeCLIFile(t, root, filepath.Join(".claude", "skills", "legacy.txt"), "legacy")
			filesystem := failingSkillFilesystem{
				skillFilesystem: osSkillFilesystem{},
				operation:       test.operation,
				path:            test.failPath(root),
				err:             denied,
			}
			cmd := command{workingDir: root, stdout: io.Discard, filesystem: filesystem}

			err := cmd.migrateSkillsDir()

			if err == nil || !strings.Contains(err.Error(), test.wantPath) || !errors.Is(err, denied) {
				t.Fatalf("migrate error = %v, want %s and injected error", err, test.wantPath)
			}
			body, readErr := os.ReadFile(legacyFile)
			if readErr != nil {
				t.Fatalf("read preserved legacy file: %v", readErr)
			}
			if string(body) != "legacy" {
				t.Fatalf("legacy file = %q, want preserved content", body)
			}
		})
	}
}

func TestMigrateSkillsDirReportsMutationErrorsWithoutRemovingLegacyData(t *testing.T) {
	denied := errors.New("permission denied")

	for _, test := range []struct {
		name      string
		operation string
		failPath  func(string) string
	}{
		{
			name:      "create destination parent",
			operation: "mkdir",
			failPath:  func(root string) string { return filepath.Join(root, ".agents") },
		},
		{
			name:      "rename legacy directory",
			operation: "rename",
			failPath:  func(root string) string { return filepath.Join(root, ".agents", "skills") },
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			legacyFile := filepath.Join(root, ".claude", "skills", "legacy.txt")
			writeCLIFile(t, root, filepath.Join(".claude", "skills", "legacy.txt"), "legacy")
			filesystem := failingSkillFilesystem{
				skillFilesystem: osSkillFilesystem{},
				operation:       test.operation,
				path:            test.failPath(root),
				err:             denied,
			}
			cmd := command{workingDir: root, stdout: io.Discard, filesystem: filesystem}

			err := cmd.migrateSkillsDir()

			if err == nil || !errors.Is(err, denied) {
				t.Fatalf("migrate error = %v, want injected error", err)
			}
			body, readErr := os.ReadFile(legacyFile)
			if readErr != nil {
				t.Fatalf("read preserved legacy file: %v", readErr)
			}
			if string(body) != "legacy" {
				t.Fatalf("legacy file = %q, want preserved content", body)
			}
		})
	}
}

func TestInstallSkillsReportsDestinationStatError(t *testing.T) {
	root := t.TempDir()
	denied := errors.New("permission denied")
	agentsDir := filepath.Join(root, ".agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("create agents directory: %v", err)
	}
	skillsPath := filepath.Join(agentsDir, "skills")
	if err := os.Symlink("missing", skillsPath); err != nil {
		t.Fatalf("create dangling symlink: %v", err)
	}
	filesystem := failingSkillFilesystem{
		skillFilesystem: osSkillFilesystem{},
		operation:       "stat",
		path:            skillsPath,
		err:             denied,
	}
	cmd := command{
		workingDir: root,
		stdout:     io.Discard,
		skill:      "# test skill\n",
		filesystem: filesystem,
	}

	err := cmd.installSkills(true)

	if err == nil || !strings.Contains(err.Error(), filepath.Join(".agents", "skills")) ||
		!errors.Is(err, denied) {
		t.Fatalf("install error = %v, want skills path and injected error", err)
	}
	info, statErr := os.Lstat(skillsPath)
	if statErr != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("installation mutated symlink after stat error: info=%v err=%v", info, statErr)
	}
}

func TestRemoveDanglingSymlinkReportsRemoveError(t *testing.T) {
	root := t.TempDir()
	agentsDir := filepath.Join(root, ".agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("create agents directory: %v", err)
	}
	skillsPath := filepath.Join(agentsDir, "skills")
	if err := os.Symlink("missing", skillsPath); err != nil {
		t.Fatalf("create dangling symlink: %v", err)
	}
	denied := errors.New("permission denied")
	filesystem := failingSkillFilesystem{
		skillFilesystem: osSkillFilesystem{},
		operation:       "remove",
		path:            skillsPath,
		err:             denied,
	}

	err := removeDanglingSymlink(filesystem, skillsPath)

	if err == nil || !strings.Contains(err.Error(), skillsPath) || !errors.Is(err, denied) {
		t.Fatalf("remove error = %v, want symlink path and injected error", err)
	}
	info, statErr := os.Lstat(skillsPath)
	if statErr != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("dangling symlink was not preserved: info=%v err=%v", info, statErr)
	}
}

func TestEnsureSkillsSymlinkReportsSymlinkErrors(t *testing.T) {
	root := t.TempDir()
	claudeSkills := filepath.Join(root, ".claude", "skills")
	denied := errors.New("permission denied")
	filesystem := failingSkillFilesystem{
		skillFilesystem: osSkillFilesystem{},
		operation:       "symlink",
		path:            claudeSkills,
		err:             denied,
	}
	cmd := command{workingDir: root, filesystem: filesystem}

	err := cmd.ensureSkillsSymlink()

	if err == nil || !strings.Contains(err.Error(), filepath.Join(".claude", "skills")) || !errors.Is(err, denied) {
		t.Fatalf("symlink error = %v, want compatibility path and injected error", err)
	}
	if _, statErr := os.Lstat(claudeSkills); !os.IsNotExist(statErr) {
		t.Fatalf("unexpected compatibility path after symlink error: %v", statErr)
	}
}

func TestEnsureSkillsSymlinkReportsReadlinkErrorWithoutReplacingLink(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("create claude directory: %v", err)
	}
	claudeSkills := filepath.Join(claudeDir, "skills")
	target := filepath.Join("..", ".agents", "skills")
	if err := os.Symlink(target, claudeSkills); err != nil {
		t.Fatalf("create compatibility symlink: %v", err)
	}
	denied := errors.New("permission denied")
	filesystem := failingSkillFilesystem{
		skillFilesystem: osSkillFilesystem{},
		operation:       "readlink",
		path:            claudeSkills,
		err:             denied,
	}
	cmd := command{workingDir: root, filesystem: filesystem}

	err := cmd.ensureSkillsSymlink()

	if err == nil || !strings.Contains(err.Error(), filepath.Join(".claude", "skills")) || !errors.Is(err, denied) {
		t.Fatalf("readlink error = %v, want compatibility path and injected error", err)
	}
	gotTarget, readErr := os.Readlink(claudeSkills)
	if readErr != nil || gotTarget != target {
		t.Fatalf("compatibility symlink changed: target=%q err=%v", gotTarget, readErr)
	}
}

func TestExecuteInstallSkillsReplacesDanglingAgentsSymlink(t *testing.T) {
	root := t.TempDir()
	agentsDir := filepath.Join(root, ".agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("create agents directory: %v", err)
	}
	skillsPath := filepath.Join(agentsDir, "skills")
	if err := os.Symlink("missing", skillsPath); err != nil {
		t.Fatalf("create dangling symlink: %v", err)
	}

	result := executeForTest(t, root, "install", "skills")

	if result.status != 0 {
		t.Fatalf("result = %+v, want successful install", result)
	}
	info, err := os.Stat(skillsPath)
	if err != nil {
		t.Fatalf("stat replacement skills directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%s is not a directory after install", skillsPath)
	}
}

func TestExecuteInstallSkillsPreservesUnexpectedCompatibilitySymlink(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	externalFile := filepath.Join(external, "custom.txt")
	if err := os.WriteFile(externalFile, []byte("custom"), 0o600); err != nil {
		t.Fatalf("write external file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatalf("create claude directory: %v", err)
	}
	compatibilityPath := filepath.Join(root, ".claude", "skills")
	if err := os.Symlink(external, compatibilityPath); err != nil {
		t.Fatalf("create custom compatibility symlink: %v", err)
	}
	currentFile := filepath.Join(root, ".agents", "skills", "current.txt")
	writeCLIFile(t, root, filepath.Join(".agents", "skills", "current.txt"), "current")

	result := executeForTest(t, root, "install", "skills", "--overwrite")

	if result.status != 1 || !strings.Contains(result.stderr, "already points to") {
		t.Fatalf("result = %+v, want compatibility conflict", result)
	}
	target, err := os.Readlink(compatibilityPath)
	if err != nil || target != external {
		t.Fatalf("compatibility symlink changed: target=%q err=%v", target, err)
	}
	body, err := os.ReadFile(externalFile)
	if err != nil || string(body) != "custom" {
		t.Fatalf("external data changed: body=%q err=%v", body, err)
	}
	body, err = os.ReadFile(currentFile)
	if err != nil || string(body) != "current" {
		t.Fatalf("current skills data changed: body=%q err=%v", body, err)
	}
}

func TestExecuteInstallSkillsRejectsSymlinkedOutput(t *testing.T) {
	root := t.TempDir()
	externalFile := filepath.Join(t.TempDir(), "overview.md")
	if err := os.WriteFile(externalFile, []byte("external"), 0o600); err != nil {
		t.Fatalf("write external file: %v", err)
	}
	outputDir := filepath.Join(root, ".agents", "skills", "specdown")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}
	outputPath := filepath.Join(outputDir, "overview.md")
	if err := os.Symlink(externalFile, outputPath); err != nil {
		t.Fatalf("create output symlink: %v", err)
	}

	result := executeForTest(t, root, "install", "skills", "--overwrite")

	if result.status != 1 || !strings.Contains(result.stderr, "not a regular file") {
		t.Fatalf("result = %+v, want output symlink rejection", result)
	}
	body, err := os.ReadFile(externalFile)
	if err != nil || string(body) != "external" {
		t.Fatalf("external file changed: body=%q err=%v", body, err)
	}
}

func TestExecuteInstallSkillsRejectsSymlinkedDirectories(t *testing.T) {
	for _, relativePath := range []string{
		".agents",
		filepath.Join(".agents", "skills", "specdown"),
	} {
		t.Run(relativePath, func(t *testing.T) {
			assertInstallRejectsSymlinkedDirectory(t, relativePath)
		})
	}
}

func assertInstallRejectsSymlinkedDirectory(t *testing.T, relativePath string) {
	t.Helper()
	root := t.TempDir()
	external := t.TempDir()
	externalFile := filepath.Join(external, "custom.txt")
	if err := os.WriteFile(externalFile, []byte("external"), 0o600); err != nil {
		t.Fatalf("write external file: %v", err)
	}
	linkPath := filepath.Join(root, relativePath)
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		t.Fatalf("create symlink parent: %v", err)
	}
	if err := os.Symlink(external, linkPath); err != nil {
		t.Fatalf("create directory symlink: %v", err)
	}

	result := executeForTest(t, root, "install", "skills", "--overwrite")

	if result.status != 1 || !strings.Contains(result.stderr, "is a symlink") {
		t.Fatalf("result = %+v, want directory symlink rejection", result)
	}
	body, err := os.ReadFile(externalFile)
	if err != nil || string(body) != "external" {
		t.Fatalf("external data changed: body=%q err=%v", body, err)
	}
}

func TestExecuteInstallSkillsRequiresOverwriteForEveryManagedFile(t *testing.T) {
	root := t.TempDir()
	overviewPath := filepath.Join(root, ".agents", "skills", "specdown", "overview.md")
	writeCLIFile(t, root, filepath.Join(".agents", "skills", "specdown", "overview.md"), "custom")

	result := executeForTest(t, root, "install", "skills")

	if result.status != 1 || !strings.Contains(result.stderr, "overview.md already exists") {
		t.Fatalf("result = %+v, want existing managed-file error", result)
	}
	body, err := os.ReadFile(overviewPath)
	if err != nil || string(body) != "custom" {
		t.Fatalf("existing managed file changed: body=%q err=%v", body, err)
	}
}
