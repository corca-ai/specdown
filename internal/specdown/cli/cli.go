package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/corca-ai/specdown/internal/specdown/config"
)

// Options contains process-level values supplied by the executable wrapper.
type Options struct {
	Version       string
	SkillSpecdown string
	WorkingDir    string
}

type command struct {
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	version    string
	skill      string
	workingDir string
	filesystem skillFilesystem
}

type recordingWriter struct {
	writer io.Writer
	err    error
}

func (writer *recordingWriter) Write(data []byte) (int, error) {
	written, err := writer.writer.Write(data)
	if err == nil && written != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil && writer.err == nil {
		writer.err = err
	}
	return written, err
}

func writeLine(writer io.Writer, values ...any) {
	_, _ = fmt.Fprintln(writer, values...)
}

func writeFormat(writer io.Writer, format string, values ...any) {
	_, _ = fmt.Fprintf(writer, format, values...)
}

type outputWriter struct {
	writer io.Writer
	err    error
}

func (output *outputWriter) line(values ...any) {
	if output.err != nil {
		return
	}
	_, output.err = fmt.Fprintln(output.writer, values...)
}

func (output *outputWriter) format(format string, values ...any) {
	if output.err != nil {
		return
	}
	_, output.err = fmt.Fprintf(output.writer, format, values...)
}

// Execute runs one CLI invocation and returns its process exit status.
func Execute(args []string, stdin io.Reader, stdout, stderr io.Writer, options Options) int {
	workingDir := options.WorkingDir
	if workingDir == "" {
		var err error
		workingDir, err = os.Getwd()
		if err != nil {
			writeFormat(stderr, "specdown: determine working directory: %v\n", err)
			return 1
		}
	}
	recordedStdout := &recordingWriter{writer: stdout}
	recordedStderr := &recordingWriter{writer: stderr}
	cmd := command{
		stdin:      stdin,
		stdout:     recordedStdout,
		stderr:     recordedStderr,
		version:    options.Version,
		skill:      options.SkillSpecdown,
		workingDir: workingDir,
		filesystem: osSkillFilesystem{},
	}
	status := cmd.execute(args)
	if status == 0 && (recordedStdout.err != nil || recordedStderr.err != nil) {
		return 1
	}
	return status
}

func (c command) execute(args []string) int {
	if len(args) == 0 {
		c.usage()
		return 2
	}

	var err error
	switch args[0] {
	case "help", "--help", "-help", "-h":
		c.usage()
	case "init":
		err = c.initCmd(args[1:])
	case "run":
		ctx, stop := interruptContext()
		err = c.run(ctx, args[1:])
		stop()
	case "trace":
		err = c.traceCmd(args[1:])
	case "alloy":
		err = c.alloyCmd(args[1:])
	case "install":
		err = c.installSkillsCmd(args[1:])
	case "version", "--version", "-version":
		err = c.versionCmd(args[1:])
	default:
		return c.unknownCmd(args)
	}
	if err != nil {
		writeFormat(c.stderr, "specdown: %v\n", err)
		return 1
	}
	return 0
}

func interruptContext() (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ctx.Done()
		stop()
	}()
	return ctx, stop
}

func (c command) unknownCmd(args []string) int {
	for _, arg := range args {
		if arg == "--version" || arg == "-version" {
			writeLine(c.stdout, c.version)
			return 0
		}
	}
	writeFormat(c.stderr, "specdown: unknown command %q\n\n", args[0])
	c.usage()
	return 2
}

func (c command) versionCmd(args []string) error {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	fs.Usage = func() {
		writeLine(c.stderr, "Usage: specdown version")
		writeLine(c.stderr)
		writeLine(c.stderr, "Print the specdown version.")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("version does not accept positional arguments")
	}
	_, err := fmt.Fprintln(c.stdout, c.version)
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

func (c command) usage() {
	writeLine(c.stderr, "specdown — Markdown-first executable specifications")
	writeLine(c.stderr)
	writeLine(c.stderr, "Commands:")
	writeLine(c.stderr, "  init            Scaffold a new project (creates specdown.json and example specs)")
	writeLine(c.stderr, "  run             Execute specs and generate HTML/JSON reports")
	writeLine(c.stderr, "  trace           Validate trace graph and output results")
	writeLine(c.stderr, "  install skills  Install Claude Code skills for this project")
	writeLine(c.stderr, "  alloy explore   Run Alloy models and show instances")
	writeLine(c.stderr, "  alloy dump      Export embedded Alloy models as .als files")
	writeLine(c.stderr, "  version         Print the specdown version")
	writeLine(c.stderr)
	writeLine(c.stderr, "Run 'specdown <command> --help' for details on a specific command.")
}

func (c command) loadConfig(fs *flag.FlagSet, configPath string) (config.Config, string, error) {
	explicit := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "config" {
			explicit = true
		}
	})
	configPath = resolvePath(c.workingDir, configPath)
	if explicit {
		return config.Load(configPath)
	}
	return config.LoadOrDefault(configPath)
}

// PrependExecutableToPath ensures child processes resolve the running binary.
func PrependExecutableToPath() {
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
