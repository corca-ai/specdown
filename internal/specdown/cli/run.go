package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"time"

	"github.com/corca-ai/specdown/internal/specdown/alloy"
	"github.com/corca-ai/specdown/internal/specdown/config"
	"github.com/corca-ai/specdown/internal/specdown/core"
	"github.com/corca-ai/specdown/internal/specdown/engine"
	htmlreport "github.com/corca-ai/specdown/internal/specdown/reporter/html"
	jsonreport "github.com/corca-ai/specdown/internal/specdown/reporter/json"
)

//nolint:gocognit // CLI entry point with flag parsing
func (c command) run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	fs.Usage = func() {
		writeLine(c.stderr, "Usage: specdown run [flags]")
		writeLine(c.stderr)
		writeLine(c.stderr, "Execute spec files and generate HTML/JSON reports.")
		writeLine(c.stderr)
		writeLine(c.stderr, "Flags:")
		fs.PrintDefaults()
	}

	configPath := fs.String("config", "specdown.json", "Path to specdown.json")
	outPath := fs.String("out", "", "Output HTML report directory")
	filter := fs.String("filter", "", "Filter cases: heading substring, type:{code,table,expect,alloy}, block:<target>, check:<name>")
	jobs := fs.Int("jobs", 1, "Number of spec files to run in parallel")
	dryRun := fs.Bool("dry-run", false, "Parse and validate without executing")
	noReport := fs.Bool("no-report", false, "Execute specs without writing report artifacts")
	showBindings := fs.Bool("show-bindings", false, "Print resolved variable bindings for each case")
	quiet := fs.Bool("quiet", false, "Suppress progress output; show only final summary")
	maxFailures := fs.Int("max-failures", 0, "Stop after N unexpected failures (0 = unlimited)")
	noSetup := fs.Bool("no-setup", false, "Skip the global setup command")
	noTeardown := fs.Bool("no-teardown", false, "Skip the global teardown command")
	onlySetup := fs.Bool("setup", false, "Run only the global setup command, then exit")
	onlyTeardown := fs.Bool("teardown", false, "Run only the global teardown command, then exit")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("run does not accept positional arguments")
	}

	cfg, configDir, err := c.loadConfig(fs, *configPath)
	if err != nil {
		return err
	}

	opts := engine.RunOptions{
		Filter:       *filter,
		Jobs:         *jobs,
		DryRun:       *dryRun,
		MaxFailures:  *maxFailures,
		NoSetup:      *noSetup,
		NoTeardown:   *noTeardown,
		OnlySetup:    *onlySetup,
		OnlyTeardown: *onlyTeardown,
	}
	if !*quiet {
		opts.Progress = c.stdoutProgress()
	}

	runStart := time.Now()
	report, err := engine.RunContext(ctx, configDir, cfg, alloy.Runner{BaseDir: configDir, JarPath: cfg.Models.JarPath}, opts)
	elapsed := time.Since(runStart)
	runErr := err
	if runErr != nil && len(report.Results) == 0 && len(report.LifecycleEvents) == 0 {
		return err
	}

	if !*quiet {
		c.printLifecycleFailures(report)
		c.printWarnings(report)
		c.printTraceErrors(report)
	}

	if *dryRun {
		if *quiet {
			c.printDryRunSummary(report)
		} else {
			c.printDryRun(report)
		}
		if reportFailed(report) {
			return fmt.Errorf("spec run failed")
		}
		return runErr
	}

	if *showBindings && !*quiet {
		c.printBindings(report)
	}

	reportPath := ""
	if !*noReport {
		reportPath = resolveReportPath(configDir, cfg, *outPath)
		if err := c.writeArtifacts(report, reportPath, configDir, cfg); err != nil {
			return err
		}
	}
	if runErr != nil {
		if !*quiet && reportPath != "" {
			writeFormat(c.stderr, "report: %s\n", reportPath)
		}
		return runErr
	}

	if reportFailed(report) {
		writeFormat(c.stderr, "\n%s\n", failureSummary(report, elapsed))
		if !*quiet && reportPath != "" {
			writeFormat(c.stderr, "report: %s\n", reportPath)
		}
		return fmt.Errorf("spec run failed")
	}

	xfailSuffix := ""
	if report.Summary.CasesExpectedFail > 0 {
		xfailSuffix = fmt.Sprintf(", %d expected fail", report.Summary.CasesExpectedFail)
	}
	if _, err := fmt.Fprintf(c.stdout, "PASS %d spec(s), %d case(s)%s in %dms\n", report.Summary.SpecsTotal, report.Summary.CasesTotal, xfailSuffix, elapsed.Milliseconds()); err != nil {
		return err
	}
	if !*quiet && reportPath != "" {
		if _, err := fmt.Fprintf(c.stdout, "report: %s\n", reportPath); err != nil {
			return err
		}
	}
	return nil
}

func failureSummary(report core.Report, elapsed time.Duration) string {
	lifecycleSuffix := ""
	if report.Summary.LifecycleFailed > 0 {
		lifecycleSuffix = fmt.Sprintf(", %d lifecycle failure(s)", report.Summary.LifecycleFailed)
	}
	xfailSuffix := ""
	if report.Summary.CasesExpectedFail > 0 {
		xfailSuffix = fmt.Sprintf(", %d expected", report.Summary.CasesExpectedFail)
	}
	skippedSuffix := ""
	if report.Summary.SpecsSkipped > 0 {
		skippedSuffix = fmt.Sprintf(", %d spec(s) skipped", report.Summary.SpecsSkipped)
	}
	if report.Summary.CasesSkipped > 0 {
		skippedSuffix += fmt.Sprintf(", %d case(s) skipped", report.Summary.CasesSkipped)
	}
	return fmt.Sprintf(
		"FAIL %d spec(s), %d case(s)%s%s%s in %dms",
		report.Summary.SpecsFailed,
		report.Summary.CasesFailed,
		lifecycleSuffix,
		xfailSuffix,
		skippedSuffix,
		elapsed.Milliseconds(),
	)
}

func reportFailed(report core.Report) bool {
	return report.Summary.SpecsFailed > 0 ||
		report.Summary.LifecycleFailed > 0 ||
		report.Summary.TraceErrorCount > 0
}

func (c command) writeArtifacts(report core.Report, reportPath, baseDir string, cfg config.Config) error {
	if reportPath != "" {
		warnings, err := htmlreport.Write(report, reportPath, cfg.TOC)
		for _, warning := range warnings {
			writeFormat(c.stderr, "specdown: warning: %s\n", warning)
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
