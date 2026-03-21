package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corca-ai/specdown/internal/specdown/testutil"
)

// --- Load ---

func TestLoadConfigParsesAdaptersAndReporters(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "specdown.json")
	body := `{
  "entry": "specs/index.spec.md",
  "adapters": [
    {
      "name": "project",
      "command": ["python3", "./tools/adapter.py"],
      "blocks": ["run:myapp"],
      "checks": ["user-exists"]
    }
  ],
  "models": {
    "builtin": "alloy"
  },
  "reporters": [
    {
      "builtin": "html",
      "outFile": ".artifacts/specdown/report.html"
    }
  ]
}`
	testutil.NilErr(t, os.WriteFile(configPath, []byte(body), 0o644))

	cfg, dir, err := Load(configPath)
	testutil.NilErr(t, err)
	testutil.Equal(t, dir, root)
	testutil.Len(t, cfg.Adapters, 1)
	testutil.Equal(t, cfg.Adapters[0].Name, "project")
	testutil.Equal(t, cfg.Entry, "specs/index.spec.md")
	testutil.Equal(t, cfg.Models.Builtin, "alloy")
	testutil.Equal(t, cfg.HTMLReportOutFile(), ".artifacts/specdown/report.html")
}

func TestLoadConfigAllowsAlloyOnlyProjectWithoutAdapters(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "specdown.json")
	body := `{
  "entry": "specs/index.spec.md",
  "reporters": [
    {
      "builtin": "html",
      "outFile": ".artifacts/specdown/report.html"
    }
  ]
}`
	testutil.NilErr(t, os.WriteFile(configPath, []byte(body), 0o644))

	cfg, _, err := Load(configPath)
	testutil.NilErr(t, err)
	testutil.Len(t, cfg.Adapters, 0)
}

func TestLoadAppliesDefaults(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "specdown.json")
	testutil.NilErr(t, os.WriteFile(configPath, []byte(`{}`), 0o644))

	cfg, _, err := Load(configPath)
	testutil.NilErr(t, err)
	testutil.Equal(t, cfg.Entry, "specs/index.spec.md")
	testutil.Equal(t, cfg.Models.Builtin, "alloy")
	testutil.Len(t, cfg.Reporters, 2)
}

func TestLoadConfigRejectsUnknownModelBuiltin(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "specdown.json")
	body := `{"models": {"builtin": "unknown"}}`
	testutil.NilErr(t, os.WriteFile(configPath, []byte(body), 0o644))

	_, _, err := Load(configPath)
	testutil.ErrContains(t, err, `models builtin "unknown" is not supported`)
}

func TestLoadConfigInvalidJSON(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "specdown.json")
	testutil.NilErr(t, os.WriteFile(configPath, []byte("{bad"), 0o644))

	_, _, err := Load(configPath)
	testutil.ErrContains(t, err, "parse config")
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, _, err := Load("/no/such/specdown.json")
	testutil.WantErr(t, err)
}

// --- LoadOrDefault ---

func TestLoadOrDefaultWhenMissing(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "nonexistent.json")
	cfg, dir, err := LoadOrDefault(configPath)
	testutil.NilErr(t, err)
	testutil.Equal(t, cfg.Entry, "specs/index.spec.md")
	testutil.Equal(t, cfg.Models.Builtin, "alloy")
	testutil.True(t, dir != "")
}

func TestLoadOrDefaultWhenPresent(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "specdown.json")
	testutil.NilErr(t, os.WriteFile(configPath, []byte(`{"entry":"custom.md"}`), 0o644))

	cfg, _, err := LoadOrDefault(configPath)
	testutil.NilErr(t, err)
	testutil.Equal(t, cfg.Entry, "custom.md")
}

func TestLoadOrDefaultReturnsErrorOnBadJSON(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "specdown.json")
	testutil.NilErr(t, os.WriteFile(configPath, []byte("{bad"), 0o644))

	_, _, err := LoadOrDefault(configPath)
	testutil.WantErr(t, err)
}

// --- Default ---

func TestDefault(t *testing.T) {
	cfg := Default()
	testutil.Equal(t, cfg.Entry, "specs/index.spec.md")
	testutil.Equal(t, cfg.Models.Builtin, "alloy")
	testutil.Len(t, cfg.Reporters, 2)
	testutil.Equal(t, cfg.HTMLReportOutFile(), "specs/report")
	testutil.Equal(t, cfg.JSONReportOutFile(), "specs/report.json")
}

// --- EffectiveDefaultTimeout ---

func TestEffectiveDefaultTimeoutUsesConstantWhenNotSet(t *testing.T) {
	cfg := Config{}
	testutil.Equal(t, cfg.EffectiveDefaultTimeout(), DefaultTimeoutMsec)
}

func TestEffectiveDefaultTimeoutUsesConfigValue(t *testing.T) {
	v := 60000
	cfg := Config{DefaultTimeoutMs: &v}
	testutil.Equal(t, cfg.EffectiveDefaultTimeout(), 60000)
}

func TestEffectiveDefaultTimeoutAllowsZero(t *testing.T) {
	v := 0
	cfg := Config{DefaultTimeoutMs: &v}
	testutil.Equal(t, cfg.EffectiveDefaultTimeout(), 0)
}

func TestLoadConfigWithDefaultTimeoutMsec(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "specdown.json")
	body := `{"defaultTimeoutMsec": 90000}`
	testutil.NilErr(t, os.WriteFile(configPath, []byte(body), 0o644))

	cfg, _, err := Load(configPath)
	testutil.NilErr(t, err)
	testutil.Equal(t, cfg.EffectiveDefaultTimeout(), 90000)
}

// --- HTMLReportOutFile / JSONReportOutFile ---

func TestJSONReportOutFile(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "specdown.json")
	body := `{
  "reporters": [
    {"builtin": "html", "outFile": "report.html"},
    {"builtin": "json", "outFile": "result.json"}
  ]
}`
	testutil.NilErr(t, os.WriteFile(configPath, []byte(body), 0o644))

	cfg, _, err := Load(configPath)
	testutil.NilErr(t, err)
	testutil.Equal(t, cfg.JSONReportOutFile(), "result.json")
}

func TestJSONReportOutFileReturnsEmptyWhenNotConfigured(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "specdown.json")
	body := `{"reporters": [{"builtin": "html", "outFile": "r.html"}]}`
	testutil.NilErr(t, os.WriteFile(configPath, []byte(body), 0o644))

	cfg, _, err := Load(configPath)
	testutil.NilErr(t, err)
	testutil.Equal(t, cfg.JSONReportOutFile(), "")
}

func TestHTMLReportOutFileReturnsEmptyWhenNoReporters(t *testing.T) {
	cfg := Config{}
	testutil.Equal(t, cfg.HTMLReportOutFile(), "")
}

// --- validateAdapters ---

func TestValidateAdaptersEmptyName(t *testing.T) {
	err := validateAdapters([]AdapterConfig{{Name: "", Command: []string{"cmd"}, Blocks: []string{"b"}}})
	testutil.ErrContains(t, err, "name must not be empty")
}

func TestValidateAdaptersDuplicate(t *testing.T) {
	err := validateAdapters([]AdapterConfig{
		{Name: "a", Command: []string{"cmd"}, Blocks: []string{"b"}},
		{Name: "a", Command: []string{"cmd"}, Blocks: []string{"b"}},
	})
	testutil.ErrContains(t, err, "defined more than once")
}

func TestValidateAdaptersNoCommand(t *testing.T) {
	err := validateAdapters([]AdapterConfig{{Name: "a", Command: nil, Blocks: []string{"b"}}})
	testutil.ErrContains(t, err, "must define a command")
}

func TestValidateAdaptersNoBlocksOrChecks(t *testing.T) {
	err := validateAdapters([]AdapterConfig{{Name: "a", Command: []string{"cmd"}}})
	testutil.ErrContains(t, err, "must declare at least one block or check")
}

func TestValidateAdaptersSkipsBuiltinShell(t *testing.T) {
	err := validateAdapters([]AdapterConfig{{BuiltinShell: true}})
	testutil.NilErr(t, err)
}

func TestValidateAdaptersValid(t *testing.T) {
	err := validateAdapters([]AdapterConfig{
		{Name: "a", Command: []string{"cmd"}, Blocks: []string{"b"}},
	})
	testutil.NilErr(t, err)
}

// --- validateTrace ---

func TestValidateTraceEmptyTypes(t *testing.T) {
	err := validateTrace(&TraceConfig{Types: nil, Edges: map[string]TraceEdge{"e": {From: "a", To: "b"}}})
	testutil.ErrContains(t, err, "trace.types must not be empty")
}

func TestValidateTraceEmptyEdges(t *testing.T) {
	err := validateTrace(&TraceConfig{Types: []string{"spec"}, Edges: nil})
	testutil.ErrContains(t, err, "trace.edges must not be empty")
}

func TestValidateTraceValid(t *testing.T) {
	err := validateTrace(&TraceConfig{
		Types: []string{"spec", "goal"},
		Edges: map[string]TraceEdge{"covers": {From: "spec", To: "goal"}},
	})
	testutil.NilErr(t, err)
}

func TestValidateTraceWithCount(t *testing.T) {
	err := validateTrace(&TraceConfig{
		Types: []string{"spec", "goal"},
		Edges: map[string]TraceEdge{"covers": {From: "spec", To: "goal", Count: "1..* -> 0..*"}},
	})
	testutil.NilErr(t, err)
}

func TestValidateTraceInvalidCount(t *testing.T) {
	err := validateTrace(&TraceConfig{
		Types: []string{"spec", "goal"},
		Edges: map[string]TraceEdge{"covers": {From: "spec", To: "goal", Count: "invalid"}},
	})
	testutil.WantErr(t, err)
}

// --- validateTraceTypes ---

func TestValidateTraceTypesInvalidIdentifier(t *testing.T) {
	_, err := validateTraceTypes([]string{"Valid", "123bad"})
	testutil.WantErr(t, err)
}

func TestValidateTraceTypesDuplicate(t *testing.T) {
	_, err := validateTraceTypes([]string{"spec", "spec"})
	testutil.ErrContains(t, err, "declared more than once")
}

func TestValidateTraceTypesValid(t *testing.T) {
	set, err := validateTraceTypes([]string{"spec", "goal", "test_case"})
	testutil.NilErr(t, err)
	testutil.Equal(t, len(set), 3)
}

// --- validateTraceEdges ---

func TestValidateTraceEdgesInvalidName(t *testing.T) {
	types := map[string]struct{}{"spec": {}}
	err := validateTraceEdges(map[string]TraceEdge{"BAD": {From: "spec", To: "spec"}}, types)
	testutil.ErrContains(t, err, "not a valid identifier")
}

func TestValidateTraceEdgesHyphenatedName(t *testing.T) {
	types := map[string]struct{}{"spec": {}}
	err := validateTraceEdges(map[string]TraceEdge{"covers-requirement": {From: "spec", To: "spec"}}, types)
	testutil.NilErr(t, err)
}

func TestValidateTraceEdgesUndeclaredFrom(t *testing.T) {
	types := map[string]struct{}{"spec": {}}
	err := validateTraceEdges(map[string]TraceEdge{"covers": {From: "unknown", To: "spec"}}, types)
	testutil.ErrContains(t, err, "undeclared type")
}

func TestValidateTraceEdgesUndeclaredTo(t *testing.T) {
	types := map[string]struct{}{"spec": {}}
	err := validateTraceEdges(map[string]TraceEdge{"covers": {From: "spec", To: "unknown"}}, types)
	testutil.ErrContains(t, err, "undeclared type")
}

// --- ParseCount ---

func TestParseCount(t *testing.T) {
	for _, tc := range []struct {
		name      string
		input     string
		srcMin    int
		srcMax    int
		tgtMin    int
		tgtMax    int
		wantError bool
	}{
		{"star to star", "0..* -> 0..*", 0, -1, 0, -1, false},
		{"one to star", "1..* -> 0..*", 1, -1, 0, -1, false},
		{"one to one", "1 -> 1", 1, 1, 1, 1, false},
		{"optional to required", "0..1 -> 1..*", 0, 1, 1, -1, false},
		{"unicode arrow", "1..* \u2192 0..*", 1, -1, 0, -1, false},
		{"no arrow", "1..*", 0, 0, 0, 0, true},
		{"invalid source", "bad -> 1", 0, 0, 0, 0, true},
		{"invalid target", "1 -> bad", 0, 0, 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src, tgt, err := ParseCount(tc.input)
			if tc.wantError {
				testutil.WantErr(t, err)
				return
			}
			testutil.NilErr(t, err)
			testutil.Equal(t, src.Min, tc.srcMin)
			testutil.Equal(t, src.Max, tc.srcMax)
			testutil.Equal(t, tgt.Min, tc.tgtMin)
			testutil.Equal(t, tgt.Max, tc.tgtMax)
		})
	}
}

// --- parseMultiplicity ---

func TestParseMultiplicity(t *testing.T) {
	for _, tc := range []struct {
		input     string
		min       int
		max       int
		wantError bool
	}{
		{"0..*", 0, -1, false},
		{"1..*", 1, -1, false},
		{"0..1", 0, 1, false},
		{"1", 1, 1, false},
		{"1..1", 1, 1, false},
		{"2", 0, 0, true},
		{"*", 0, 0, true},
		{"", 0, 0, true},
	} {
		t.Run(tc.input, func(t *testing.T) {
			m, err := parseMultiplicity(tc.input)
			if tc.wantError {
				testutil.WantErr(t, err)
				return
			}
			testutil.NilErr(t, err)
			testutil.Equal(t, m.Min, tc.min)
			testutil.Equal(t, m.Max, tc.max)
		})
	}
}

// --- Config with trace validation integration ---

func TestLoadConfigWithTrace(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "specdown.json")
	body := `{
  "trace": {
    "types": ["spec", "goal"],
    "edges": {
      "covers": {"from": "spec", "to": "goal", "count": "0..* -> 1..*"}
    }
  }
}`
	testutil.NilErr(t, os.WriteFile(configPath, []byte(body), 0o644))

	cfg, _, err := Load(configPath)
	testutil.NilErr(t, err)
	testutil.True(t, cfg.Trace != nil)
	testutil.Len(t, cfg.Trace.Types, 2)
}

func TestLoadConfigRejectsInvalidTrace(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "specdown.json")
	body := `{"trace": {"types": [], "edges": {}}}`
	testutil.NilErr(t, os.WriteFile(configPath, []byte(body), 0o644))

	_, _, err := Load(configPath)
	testutil.ErrContains(t, err, "trace.types must not be empty")
}
