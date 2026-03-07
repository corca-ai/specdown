package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"specdown/internal/specdown/config"
	"specdown/internal/specdown/core"
	"specdown/internal/specdown/engine"
	htmlreport "specdown/internal/specdown/reporter/html"
	jsonreport "specdown/internal/specdown/reporter/json"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "run":
		if err := run(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "specdown: %v\n", err)
			os.Exit(1)
		}
	case "alloy":
		if err := alloyCmd(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "specdown: %v\n", err)
			os.Exit(1)
		}
	case "version", "--version", "-version":
		fmt.Println(version)
	default:
		// Check for --version anywhere
		for _, arg := range os.Args[1:] {
			if arg == "--version" || arg == "-version" {
				fmt.Println(version)
				return
			}
		}
		usage()
		os.Exit(2)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	configPath := fs.String("config", "specdown.json", "Path to specdown.json")
	outPath := fs.String("out", "", "Output HTML report path")
	filter := fs.String("filter", "", "Run only cases whose heading path contains this string")
	jobs := fs.Int("jobs", 1, "Number of spec files to run in parallel")
	dryRun := fs.Bool("dry-run", false, "Parse and validate without executing")

	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, configDir, err := config.Load(*configPath)
	if err != nil {
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

	if *dryRun {
		printDryRun(report)
		return nil
	}

	reportPath := resolveReportPath(configDir, cfg, *outPath)
	if err := writeArtifacts(report, reportPath, cfg); err != nil {
		return err
	}

	if report.Summary.SpecsFailed > 0 || report.Summary.CasesFailed > 0 || report.Summary.AlloyChecksFailed > 0 {
		printFailures(report)
		fmt.Fprintf(os.Stderr, "\nFAIL %d spec(s), %d case(s), %d alloy check(s)\n", report.Summary.SpecsFailed, report.Summary.CasesFailed, report.Summary.AlloyChecksFailed)
		fmt.Fprintf(os.Stderr, "report: %s\n", reportPath)
		return fmt.Errorf("spec run failed")
	}

	fmt.Printf("PASS %d spec(s), %d case(s), %d alloy check(s)\n", report.Summary.SpecsPassed, report.Summary.CasesPassed, report.Summary.AlloyChecksPassed)
	fmt.Printf("report: %s\n", reportPath)
	return nil
}

func alloyCmd(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: specdown alloy dump [-config specdown.json]")
	}

	switch args[0] {
	case "dump":
		return alloyDump(args[1:])
	default:
		return fmt.Errorf("unknown alloy subcommand %q", args[0])
	}
}

func alloyDump(args []string) error {
	fs := flag.NewFlagSet("alloy dump", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "specdown.json", "Path to specdown.json")
	if err := fs.Parse(args); err != nil {
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
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  specdown run [-config specdown.json] [-out report.html] [-filter pattern] [-jobs N] [-dry-run]")
	fmt.Fprintln(os.Stderr, "  specdown alloy dump [-config specdown.json]")
	fmt.Fprintln(os.Stderr, "  specdown version")
}

func writeArtifacts(report core.Report, reportPath string, cfg config.Config) error {
	if err := htmlreport.Write(report, cfg.Title, reportPath); err != nil {
		return err
	}
	jsonPath := cfg.JSONReportOutFile()
	if jsonPath == "" {
		jsonPath = jsonReportPath(reportPath)
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
		path := strings.Join(c.ID.HeadingPath, " > ")
		fmt.Fprintf(os.Stderr, "  FAIL  %s  [%s]\n", path, c.Block+c.Fixture)
		if c.Message != "" {
			fmt.Fprintf(os.Stderr, "        %s\n", c.Message)
		}
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

func printDryRun(report core.Report) {
	for _, doc := range report.Results {
		fmt.Printf("spec: %s\n", doc.Document.RelativeTo)
		for _, c := range doc.Cases {
			kind := c.Block
			if c.Kind == core.CaseKindTableRow {
				kind = "fixture:" + c.Fixture
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
