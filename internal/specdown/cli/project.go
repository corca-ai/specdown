package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	specdown "github.com/corca-ai/specdown"
)

type skillFilesystem interface {
	Lstat(string) (os.FileInfo, error)
	Stat(string) (os.FileInfo, error)
	Readlink(string) (string, error)
	Remove(string) error
	MkdirAll(string, os.FileMode) error
	Rename(string, string) error
	Symlink(string, string) error
}

type osSkillFilesystem struct{}

func (osSkillFilesystem) Lstat(path string) (os.FileInfo, error) {
	return os.Lstat(path)
}

func (osSkillFilesystem) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (osSkillFilesystem) Readlink(path string) (string, error) {
	return os.Readlink(path)
}

func (osSkillFilesystem) Remove(path string) error {
	return os.Remove(path)
}

func (osSkillFilesystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (osSkillFilesystem) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func (osSkillFilesystem) Symlink(oldPath, newPath string) error {
	return os.Symlink(oldPath, newPath)
}

func (c command) skillFilesystem() skillFilesystem {
	if c.filesystem != nil {
		return c.filesystem
	}
	return osSkillFilesystem{}
}

func (c command) initCmd(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	fs.Usage = func() {
		writeLine(c.stderr, "Usage: specdown init")
		writeLine(c.stderr)
		writeLine(c.stderr, "Scaffold a new specdown project in the current directory.")
		writeLine(c.stderr, "Creates specdown.json, specs/index.md, and specs/example.md.")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("init does not accept positional arguments")
	}
	return c.initProject()
}

func removeDanglingSymlink(filesystem skillFilesystem, path string) error {
	info, err := filesystem.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	if _, err := filesystem.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("resolve symlink %s: %w", path, err)
	}
	if err := filesystem.Remove(path); err != nil {
		return fmt.Errorf("remove dangling symlink %s: %w", path, err)
	}
	return nil
}

func (c command) migrateSkillsDir() error {
	filesystem := c.skillFilesystem()
	claudeSkillsRelative := filepath.Join(".claude", "skills")
	agentsSkillsRelative := filepath.Join(".agents", "skills")
	claudeSkills := filepath.Join(c.workingDir, claudeSkillsRelative)
	agentsDir := filepath.Join(c.workingDir, ".agents")
	agentsSkills := filepath.Join(c.workingDir, agentsSkillsRelative)
	info, err := filesystem.Lstat(claudeSkills)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect %s before migration: %w", claudeSkillsRelative, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	if !info.IsDir() {
		return fmt.Errorf("cannot migrate %s: path exists and is not a directory", claudeSkillsRelative)
	}
	if err := validateRealDirectoryIfPresent(filesystem, agentsDir, ".agents"); err != nil {
		return err
	}
	if _, err := filesystem.Lstat(agentsSkills); err == nil {
		return fmt.Errorf(
			"cannot migrate skills because both %s and %s exist; merge or remove one directory and retry",
			claudeSkillsRelative,
			agentsSkillsRelative,
		)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect %s before migration: %w", agentsSkillsRelative, err)
	}
	if err := filesystem.MkdirAll(filepath.Join(c.workingDir, ".agents"), 0o755); err != nil {
		return fmt.Errorf("create .agents for skill migration: %w", err)
	}
	if err := filesystem.Rename(claudeSkills, agentsSkills); err != nil {
		return fmt.Errorf("migrate %s → %s: %w", claudeSkillsRelative, agentsSkillsRelative, err)
	}
	_, err = fmt.Fprintf(c.stdout, "Migrated %s → %s\n", claudeSkillsRelative, agentsSkillsRelative)
	return err
}

func validateRealDirectoryIfPresent(filesystem skillFilesystem, path, relativePath string) error {
	info, err := filesystem.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect %s: %w", relativePath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("cannot install skills: %s is a symlink; use a directory inside the project", relativePath)
	}
	if !info.IsDir() {
		return fmt.Errorf("cannot install skills: %s exists and is not a directory", relativePath)
	}
	return nil
}

func (c command) inspectSkillsSymlink() (bool, error) {
	filesystem := c.skillFilesystem()
	claudeSkillsRelative := filepath.Join(".claude", "skills")
	claudeSkills := filepath.Join(c.workingDir, ".claude", "skills")
	agentsSkills := filepath.Join(".agents", "skills")
	relTarget := filepath.Join("..", agentsSkills)
	info, err := filesystem.Lstat(claudeSkills)
	switch {
	case err == nil && info.Mode()&os.ModeSymlink != 0:
		target, readErr := filesystem.Readlink(claudeSkills)
		if readErr != nil {
			return false, fmt.Errorf("read compatibility symlink %s: %w", claudeSkillsRelative, readErr)
		}
		if target == relTarget {
			return true, nil
		}
		return false, fmt.Errorf(
			"cannot create compatibility symlink: %s already points to %q; preserve or remove it explicitly and retry",
			claudeSkillsRelative,
			target,
		)
	case err == nil:
		return false, fmt.Errorf("cannot create compatibility symlink: %s exists and is not a symlink", claudeSkillsRelative)
	case !os.IsNotExist(err):
		return false, fmt.Errorf("inspect compatibility symlink %s: %w", claudeSkillsRelative, err)
	}
	return false, nil
}

func (c command) ensureSkillsSymlink() error {
	exists, err := c.inspectSkillsSymlink()
	if err != nil || exists {
		return err
	}
	filesystem := c.skillFilesystem()
	claudeSkillsRelative := filepath.Join(".claude", "skills")
	claudeSkills := filepath.Join(c.workingDir, claudeSkillsRelative)
	relTarget := filepath.Join("..", ".agents", "skills")
	if err := filesystem.MkdirAll(filepath.Join(c.workingDir, ".claude"), 0o755); err != nil {
		return fmt.Errorf("create .claude for compatibility symlink: %w", err)
	}
	if err := filesystem.Symlink(relTarget, claudeSkills); err != nil {
		return fmt.Errorf("create compatibility symlink %s: %w", claudeSkillsRelative, err)
	}
	return nil
}

func (c command) installSkillsCmd(args []string) error {
	if len(args) == 0 || hasHelpFlag(args) {
		c.printInstallSkillsUsage()
		return nil
	}
	if args[0] != "skills" {
		return fmt.Errorf("unknown install target %q\nhint: run 'specdown install --help'", args[0])
	}

	overwrite, err := c.parseInstallSkillsFlags(args[1:])
	if err != nil {
		return err
	}
	return c.installSkills(overwrite)
}

func (c command) printInstallSkillsUsage() {
	writeLine(c.stderr, "Usage: specdown install skills [--overwrite]")
	writeLine(c.stderr)
	writeLine(c.stderr, "Install Claude Code skills for this project.")
	writeLine(c.stderr, "Creates .agents/skills/specdown/SKILL.md in the current directory.")
	writeLine(c.stderr, "Use --overwrite to replace existing files.")
}

func (c command) parseInstallSkillsFlags(args []string) (bool, error) {
	fs := flag.NewFlagSet("install skills", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	overwrite := fs.Bool("overwrite", false, "Replace existing skill files")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return false, nil
		}
		return false, err
	}
	if fs.NArg() > 0 {
		return false, fmt.Errorf("install skills does not accept positional arguments")
	}
	return *overwrite, nil
}

type skillFile struct {
	name    string
	content string
}

func (c command) skillFiles() []skillFile {
	return []skillFile{
		{"SKILL.md", c.skill},
		{"overview.md", specdown.SkillOverview},
		{"syntax.md", specdown.SkillSyntax},
		{"config.md", specdown.SkillConfig},
		{"cli.md", specdown.SkillCLI},
		{"adapter-protocol.md", specdown.SkillAdapterProtocol},
		{"alloy.md", specdown.SkillAlloy},
		{"report.md", specdown.SkillReport},
		{"internals.md", specdown.SkillInternals},
		{"best-practices.md", specdown.SkillBestPractices},
		{"validation.md", specdown.SkillValidation},
		{"traceability.md", specdown.SkillTraceability},
		{"guide-alloy-explore.md", specdown.SkillGuideAlloyExplore},
		{"workflow-new-project.md", specdown.SkillWorkflowNewProject},
		{"workflow-adopt.md", specdown.SkillWorkflowAdopt},
		{"workflow-evolve.md", specdown.SkillWorkflowEvolve},
	}
}

func (c command) preflightSkillDestination(overwrite bool) error {
	if err := c.preflightSkillDirectories(); err != nil {
		return err
	}
	return c.preflightSkillFiles(overwrite)
}

func (c command) preflightSkillDirectories() error {
	filesystem := c.skillFilesystem()
	directories := []string{
		".agents",
		filepath.Join(".agents", "skills"),
		filepath.Join(".agents", "skills", "specdown"),
	}
	for _, relativePath := range directories {
		missing, err := preflightSkillDirectory(filesystem, filepath.Join(c.workingDir, relativePath), relativePath)
		if err != nil {
			return err
		}
		if missing {
			return nil
		}
	}
	return nil
}

func preflightSkillDirectory(filesystem skillFilesystem, path, relativePath string) (bool, error) {
	info, err := filesystem.Lstat(path)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect %s before installation: %w", relativePath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		if !info.IsDir() {
			return false, fmt.Errorf("cannot install skills: %s exists and is not a directory", relativePath)
		}
		return false, nil
	}
	if relativePath != filepath.Join(".agents", "skills") {
		return false, fmt.Errorf("cannot install skills: %s is a symlink; use a directory inside the project", relativePath)
	}
	if _, statErr := filesystem.Stat(path); os.IsNotExist(statErr) {
		return true, nil
	} else if statErr != nil {
		return false, fmt.Errorf("resolve symlink %s before installation: %w", relativePath, statErr)
	}
	return false, fmt.Errorf("cannot install skills: %s is a symlink; use a directory inside the project", relativePath)
}

func (c command) preflightSkillFiles(overwrite bool) error {
	filesystem := c.skillFilesystem()
	dirRelative := filepath.Join(".agents", "skills", "specdown")
	for _, file := range c.skillFiles() {
		relativePath := filepath.Join(dirRelative, file.name)
		info, err := filesystem.Lstat(filepath.Join(c.workingDir, relativePath))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect %s before installation: %w", relativePath, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("cannot install skills: %s exists and is not a regular file", relativePath)
		}
		if !overwrite {
			return fmt.Errorf("%s already exists\nhint: use --overwrite to replace existing files", relativePath)
		}
	}
	return nil
}

func (c command) installSkills(overwrite bool) error {
	filesystem := c.skillFilesystem()
	dirRelative := filepath.Join(".agents", "skills", "specdown")
	dir := filepath.Join(c.workingDir, dirRelative)

	if err := c.migrateSkillsDir(); err != nil {
		return err
	}
	if _, err := c.inspectSkillsSymlink(); err != nil {
		return err
	}
	if err := c.preflightSkillDestination(overwrite); err != nil {
		return err
	}

	if err := removeDanglingSymlink(filesystem, filepath.Join(c.workingDir, ".agents", "skills")); err != nil {
		return err
	}
	if err := filesystem.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dirRelative, err)
	}
	if err := c.ensureSkillsSymlink(); err != nil {
		return err
	}
	return c.writeSkillFiles(dir, dirRelative)
}

func (c command) writeSkillFiles(dir, dirRelative string) error {
	for _, file := range c.skillFiles() {
		path := filepath.Join(dir, file.name)
		if err := os.WriteFile(path, []byte(file.content), 0o644); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(c.stdout, "Created %s\n", filepath.Join(dirRelative, file.name)); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(c.stdout, "Use /specdown in Claude Code to run and fix specs.")
	return err
}

func (c command) initProject() error {
	configPath := filepath.Join(c.workingDir, "specdown.json")
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("specdown.json already exists\nhint: to start fresh, remove specdown.json and the specs/ directory first")
	}

	specsDir := filepath.Join(c.workingDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		return err
	}

	configJSON := `{
  "entry": "specs/index.md"
}
`
	if err := os.WriteFile(configPath, []byte(configJSON), 0o644); err != nil {
		return err
	}

	indexMD := "# My Project\n\n- [Example](example.md)\n"
	if err := os.WriteFile(filepath.Join(specsDir, "index.md"), []byte(indexMD), 0o644); err != nil {
		return err
	}

	exampleMD := `# Example

This is a sample spec. Add executable blocks and check tables to make it live.

## Getting Started

Prose paragraphs are preserved in the HTML report.
Only executable blocks and check tables are run.
`
	if err := os.WriteFile(filepath.Join(specsDir, "example.md"), []byte(exampleMD), 0o644); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(c.stdout, "Created specdown.json, specs/index.md, specs/example.md"); err != nil {
		return err
	}
	_, err := fmt.Fprintln(c.stdout, "Run: specdown run")
	return err
}
