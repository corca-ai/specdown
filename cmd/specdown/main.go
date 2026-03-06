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
	default:
		usage()
		os.Exit(2)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	configPath := fs.String("config", "specdown.json", "Path to specdown.json")
	outPath := fs.String("out", "", "Output HTML report path")

	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, configDir, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	report, err := engine.Run(configDir, cfg)
	if err != nil {
		return err
	}

	reportPath := resolveReportPath(configDir, cfg, *outPath)
	if err := writeArtifacts(report, reportPath); err != nil {
		return err
	}

	if report.Summary.SpecsFailed > 0 || report.Summary.CasesFailed > 0 {
		printFailures(report)
		fmt.Fprintf(os.Stderr, "\nFAIL %d spec(s), %d case(s), %d alloy check(s)\n", report.Summary.SpecsFailed, report.Summary.CasesFailed, report.Summary.AlloyChecksFailed)
		fmt.Fprintf(os.Stderr, "report: %s\n", reportPath)
		return fmt.Errorf("spec run failed")
	}

	fmt.Printf("PASS %d spec(s), %d case(s), %d alloy check(s)\n", report.Summary.SpecsPassed, report.Summary.CasesPassed, report.Summary.AlloyChecksPassed)
	fmt.Printf("report: %s\n", reportPath)
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  specdown run [-config specdown.json] [-out .artifacts/specdown/report.html]")
}

func writeArtifacts(report core.Report, reportPath string) error {
	if err := htmlreport.Write(report, reportPath); err != nil {
		return err
	}
	if err := jsonreport.Write(report, jsonReportPath(reportPath)); err != nil {
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
		for _, c := range doc.Cases {
			if c.Status != core.StatusFailed {
				continue
			}
			path := strings.Join(c.ID.HeadingPath, " > ")
			msg := c.Message
			if c.Actual != "" {
				msg = c.Actual
			}
			fmt.Fprintf(os.Stderr, "  FAIL  %s  [%s]\n", path, c.Block+c.Fixture)
			if msg != "" {
				fmt.Fprintf(os.Stderr, "        %s\n", msg)
			}
		}
		for _, c := range doc.AlloyChecks {
			if c.Status != core.StatusFailed {
				continue
			}
			path := strings.Join(c.ID.HeadingPath, " > ")
			msg := c.Message
			if c.Actual != "" {
				msg = c.Actual
			}
			fmt.Fprintf(os.Stderr, "  FAIL  %s  [alloy:%s#%s]\n", path, c.Model, c.Assertion)
			if msg != "" {
				fmt.Fprintf(os.Stderr, "        %s\n", msg)
			}
		}
	}
}
