package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	specdown "github.com/corca-ai/specdown"
)

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

func removeDanglingSymlink(path string) {
	if _, err := os.Readlink(path); err == nil {
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			_ = os.Remove(path)
		}
	}
}

func (c command) migrateSkillsDir() error {
	claudeSkillsRelative := filepath.Join(".claude", "skills")
	agentsSkillsRelative := filepath.Join(".agents", "skills")
	claudeSkills := filepath.Join(c.workingDir, claudeSkillsRelative)
	agentsSkills := filepath.Join(c.workingDir, agentsSkillsRelative)
	info, err := os.Lstat(claudeSkills)
	if err != nil {
		return nil //nolint:nilerr // Lstat error means path doesn't exist; nothing to migrate.
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	if _, err := os.Stat(agentsSkills); !os.IsNotExist(err) {
		return os.RemoveAll(claudeSkills)
	}
	if err := os.MkdirAll(filepath.Join(c.workingDir, ".agents"), 0o755); err != nil {
		return err
	}
	if err := os.Rename(claudeSkills, agentsSkills); err != nil {
		return fmt.Errorf("migrate %s → %s: %w", claudeSkillsRelative, agentsSkillsRelative, err)
	}
	_, err = fmt.Fprintf(c.stdout, "Migrated %s → %s\n", claudeSkillsRelative, agentsSkillsRelative)
	return err
}

func (c command) ensureSkillsSymlink() error {
	claudeSkills := filepath.Join(c.workingDir, ".claude", "skills")
	agentsSkills := filepath.Join(".agents", "skills")
	relTarget := filepath.Join("..", agentsSkills)
	if target, err := os.Readlink(claudeSkills); err == nil && target == relTarget {
		return nil
	}
	_ = os.Remove(claudeSkills)
	if err := os.MkdirAll(filepath.Join(c.workingDir, ".claude"), 0o755); err != nil {
		return err
	}
	return os.Symlink(relTarget, claudeSkills)
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

func (c command) installSkills(overwrite bool) error {
	dirRelative := filepath.Join(".agents", "skills", "specdown")
	destRelative := filepath.Join(dirRelative, "SKILL.md")
	dir := filepath.Join(c.workingDir, dirRelative)
	dest := filepath.Join(c.workingDir, destRelative)

	if err := c.migrateSkillsDir(); err != nil {
		return err
	}
	if _, err := os.Stat(dest); err == nil && !overwrite {
		return fmt.Errorf("%s already exists\nhint: use --overwrite to replace existing files", destRelative)
	}

	removeDanglingSymlink(filepath.Join(c.workingDir, ".agents", "skills"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := c.ensureSkillsSymlink(); err != nil {
		return err
	}
	return c.writeSkillFiles(dir, dirRelative)
}

func (c command) writeSkillFiles(dir, dirRelative string) error {
	files := []struct{ name, content string }{
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
	for _, file := range files {
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
