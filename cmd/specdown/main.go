package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "embed"

	specdown "github.com/corca-ai/specdown"
	"github.com/corca-ai/specdown/internal/specdown/alloy"
	"github.com/corca-ai/specdown/internal/specdown/config"
	"github.com/corca-ai/specdown/internal/specdown/core"
	"github.com/corca-ai/specdown/internal/specdown/engine"
	htmlreport "github.com/corca-ai/specdown/internal/specdown/reporter/html"
	jsonreport "github.com/corca-ai/specdown/internal/specdown/reporter/json"
	"github.com/corca-ai/specdown/internal/specdown/trace"
)

var version = "dev"

//go:embed skills/specdown/SKILL.md
var skillSpecdown string

func main() {
	prependSelfToPath()

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
	case "trace":
		err = traceCmd(os.Args[2:])
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

func hasHelpFlag(args []string) bool {
	for _, a := range args {
		if a == "-help" || a == "--help" || a == "-h" {
			return true
		}
	}
	return false
}

//nolint:gocognit // CLI entry point with flag parsing
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
	outPath := fs.String("out", "", "Output HTML report directory")
	filter := fs.String("filter", "", "Filter cases: heading substring, type:{code,table,expect,alloy}, block:<target>, check:<name>")
	jobs := fs.Int("jobs", 1, "Number of spec files to run in parallel")
	dryRun := fs.Bool("dry-run", false, "Parse and validate without executing")
	showBindings := fs.Bool("show-bindings", false, "Print resolved variable bindings for each case")
	quiet := fs.Bool("quiet", false, "Suppress progress output; show only final summary")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	cfg, configDir, err := config.LoadOrDefault(*configPath)
	if err != nil {
		return err
	}

	opts := engine.RunOptions{
		Filter: *filter,
		Jobs:   *jobs,
		DryRun: *dryRun,
	}

	runStart := time.Now()
	report, err := engine.Run(configDir, cfg, alloy.Runner{BaseDir: configDir}, opts)
	elapsed := time.Since(runStart)
	if err != nil {
		return err
	}

	if !*quiet {
		printWarnings(report)
		printTraceErrors(report)
	}

	if *dryRun {
		if !*quiet {
			printDryRun(report)
		}
		return nil
	}

	if !*quiet {
		printResults(report)
	}

	if *showBindings && !*quiet {
		printBindings(report)
	}

	reportPath := resolveReportPath(configDir, cfg, *outPath)
	if err := writeArtifacts(report, reportPath, configDir, cfg); err != nil {
		return err
	}

	if report.Summary.SpecsFailed > 0 || report.Summary.TraceErrorCount > 0 {
		if !*quiet {
			printFailures(report)
		}
		xfailSuffix := ""
		if report.Summary.CasesExpectedFail > 0 {
			xfailSuffix = fmt.Sprintf(", %d expected", report.Summary.CasesExpectedFail)
		}
		if !*quiet {
			fmt.Fprintf(os.Stderr, "\nFAIL %d spec(s), %d case(s)%s in %dms\n", report.Summary.SpecsFailed, report.Summary.CasesFailed, xfailSuffix, elapsed.Milliseconds())
			if reportPath != "" {
				fmt.Fprintf(os.Stderr, "report: %s\n", reportPath)
			}
		}
		return fmt.Errorf("spec run failed")
	}

	if !*quiet {
		xfailSuffix := ""
		if report.Summary.CasesExpectedFail > 0 {
			xfailSuffix = fmt.Sprintf(", %d expected fail", report.Summary.CasesExpectedFail)
		}
		fmt.Printf("PASS %d spec(s), %d case(s)%s in %dms\n", report.Summary.SpecsTotal, report.Summary.CasesTotal, xfailSuffix, elapsed.Milliseconds())
		if reportPath != "" {
			fmt.Printf("report: %s\n", reportPath)
		}
	}
	return nil
}

func traceCmd(args []string) error {
	fs := flag.NewFlagSet("trace", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: specdown trace [flags]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Validate trace graph and output results.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Flags:")
		fs.PrintDefaults()
	}

	configPath := fs.String("config", "specdown.json", "Path to specdown.json")
	format := fs.String("format", "json", "Output format: json, dot, matrix")
	strict := fs.Bool("strict", false, "Suppress output when validation errors exist")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	cfg, configDir, err := config.LoadOrDefault(*configPath)
	if err != nil {
		return err
	}

	if cfg.Trace == nil {
		return fmt.Errorf("no trace configuration found in config")
	}

	graph, traceErrs := trace.Validate(configDir, cfg.Trace)

	if len(traceErrs) > 0 {
		for _, e := range traceErrs {
			fmt.Fprintln(os.Stderr, e.Error())
		}
		if *strict {
			return fmt.Errorf("trace validation failed with %d error(s)", len(traceErrs))
		}
	}

	switch *format {
	case "json":
		return traceOutputJSON(graph)
	case "dot":
		return traceOutputDOT(graph, cfg.Trace)
	case "matrix":
		return traceOutputMatrix(graph, cfg.Trace)
	default:
		return fmt.Errorf("unknown trace format %q (expected json, dot, or matrix)", *format)
	}
}

func traceOutputJSON(graph trace.Graph) error {
	type jsonEdge struct {
		Source   string `json:"source"`
		Target   string `json:"target"`
		EdgeName string `json:"edge"`
	}
	type jsonDoc struct {
		Path string `json:"path"`
		Type string `json:"type,omitempty"`
	}
	type jsonOutput struct {
		Documents       []jsonDoc  `json:"documents"`
		DirectEdges     []jsonEdge `json:"directEdges"`
		TransitiveEdges []jsonEdge `json:"transitiveEdges,omitempty"`
	}

	out := jsonOutput{}
	for _, d := range graph.Documents {
		if d.Type != "" {
			out.Documents = append(out.Documents, jsonDoc{Path: d.Path, Type: d.Type})
		}
	}
	for _, e := range graph.DirectEdges {
		out.DirectEdges = append(out.DirectEdges, jsonEdge{Source: e.Source, Target: e.Target, EdgeName: e.EdgeName})
	}
	for _, e := range graph.TransitiveEdges {
		out.TransitiveEdges = append(out.TransitiveEdges, jsonEdge{Source: e.Source, Target: e.Target, EdgeName: e.EdgeName})
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func traceOutputDOT(graph trace.Graph, cfg *config.TraceConfig) error {
	fmt.Println("digraph trace {")
	fmt.Println("  rankdir=LR;")

	// Group documents by type
	typeGroups := make(map[string][]string)
	for _, d := range graph.Documents {
		if d.Type != "" {
			typeGroups[d.Type] = append(typeGroups[d.Type], d.Path)
		}
	}
	for _, t := range cfg.Types {
		paths := typeGroups[t]
		if len(paths) == 0 {
			continue
		}
		fmt.Printf("  subgraph cluster_%s {\n", t)
		fmt.Printf("    label=%q;\n", t)
		for _, p := range paths {
			fmt.Printf("    %q;\n", p)
		}
		fmt.Println("  }")
	}

	for _, e := range graph.DirectEdges {
		fmt.Printf("  %q -> %q [label=%q];\n", e.Source, e.Target, e.EdgeName)
	}
	for _, e := range graph.TransitiveEdges {
		fmt.Printf("  %q -> %q [label=%q, style=dashed];\n", e.Source, e.Target, e.EdgeName)
	}

	fmt.Println("}")
	return nil
}

func traceOutputMatrix(graph trace.Graph, _ *config.TraceConfig) error {
	var docs []string
	for _, d := range graph.Documents {
		if d.Type != "" {
			docs = append(docs, d.Path)
		}
	}
	if len(docs) == 0 {
		fmt.Println("(no typed documents)")
		return nil
	}

	edgeLookup := buildEdgeLookup(graph)

	maxLen := 0
	for _, d := range docs {
		if len(d) > maxLen {
			maxLen = len(d)
		}
	}
	col := maxLen + 2

	fmt.Printf("%-*s", col, "")
	for _, d := range docs {
		fmt.Printf(" %-*s", col, d)
	}
	fmt.Println()

	for _, src := range docs {
		fmt.Printf("%-*s", col, src)
		for _, tgt := range docs {
			fmt.Printf(" %-*s", col, edgeLookup(src, tgt))
		}
		fmt.Println()
	}
	return nil
}

func buildEdgeLookup(graph trace.Graph) func(src, tgt string) string {
	directSet := make(map[string]string)
	transitiveSet := make(map[string]string)
	for _, e := range graph.DirectEdges {
		directSet[e.Source+"|"+e.Target] = e.EdgeName
	}
	for _, e := range graph.TransitiveEdges {
		transitiveSet[e.Source+"|"+e.Target] = e.EdgeName
	}
	return func(src, tgt string) string {
		key := src + "|" + tgt
		if edge, ok := directSet[key]; ok {
			return edge
		}
		if edge, ok := transitiveSet[key]; ok {
			return "(" + edge + ")"
		}
		return "."
	}
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

	cfg, configDir, err := config.LoadOrDefault(*configPath)
	if err != nil {
		return err
	}

	paths, err := engine.DumpModels(configDir, cfg, alloy.Runner{BaseDir: configDir})
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
	fmt.Fprintln(os.Stderr, "  trace           Validate trace graph and output results")
	fmt.Fprintln(os.Stderr, "  install skills  Install Claude Code skills for this project")
	fmt.Fprintln(os.Stderr, "  alloy dump      Export embedded Alloy models as .als files")
	fmt.Fprintln(os.Stderr, "  version         Print the specdown version")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Run 'specdown <command> --help' for details on a specific command.")
}

func installSkillsCmd(args []string) error {
	if len(args) == 0 || hasHelpFlag(args) {
		fmt.Fprintln(os.Stderr, "Usage: specdown install skills [--overwrite]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Install Claude Code skills for this project.")
		fmt.Fprintln(os.Stderr, "Creates .claude/skills/specdown/SKILL.md in the current directory.")
		fmt.Fprintln(os.Stderr, "Use --overwrite to replace existing files.")
		return nil
	}
	if args[0] != "skills" {
		return fmt.Errorf("unknown install target %q\nhint: run 'specdown install --help'", args[0])
	}

	overwrite := false
	for _, a := range args[1:] {
		if a == "--overwrite" {
			overwrite = true
		}
	}

	dir := filepath.Join(".claude", "skills", "specdown")
	dest := filepath.Join(dir, "SKILL.md")

	if _, err := os.Stat(dest); err == nil && !overwrite {
		return fmt.Errorf("%s already exists\nhint: use --overwrite to replace existing files", dest)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := []struct{ name, content string }{
		{"SKILL.md", skillSpecdown},
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
	}
	for _, f := range files {
		p := filepath.Join(dir, f.name)
		if err := os.WriteFile(p, []byte(f.content), 0o644); err != nil {
			return err
		}
		fmt.Printf("Created %s\n", p)
	}
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
  "models": { "builtin": "alloy" },
  "reporters": [
    { "builtin": "html", "outFile": "specs/report" },
    { "builtin": "json", "outFile": "specs/report.json" }
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

func writeArtifacts(report core.Report, reportPath, baseDir string, cfg config.Config) error {
	if reportPath != "" {
		warnings, err := htmlreport.Write(report, reportPath, cfg.TOC)
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "specdown: warning: %s\n", w)
		}
		if err != nil {
			return err
		}
	}
	jsonPath := cfg.JSONReportOutFile()
	if jsonPath == "" {
		if reportPath != "" {
			jsonPath = jsonReportPath(reportPath)
		}
	} else {
		jsonPath = resolvePath(baseDir, jsonPath)
	}
	if jsonPath != "" {
		if err := jsonreport.Write(report, jsonPath); err != nil {
			return err
		}
	}
	return nil
}

func resolveReportPath(baseDir string, cfg config.Config, requested string) string {
	reportPath := requested
	if reportPath == "" {
		reportPath = cfg.HTMLReportOutFile()
	}
	if reportPath == "" {
		return ""
	}
	return resolvePath(baseDir, reportPath)
}

func jsonReportPath(htmlReportDir string) string {
	return filepath.Join(htmlReportDir, "report.json")
}

func resolvePath(baseDir, value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Clean(filepath.Join(baseDir, value))
}

func printFailures(report core.Report) {
	for i := range report.Results {
		printCaseFailures(report.Results[i].Cases)
	}
}

func printCaseFailures(cases []core.CaseResult) {
	for i := range cases {
		if cases[i].Status != core.StatusFailed {
			continue
		}
		printCaseFailure(cases[i])
	}
}

func printCaseFailure(c core.CaseResult) {
	path := strings.Join(c.ID.HeadingPath, " > ")
	kind := c.Block + c.Check
	if c.Kind == core.CaseKindAlloy {
		kind = "alloy:" + c.Model + "#" + c.Assertion
	}
	label := ""
	if c.Kind == core.CaseKindTableRow && c.RowNumber > 0 {
		label = fmt.Sprintf(" row %d", c.RowNumber)
		if c.Label != "" {
			label = fmt.Sprintf(" row %d %q", c.RowNumber, c.Label)
		}
	}
	tag := "FAIL"
	if c.ExpectFail {
		tag = "XFAIL"
	}
	fmt.Fprintf(os.Stderr, "  %s  %s  [%s]%s\n", tag, path, kind, label)
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

func printBindings(report core.Report) {
	for i := range report.Results {
		for j := range report.Results[i].Cases {
			c := report.Results[i].Cases[j]
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

func printTraceErrors(report core.Report) {
	for _, e := range report.TraceErrors {
		fmt.Fprintf(os.Stderr, "trace: %s\n", e)
	}
}

func printWarnings(report core.Report) {
	for i := range report.Results {
		for _, w := range report.Results[i].Document.Warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", w)
		}
	}
}

func printResults(report core.Report) {
	for i := range report.Results {
		fmt.Printf("spec: %s\n", report.Results[i].Document.RelativeTo)
		for j := range report.Results[i].Cases {
			printCaseResult(report.Results[i].Cases[j])
		}
	}
}

func caseTag(status core.Status, expectFail bool) string {
	if status == core.StatusFailed {
		if expectFail {
			return "XFAIL"
		}
		return "FAIL"
	}
	return "PASS"
}

func printCaseResult(c core.CaseResult) {
	tag := caseTag(c.Status, c.ExpectFail)
	kind := c.Block + c.Check
	if c.Kind == core.CaseKindAlloy {
		kind = "alloy:" + c.Model + "#" + c.Assertion
	}
	label := ""
	if c.Kind == core.CaseKindTableRow && c.RowNumber > 0 {
		label = fmt.Sprintf(" row %d", c.RowNumber)
	}
	fmt.Printf("  %s  %s  [%s]%s  (%dms)\n", tag, strings.Join(c.ID.HeadingPath, " > "), kind, label, c.DurationMs)
}

// prependSelfToPath ensures that child processes (adapters, setup/teardown
// commands, and shell blocks that invoke specdown recursively) resolve the
// same binary that is currently running.  Without this, a stale "specdown"
// earlier on PATH can silently take precedence.
func prependSelfToPath() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Dir(exe)
	path := os.Getenv("PATH")
	if path == "" {
		_ = os.Setenv("PATH", dir)
		return
	}
	// Skip if already at the front.
	if strings.HasPrefix(path, dir+string(os.PathListSeparator)) || path == dir {
		return
	}
	_ = os.Setenv("PATH", dir+string(os.PathListSeparator)+path)
}

func printDryRun(report core.Report) {
	for i := range report.Results {
		fmt.Printf("spec: %s\n", report.Results[i].Document.RelativeTo)
		for j := range report.Results[i].Cases {
			c := report.Results[i].Cases[j]
			if c.Kind == core.CaseKindAlloy {
				fmt.Printf("  alloy: %s [%s#%s, scope=%s]\n", strings.Join(c.ID.HeadingPath, " > "), c.Model, c.Assertion, c.Scope)
				continue
			}
			kind := c.Block
			if c.Kind == core.CaseKindTableRow {
				kind = "check:" + c.Check
			}
			fmt.Printf("  case: %s [%s]\n", strings.Join(c.ID.HeadingPath, " > "), kind)
		}
	}
	fmt.Printf("\ntotal: %d spec(s), %d case(s)\n",
		report.Summary.SpecsTotal, report.Summary.CasesTotal)
}
