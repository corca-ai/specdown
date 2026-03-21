package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DefaultTimeoutMsec is the global default timeout applied to adapter requests
// when no per-document frontmatter timeout is set. 30 seconds.
const DefaultTimeoutMsec = 30_000

type Config struct {
	Entry             string          `json:"entry"`
	Adapters          []AdapterConfig `json:"adapters"`
	Models            ModelConfig     `json:"models"`
	Reporters         []Reporter      `json:"reporters"`
	IgnorePrefixes    []string        `json:"ignorePrefixes,omitempty"`
	Trace             *TraceConfig    `json:"trace,omitempty"`
	TOC               []TOCEntry      `json:"toc,omitempty"`
	Setup             string          `json:"setup,omitempty"`
	Teardown          string          `json:"teardown,omitempty"`
	DefaultTimeoutPtr *int            `json:"defaultTimeoutMsec,omitempty"`
}

// TOCEntry represents a single item in the toc config array.
// It is either a standalone document reference (Doc is set) or a named group (Group and Docs are set).
type TOCEntry struct {
	Group string   // non-empty for group entries
	Docs  []string // document paths within a group
	Doc   string   // non-empty for standalone doc references
}

func (e *TOCEntry) UnmarshalJSON(data []byte) error {
	// Try string first (standalone doc reference).
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		e.Doc = s
		return nil
	}
	// Try object (group).
	var obj struct {
		Group string   `json:"group"`
		Docs  []string `json:"docs"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("toc entry must be a string or {\"group\":..., \"docs\":[...]}: %w", err)
	}
	if obj.Group == "" {
		return fmt.Errorf("toc group entry must have a non-empty \"group\" field")
	}
	if len(obj.Docs) == 0 {
		return fmt.Errorf("toc group %q must have at least one doc", obj.Group)
	}
	e.Group = obj.Group
	e.Docs = obj.Docs
	return nil
}

type TraceConfig struct {
	Types  []string             `json:"types"`
	Ignore []string             `json:"ignore,omitempty"`
	Edges  map[string]TraceEdge `json:"edges"`
}

type TraceEdge struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Count      string `json:"count,omitempty"`
	Acyclic    bool   `json:"acyclic,omitempty"`
	Transitive bool   `json:"transitive,omitempty"`
}

type ModelConfig struct {
	Builtin string `json:"builtin"`
}

type AdapterConfig struct {
	Name         string   `json:"name"`
	Command      []string `json:"command"`
	Blocks       []string `json:"blocks"`
	Checks       []string `json:"checks,omitempty"`
	BuiltinShell bool     `json:"-"` // set internally for the auto-registered shell adapter
	BuiltinJQ    bool     `json:"-"` // set internally for the auto-registered jq check adapter
}

type Reporter struct {
	Builtin string `json:"builtin"`
	OutFile string `json:"outFile"`
}

// Default returns sensible defaults that allow specdown to run without a config file.
func Default() Config {
	return Config{
		Entry:    "specs/index.spec.md",
		Adapters: nil,
		Models:   ModelConfig{Builtin: "alloy"},
		Reporters: []Reporter{
			{Builtin: "html", OutFile: "specs/report"},
			{Builtin: "json", OutFile: "specs/report.json"},
		},
	}
}

func applyDefaults(cfg *Config) {
	if cfg.Entry == "" {
		cfg.Entry = "specs/index.spec.md"
	}
	if cfg.Models.Builtin == "" {
		cfg.Models.Builtin = "alloy"
	}
	if len(cfg.Reporters) == 0 {
		cfg.Reporters = []Reporter{
			{Builtin: "html", OutFile: "specs/report"},
			{Builtin: "json", OutFile: "specs/report.json"},
		}
	}
}

func Load(path string) (Config, string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return Config{}, "", fmt.Errorf("resolve config path: %w", err)
	}

	body, err := os.ReadFile(absPath)
	if err != nil {
		return Config{}, "", fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(body, &cfg); err != nil {
		return Config{}, "", fmt.Errorf("parse config: %w", err)
	}

	applyDefaults(&cfg)
	if err := validateAdapters(cfg.Adapters); err != nil {
		return Config{}, "", err
	}
	if cfg.Models.Builtin != "alloy" {
		return Config{}, "", fmt.Errorf("models builtin %q is not supported (only \"alloy\" is available)", cfg.Models.Builtin)
	}
	if cfg.Trace != nil {
		if err := validateTrace(cfg.Trace); err != nil {
			return Config{}, "", err
		}
	}

	return cfg, filepath.Dir(absPath), nil
}

// LoadOrDefault loads config from path, or returns Default() if the file does not exist.
func LoadOrDefault(path string) (Config, string, error) {
	cfg, dir, err := Load(path)
	if err != nil {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) && os.IsNotExist(pathErr.Err) {
			cwd, wdErr := os.Getwd()
			if wdErr != nil {
				return Config{}, "", fmt.Errorf("get working directory: %w", wdErr)
			}
			return Default(), cwd, nil
		}
		return Config{}, "", err
	}
	return cfg, dir, nil
}

func validateAdapters(adapters []AdapterConfig) error {
	seen := make(map[string]struct{}, len(adapters))
	for _, adapter := range adapters {
		if adapter.BuiltinShell {
			continue
		}
		if adapter.Name == "" {
			return fmt.Errorf("adapter name must not be empty")
		}
		if _, ok := seen[adapter.Name]; ok {
			return fmt.Errorf("adapter %q is defined more than once", adapter.Name)
		}
		seen[adapter.Name] = struct{}{}
		if len(adapter.Command) == 0 {
			return fmt.Errorf("adapter %q must define a command", adapter.Name)
		}
		if len(adapter.Blocks) == 0 && len(adapter.Checks) == 0 {
			return fmt.Errorf("adapter %q must declare at least one block or check", adapter.Name)
		}
	}
	return nil
}

var identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

func validateTrace(trace *TraceConfig) error {
	if len(trace.Types) == 0 {
		return fmt.Errorf("trace.types must not be empty")
	}
	typeSet, err := validateTraceTypes(trace.Types)
	if err != nil {
		return err
	}
	if len(trace.Edges) == 0 {
		return fmt.Errorf("trace.edges must not be empty")
	}
	return validateTraceEdges(trace.Edges, typeSet)
}

func validateTraceTypes(types []string) (map[string]struct{}, error) {
	typeSet := make(map[string]struct{}, len(types))
	for _, t := range types {
		if !identifierPattern.MatchString(t) {
			return nil, fmt.Errorf("trace type %q is not a valid identifier (must match [a-z][a-z0-9_-]*)", t)
		}
		if _, ok := typeSet[t]; ok {
			return nil, fmt.Errorf("trace type %q is declared more than once", t)
		}
		typeSet[t] = struct{}{}
	}
	return typeSet, nil
}

func validateTraceEdges(edges map[string]TraceEdge, typeSet map[string]struct{}) error {
	for name, edge := range edges {
		if !identifierPattern.MatchString(name) {
			return fmt.Errorf("trace edge name %q is not a valid identifier", name)
		}
		if _, ok := typeSet[edge.From]; !ok {
			return fmt.Errorf("trace edge %q references undeclared type %q in 'from'", name, edge.From)
		}
		if _, ok := typeSet[edge.To]; !ok {
			return fmt.Errorf("trace edge %q references undeclared type %q in 'to'", name, edge.To)
		}
		if edge.Count != "" {
			if _, _, err := ParseCount(edge.Count); err != nil {
				return fmt.Errorf("trace edge %q: %w", name, err)
			}
		}
	}
	return nil
}

// Multiplicity represents a UML-style cardinality constraint.
type Multiplicity struct {
	Min int // 0 or 1
	Max int // -1 means unlimited (*)
}

// ParseCount parses a count string like "1..* → 1..*" into source-side and target-side
// multiplicities using UML convention: the source-side number (left of →) describes how many
// sources each target has; the target-side number (right of →) describes how many targets
// each source has.
func ParseCount(s string) (source, target Multiplicity, err error) {
	// Normalize arrow
	normalized := strings.ReplaceAll(s, "→", "->")
	parts := strings.SplitN(normalized, "->", 2)
	if len(parts) != 2 {
		return Multiplicity{}, Multiplicity{}, fmt.Errorf("invalid count %q: expected format 'source -> target'", s)
	}
	source, err = parseMultiplicity(strings.TrimSpace(parts[0]))
	if err != nil {
		return Multiplicity{}, Multiplicity{}, fmt.Errorf("invalid count %q: source side: %w", s, err)
	}
	target, err = parseMultiplicity(strings.TrimSpace(parts[1]))
	if err != nil {
		return Multiplicity{}, Multiplicity{}, fmt.Errorf("invalid count %q: target side: %w", s, err)
	}
	return source, target, nil
}

func parseMultiplicity(s string) (Multiplicity, error) {
	switch s {
	case "0..*":
		return Multiplicity{Min: 0, Max: -1}, nil
	case "1..*":
		return Multiplicity{Min: 1, Max: -1}, nil
	case "0..1":
		return Multiplicity{Min: 0, Max: 1}, nil
	case "1":
		return Multiplicity{Min: 1, Max: 1}, nil
	case "1..1":
		return Multiplicity{Min: 1, Max: 1}, nil
	default:
		return Multiplicity{}, fmt.Errorf("unsupported multiplicity %q (expected 1, 0..1, 1..*, or 0..*)", s)
	}
}

// EffectiveDefaultTimeout returns the configured default timeout, or
// DefaultTimeoutMsec (30s) when not explicitly set in the config file.
func (c Config) EffectiveDefaultTimeout() int {
	if c.DefaultTimeoutPtr != nil {
		return *c.DefaultTimeoutPtr
	}
	return DefaultTimeoutMsec
}

func (c Config) HTMLReportOutFile() string {
	for _, reporter := range c.Reporters {
		if reporter.Builtin == "html" && reporter.OutFile != "" {
			return reporter.OutFile
		}
	}
	return ""
}

func (c Config) JSONReportOutFile() string {
	for _, reporter := range c.Reporters {
		if reporter.Builtin == "json" && reporter.OutFile != "" {
			return reporter.OutFile
		}
	}
	return ""
}
