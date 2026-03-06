package main

import (
	"flag"
	"fmt"
	"os"

	"specdown/internal/specdown/core"
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

	specRoot := fs.String("spec-root", "specs", "Directory containing .spec.md files")
	outPath := fs.String("out", ".artifacts/specdown/report.html", "Output HTML report path")

	if err := fs.Parse(args); err != nil {
		return err
	}

	report, err := core.Run(*specRoot)
	if err != nil {
		return err
	}

	if err := htmlreport.Write(report, *outPath); err != nil {
		return err
	}

	if report.Summary.SpecsFailed > 0 || report.Summary.CasesFailed > 0 {
		fmt.Printf("FAIL %d spec(s), %d case(s)\n", report.Summary.SpecsFailed, report.Summary.CasesFailed)
		fmt.Printf("report: %s\n", *outPath)
		return fmt.Errorf("spec run failed")
	}

	fmt.Printf("PASS %d spec(s), %d case(s)\n", report.Summary.SpecsPassed, report.Summary.CasesPassed)
	fmt.Printf("report: %s\n", *outPath)
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  specdown run [-spec-root specs] [-out .artifacts/specdown/report.html]")
}
