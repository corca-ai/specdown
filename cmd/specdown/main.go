package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"specdown/internal/specdown/config"
	"specdown/internal/specdown/engine"
	htmlreport "specdown/internal/specdown/reporter/html"
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

	reportPath := *outPath
	if reportPath == "" {
		reportPath = cfg.HTMLReportOutFile()
	}
	if reportPath == "" {
		reportPath = ".artifacts/specdown/report.html"
	}
	reportPath = resolvePath(configDir, reportPath)

	if err := htmlreport.Write(report, reportPath); err != nil {
		return err
	}

	if report.Summary.SpecsFailed > 0 || report.Summary.CasesFailed > 0 {
		fmt.Printf("FAIL %d spec(s), %d case(s)\n", report.Summary.SpecsFailed, report.Summary.CasesFailed)
		fmt.Printf("report: %s\n", reportPath)
		return fmt.Errorf("spec run failed")
	}

	fmt.Printf("PASS %d spec(s), %d case(s)\n", report.Summary.SpecsPassed, report.Summary.CasesPassed)
	fmt.Printf("report: %s\n", reportPath)
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  specdown run [-config specdown.json] [-out .artifacts/specdown/report.html]")
}

func resolvePath(baseDir string, value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Clean(filepath.Join(baseDir, value))
}
