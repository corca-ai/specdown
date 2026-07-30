package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"

	"github.com/corca-ai/specdown/internal/specdown/config"
	"github.com/corca-ai/specdown/internal/specdown/trace"
)

func (c command) traceCmd(args []string) error {
	fs := flag.NewFlagSet("trace", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	fs.Usage = func() {
		writeLine(c.stderr, "Usage: specdown trace [flags]")
		writeLine(c.stderr)
		writeLine(c.stderr, "Validate trace graph and output results.")
		writeLine(c.stderr)
		writeLine(c.stderr, "Flags:")
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
	if fs.NArg() > 0 {
		return fmt.Errorf("trace does not accept positional arguments")
	}

	cfg, configDir, err := c.loadConfig(fs, *configPath)
	if err != nil {
		return err
	}

	if cfg.Trace == nil {
		return fmt.Errorf("no trace configuration found in config")
	}

	graph, traceErrs := trace.Validate(configDir, cfg.Trace)

	if len(traceErrs) > 0 {
		for _, e := range traceErrs {
			writeLine(c.stderr, e.Error())
		}
		if *strict {
			return fmt.Errorf("trace validation failed with %d error(s)", len(traceErrs))
		}
	}

	switch *format {
	case "json":
		return c.traceOutputJSON(graph)
	case "dot":
		return c.traceOutputDOT(graph, cfg.Trace)
	case "matrix":
		return c.traceOutputMatrix(graph, cfg.Trace)
	default:
		return fmt.Errorf("unknown trace format %q (expected json, dot, or matrix)", *format)
	}
}

func (c command) traceOutputJSON(graph trace.Graph) error {
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
	_, err = fmt.Fprintln(c.stdout, string(data))
	return err
}

func (c command) traceOutputDOT(graph trace.Graph, cfg *config.TraceConfig) error {
	output := outputWriter{writer: c.stdout}
	output.line("digraph trace {")
	output.line("  rankdir=LR;")

	typeGroups := make(map[string][]string)
	for _, d := range graph.Documents {
		if d.Type != "" {
			typeGroups[d.Type] = append(typeGroups[d.Type], d.Path)
		}
	}
	for _, traceType := range cfg.Types {
		paths := typeGroups[traceType]
		if len(paths) == 0 {
			continue
		}
		output.format("  subgraph cluster_%s {\n", traceType)
		output.format("    label=%q;\n", traceType)
		for _, path := range paths {
			output.format("    %q;\n", path)
		}
		output.line("  }")
	}

	for _, edge := range graph.DirectEdges {
		output.format("  %q -> %q [label=%q];\n", edge.Source, edge.Target, edge.EdgeName)
	}
	for _, edge := range graph.TransitiveEdges {
		output.format("  %q -> %q [label=%q, style=dashed];\n", edge.Source, edge.Target, edge.EdgeName)
	}

	output.line("}")
	return output.err
}

func (c command) traceOutputMatrix(graph trace.Graph, _ *config.TraceConfig) error {
	var docs []string
	for _, doc := range graph.Documents {
		if doc.Type != "" {
			docs = append(docs, doc.Path)
		}
	}
	if len(docs) == 0 {
		_, err := fmt.Fprintln(c.stdout, "(no typed documents)")
		return err
	}

	edgeLookup := buildEdgeLookup(graph)

	maxLen := 0
	for _, doc := range docs {
		if len(doc) > maxLen {
			maxLen = len(doc)
		}
	}
	columnWidth := maxLen + 2

	output := outputWriter{writer: c.stdout}
	output.format("%-*s", columnWidth, "")
	for _, doc := range docs {
		output.format(" %-*s", columnWidth, doc)
	}
	output.line()

	for _, source := range docs {
		output.format("%-*s", columnWidth, source)
		for _, target := range docs {
			output.format(" %-*s", columnWidth, edgeLookup(source, target))
		}
		output.line()
	}
	return output.err
}

func buildEdgeLookup(graph trace.Graph) func(source, target string) string {
	directSet := make(map[string]string)
	transitiveSet := make(map[string]string)
	for _, edge := range graph.DirectEdges {
		directSet[edge.Source+"|"+edge.Target] = edge.EdgeName
	}
	for _, edge := range graph.TransitiveEdges {
		transitiveSet[edge.Source+"|"+edge.Target] = edge.EdgeName
	}
	return func(source, target string) string {
		key := source + "|" + target
		if edge, ok := directSet[key]; ok {
			return edge
		}
		if edge, ok := transitiveSet[key]; ok {
			return "(" + edge + ")"
		}
		return "."
	}
}
