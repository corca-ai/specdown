package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "embed"

	specdown "github.com/corca-ai/specdown"
	"github.com/corca-ai/specdown/internal/specdown/config"
	"github.com/corca-ai/specdown/internal/specdown/core"
	"github.com/corca-ai/specdown/internal/specdown/engine"
	htmlreport "github.com/corca-ai/specdown/internal/specdown/reporter/html"
	jsonreport "github.com/corca-ai/specdown/internal/specdown/reporter/json"
)

var version = "dev"

//go:embed skills/specdown/SKILL.md
var skillSpecdown string

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "help", "--help", "-help", "-h":
		usage()
	case "init":
		err = initCmd(os.Args[2:])
	case "run":
		err = run(os.Args[2:])
	case "alloy":
		err = alloyCmd(os.Args[2:])
	case "install":
		err = installSkillsCmd(os.Args[2:])
	case "version", "--version", "-version":
		fmt.Println(version)
	default:
		unknownCmd(os.Args[1:])
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "specdown: %v\n", err)
		os.Exit(1)
	}
}

func unknownCmd(args []string) {
	for _, arg := range args {
		if arg == "--version" || arg == "-version" {
			fmt.Println(version)
			return
		}
	}
	fmt.Fprintf(os.Stderr, "specdown: unknown command %q\n\n", args[0])
	usage()
	os.Exit(2)
}

func initCmd(args []string) error {
	if hasHelpFlag(args) {
		fmt.Fprintln(os.Stderr, "Usage: specdown init")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Scaffold a new specdown project in the current directory.")
		fmt.Fprintln(os.Stderr, "Creates specdown.json, specs/index.spec.md, and specs/example.spec.md.")
		return nil
	}
	return initProject()
}

func configLoadErr(err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return pathErr
	}
	return err
}

func hasHelpFlag(args []string) bool {
	for _, a := range args {
		if a == "-help" || a == "--help" || a == "-h" {
			return true
		}
	}
	return false
}

func run(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: specdown run [flags]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Execute spec files and generate HTML/JSON reports.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Flags:")
		fs.PrintDefaults()
	}

	configPath := fs.String("config", "specdown.json", "Path to specdown.json")
	outPath := fs.String("out", "", "Output HTML report path")
	filter := fs.String("filter", "", "Run only cases whose heading path contains this string")
	jobs := fs.Int("jobs", 1, "Number of spec files to run in parallel")
	dryRun := fs.Bool("dry-run", false, "Parse and validate without executing")
	showBindings := fs.Bool("show-bindings", false, "Print resolved variable bindings for each case")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	cfg, configDir, err := config.Load(*configPath)
	if err != nil {
		if os.IsNotExist(configLoadErr(err)) {
			return fmt.Errorf("%w\nhint: run 'specdown init' to create a new project, or use -config to specify a config file", err)
		}
		return err
	}

	opts := engine.RunOptions{
		Filter: *filter,
		Jobs:   *jobs,
		DryRun: *dryRun,
	}

	report, err := engine.Run(configDir, cfg, opts)
	if err != nil {
		return err
	}

	printWarnings(report)

	if *dryRun {
		printDryRun(report)
		return nil
	}

	if *showBindings {
		printBindings(report)
	}

	reportPath := resolveReportPath(configDir, cfg, *outPath)
	if err := writeArtifacts(report, reportPath, configDir, cfg); err != nil {
		return err
	}

	if report.Summary.SpecsFailed > 0 {
		printFailures(report)
		fmt.Fprintf(os.Stderr, "\nFAIL %d spec(s), %d case(s), %d alloy check(s)\n", report.Summary.SpecsFailed, report.Summary.CasesFailed, report.Summary.AlloyChecksFailed)
		fmt.Fprintf(os.Stderr, "report: %s\n", reportPath)
		return fmt.Errorf("spec run failed")
	}

	fmt.Printf("PASS %d spec(s), %d case(s), %d alloy check(s)\n", report.Summary.SpecsTotal, report.Summary.CasesTotal, report.Summary.AlloyChecksTotal)
	fmt.Printf("report: %s\n", reportPath)
	return nil
}

func alloyCmd(args []string) error {
	if len(args) == 0 || (len(args) == 1 && hasHelpFlag(args)) {
		fmt.Fprintln(os.Stderr, "Usage: specdown alloy <subcommand>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  dump  Export embedded Alloy models as .als files")
		return nil
	}

	switch args[0] {
	case "dump":
		return alloyDump(args[1:])
	default:
		return fmt.Errorf("unknown alloy subcommand %q\nhint: run 'specdown alloy --help' for available subcommands", args[0])
	}
}

func alloyDump(args []string) error {
	fs := flag.NewFlagSet("alloy dump", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: specdown alloy dump [flags]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Export embedded Alloy models from spec files as .als files.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Flags:")
		fs.PrintDefaults()
	}
	configPath := fs.String("config", "specdown.json", "Path to specdown.json")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	cfg, configDir, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	paths, err := engine.DumpAlloyModels(configDir, cfg)
	if err != nil {
		return err
	}

	for _, p := range paths {
		fmt.Println(p)
	}
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "specdown — Markdown-first executable specifications")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  init            Scaffold a new project (creates specdown.json and example specs)")
	fmt.Fprintln(os.Stderr, "  run             Execute specs and generate HTML/JSON reports")
	fmt.Fprintln(os.Stderr, "  install skills  Install Claude Code skills for this project")
	fmt.Fprintln(os.Stderr, "  alloy dump      Export embedded Alloy models as .als files")
	fmt.Fprintln(os.Stderr, "  version         Print the specdown version")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Run 'specdown <command> --help' for details on a specific command.")
}

func installSkillsCmd(args []string) error {
	if len(args) == 0 || hasHelpFlag(args) {
		fmt.Fprintln(os.Stderr, "Usage: specdown install skills")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Install Claude Code skills for this project.")
		fmt.Fprintln(os.Stderr, "Creates .claude/skills/specdown/SKILL.md in the current directory.")
		return nil
	}
	if args[0] != "skills" {
		return fmt.Errorf("unknown install target %q\nhint: run 'specdown install --help'", args[0])
	}

	dir := filepath.Join(".claude", "skills", "specdown")
	dest := filepath.Join(dir, "SKILL.md")

	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("%s already exists", dest)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dest, []byte(skillSpecdown), 0o644); err != nil {
		return err
	}
	guideDest := filepath.Join(dir, "guide-writing.md")
	if err := os.WriteFile(guideDest, []byte(specdown.SkillWritingGuide), 0o644); err != nil {
		return err
	}
	protocolDest := filepath.Join(dir, "adapter-protocol.md")
	if err := os.WriteFile(protocolDest, []byte(specdown.SkillAdapterProtocol), 0o644); err != nil {
		return err
	}
	syntaxDest := filepath.Join(dir, "syntax.md")
	if err := os.WriteFile(syntaxDest, []byte(specdown.SkillSyntax), 0o644); err != nil {
		return err
	}

	fmt.Printf("Created %s\n", dest)
	fmt.Printf("Created %s\n", guideDest)
	fmt.Printf("Created %s\n", protocolDest)
	fmt.Printf("Created %s\n", syntaxDest)
	fmt.Println("Use /specdown in Claude Code to run and fix specs.")
	return nil
}

func initProject() error {
	if _, err := os.Stat("specdown.json"); err == nil {
		return fmt.Errorf("specdown.json already exists")
	}

	if err := os.MkdirAll("specs", 0o755); err != nil {
		return err
	}

	configJSON := `{
  "entry": "specs/index.spec.md",
  "adapters": [],
  "reporters": [
    { "builtin": "html", "outFile": ".artifacts/specdown/report.html" },
    { "builtin": "json", "outFile": ".artifacts/specdown/report.json" }
  ]
}
`
	if err := os.WriteFile("specdown.json", []byte(configJSON), 0o644); err != nil {
		return err
	}

	indexMD := "# My Project\n\n- [Example](example.spec.md)\n"
	if err := os.WriteFile("specs/index.spec.md", []byte(indexMD), 0o644); err != nil {
		return err
	}

	exampleMD := `# Example

This is a sample spec. Add executable blocks and check tables to make it live.

## Getting Started

Prose paragraphs are preserved in the HTML report.
Only executable blocks and check tables are run.
`
	if err := os.WriteFile("specs/example.spec.md", []byte(exampleMD), 0o644); err != nil {
		return err
	}

	fmt.Println("Created specdown.json, specs/index.spec.md, specs/example.spec.md")
	fmt.Println("Run: specdown run")
	return nil
}

func writeArtifacts(report core.Report, reportPath string, baseDir string, cfg config.Config) error {
	if err := htmlreport.Write(report, reportPath); err != nil {
		return err
	}
	jsonPath := cfg.JSONReportOutFile()
	if jsonPath == "" {
		jsonPath = jsonReportPath(reportPath)
	} else {
		jsonPath = resolvePath(baseDir, jsonPath)
	}
	if err := jsonreport.Write(report, jsonPath); err != nil {
		return err
	}
	return nil
}

func resolveReportPath(baseDir string, cfg config.Config, requested string) string {
	reportPath := requested
	if reportPath == "" {
		reportPath = cfg.HTMLReportOutFile()
	}
	if reportPath == "" {
		reportPath = ".artifacts/specdown/report.html"
	}
	return resolvePath(baseDir, reportPath)
}

func jsonReportPath(htmlReportPath string) string {
	dir := filepath.Dir(htmlReportPath)
	return filepath.Join(dir, "report.json")
}

func resolvePath(baseDir string, value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Clean(filepath.Join(baseDir, value))
}

func printFailures(report core.Report) {
	for _, doc := range report.Results {
		printCaseFailures(doc.Cases)
		printAlloyFailures(doc.AlloyChecks)
	}
}

func printCaseFailures(cases []core.CaseResult) {
	for _, c := range cases {
		if c.Status != core.StatusFailed {
			continue
		}
		printCaseFailure(c)
	}
}

func printCaseFailure(c core.CaseResult) {
	path := strings.Join(c.ID.HeadingPath, " > ")
	kind := c.Block + c.Check
	label := ""
	if c.Kind == core.CaseKindTableRow && c.RowNumber > 0 {
		label = fmt.Sprintf(" row %d", c.RowNumber)
		if c.Label != "" {
			label = fmt.Sprintf(" row %d %q", c.RowNumber, c.Label)
		}
	}
	fmt.Fprintf(os.Stderr, "  FAIL  %s  [%s]%s\n", path, kind, label)
	if c.Message != "" {
		fmt.Fprintf(os.Stderr, "        %s\n", c.Message)
	}
	if c.Expected != "" {
		fmt.Fprintf(os.Stderr, "        expected: %s\n", c.Expected)
	}
	if c.Actual != "" {
		fmt.Fprintf(os.Stderr, "        actual:   %s\n", c.Actual)
	}
	for _, step := range c.Steps {
		if step.Status != core.StatusFailed {
			continue
		}
		fmt.Fprintf(os.Stderr, "        $ %s\n", step.Command)
		fmt.Fprintf(os.Stderr, "        expected: %s\n", step.Expected)
		fmt.Fprintf(os.Stderr, "        actual:   %s\n", step.Actual)
	}
}

func printAlloyFailures(checks []core.AlloyCheckResult) {
	for _, c := range checks {
		if c.Status != core.StatusFailed {
			continue
		}
		path := strings.Join(c.ID.HeadingPath, " > ")
		fmt.Fprintf(os.Stderr, "  FAIL  %s  [alloy:%s#%s]\n", path, c.Model, c.Assertion)
		if c.Message != "" {
			fmt.Fprintf(os.Stderr, "        %s\n", c.Message)
		}
	}
}

func printBindings(report core.Report) {
	for _, doc := range report.Results {
		for _, c := range doc.Cases {
			if len(c.VisibleBindings) == 0 {
				continue
			}
			path := strings.Join(c.ID.HeadingPath, " > ")
			kind := c.Block + c.Check
			var pairs []string
			for _, b := range c.VisibleBindings {
				pairs = append(pairs, fmt.Sprintf("$%s=%v", b.Name, b.Value))
			}
			fmt.Fprintf(os.Stderr, "  BIND  %s  [%s]  %s\n", path, kind, strings.Join(pairs, ", "))
		}
	}
}

func printWarnings(report core.Report) {
	for _, doc := range report.Results {
		for _, w := range doc.Document.Warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", w)
		}
	}
}

func printDryRun(report core.Report) {
	for _, doc := range report.Results {
		fmt.Printf("spec: %s\n", doc.Document.RelativeTo)
		for _, c := range doc.Cases {
			kind := c.Block
			if c.Kind == core.CaseKindTableRow {
				kind = "check:" + c.Check
			}
			fmt.Printf("  case: %s [%s]\n", strings.Join(c.ID.HeadingPath, " > "), kind)
		}
		for _, c := range doc.AlloyChecks {
			fmt.Printf("  alloy: %s [%s#%s, scope=%s]\n", strings.Join(c.ID.HeadingPath, " > "), c.Model, c.Assertion, c.Scope)
		}
	}
	fmt.Printf("\ntotal: %d spec(s), %d case(s), %d alloy check(s)\n",
		report.Summary.SpecsTotal, report.Summary.CasesTotal, report.Summary.AlloyChecksTotal)
}
