package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/corca-ai/specdown/internal/specdown/alloy"
	"github.com/corca-ai/specdown/internal/specdown/engine"
)

func (c command) alloyCmd(args []string) error {
	if len(args) == 0 || (len(args) == 1 && hasHelpFlag(args)) {
		_, err := fmt.Fprint(c.stderr, `Usage: specdown alloy <subcommand>

Subcommands:
  explore  Run Alloy models and show instances
  dump     Export embedded Alloy models as .als files
`)
		return err
	}

	switch args[0] {
	case "dump":
		return c.alloyDump(args[1:])
	case "explore":
		ctx, stop := interruptContext()
		defer stop()
		return c.alloyExplore(ctx, args[1:])
	default:
		return fmt.Errorf("unknown alloy subcommand %q\nhint: run 'specdown alloy --help' for available subcommands", args[0])
	}
}

func (c command) alloyExplore(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("alloy explore", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	fs.Usage = func() {
		writeLine(c.stderr, "Usage: specdown alloy explore [flags]")
		writeLine(c.stderr)
		writeLine(c.stderr, "Run embedded Alloy models and display instance-level results.")
		writeLine(c.stderr, "Only Alloy commands are executed — shell blocks and check tables are skipped.")
		writeLine(c.stderr)
		writeLine(c.stderr, "Flags:")
		fs.PrintDefaults()
	}
	configPath := fs.String("config", "specdown.json", "Path to specdown.json")
	filter := fs.String("filter", "", "Filter by spec file path substring")
	modelFilter := fs.String("model", "", "Filter by model name")
	repeat := fs.Int("repeat", 1, "Number of solutions to find per command")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("alloy explore does not accept positional arguments")
	}

	cfg, configDir, err := c.loadConfig(fs, *configPath)
	if err != nil {
		return err
	}

	opts := alloy.ExploreOptions{Repeat: *repeat}
	resultsByDoc, err := engine.ExploreModelsContext(ctx, configDir, cfg, alloy.Runner{BaseDir: configDir, JarPath: cfg.Models.JarPath}, *filter, opts)
	if err != nil {
		return err
	}

	if len(resultsByDoc) == 0 {
		_, err := fmt.Fprintln(c.stdout, "no Alloy models found")
		return err
	}

	return c.printExploreResults(resultsByDoc, *modelFilter)
}

func (c command) printExploreResults(resultsByDoc map[string][]alloy.ExploreModelResult, modelFilter string) error {
	var docPaths []string
	for path := range resultsByDoc {
		docPaths = append(docPaths, path)
	}
	sort.Strings(docPaths)

	for _, docPath := range docPaths {
		if _, err := fmt.Fprintf(c.stdout, "spec: %s\n", docPath); err != nil {
			return err
		}
		for _, result := range resultsByDoc[docPath] {
			if modelFilter != "" && result.Model != modelFilter {
				continue
			}
			if err := c.printModelExploreResult(result); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(c.stdout); err != nil {
			return err
		}
	}
	return nil
}

func (c command) printModelExploreResult(result alloy.ExploreModelResult) error {
	if _, err := fmt.Fprintf(c.stdout, "\n  model: %s\n", result.Model); err != nil {
		return err
	}
	if err := c.printExploreSigs(result.Sigs); err != nil {
		return err
	}
	return c.printExploreCommands(result.Commands)
}

func (c command) printExploreSigs(sigs string) error {
	if sigs == "" {
		return nil
	}
	if _, err := fmt.Fprintln(c.stdout, "\n    sigs:"); err != nil {
		return err
	}
	for _, line := range strings.Split(sigs, "\n") {
		if _, err := fmt.Fprintf(c.stdout, "      %s\n", line); err != nil {
			return err
		}
	}
	return nil
}

func (c command) printExploreCommands(results []alloy.ExploreResult) error {
	for _, commandResult := range results {
		tag := "✓"
		if !commandResult.Ok {
			tag = "✗"
		}
		if _, err := fmt.Fprintf(c.stdout, "\n    %s %s\n", tag, commandResult.Command); err != nil {
			return err
		}
		for _, line := range strings.Split(commandResult.Summary, "\n") {
			if _, err := fmt.Fprintf(c.stdout, "      %s\n", line); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c command) alloyDump(args []string) error {
	fs := flag.NewFlagSet("alloy dump", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	fs.Usage = func() {
		writeLine(c.stderr, "Usage: specdown alloy dump [flags]")
		writeLine(c.stderr)
		writeLine(c.stderr, "Export embedded Alloy models from spec files as .als files.")
		writeLine(c.stderr)
		writeLine(c.stderr, "Flags:")
		fs.PrintDefaults()
	}
	configPath := fs.String("config", "specdown.json", "Path to specdown.json")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("alloy dump does not accept positional arguments")
	}

	cfg, configDir, err := c.loadConfig(fs, *configPath)
	if err != nil {
		return err
	}

	paths, err := engine.DumpModels(configDir, cfg, alloy.Runner{BaseDir: configDir, JarPath: cfg.Models.JarPath})
	if err != nil {
		return err
	}

	for _, path := range paths {
		if _, err := fmt.Fprintln(c.stdout, path); err != nil {
			return err
		}
	}
	return nil
}
