package trace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corca-ai/specdown/internal/specdown/config"
	"github.com/corca-ai/specdown/internal/specdown/testutil"
)

// --- TraceError.Error ---

func TestTraceErrorFormat(t *testing.T) {
	t.Run("with line", func(t *testing.T) {
		e := TraceError{File: "f.md", Line: 5, Edge: "covers", Message: "bad"}
		testutil.Equal(t, e.Error(), "f.md:5: [covers] bad")
	})
	t.Run("graph level", func(t *testing.T) {
		e := TraceError{File: "GRAPH", Edge: "covers", Message: "cycle"}
		testutil.Equal(t, e.Error(), "GRAPH: [covers] cycle")
	})
	t.Run("file level no line", func(t *testing.T) {
		e := TraceError{File: "f.md", Edge: "covers", Message: "err"}
		testutil.Equal(t, e.Error(), "f.md: [covers] err")
	})
}

// --- ParseTraceLinks ---

func TestParseTraceLinks_SimpleLinks(t *testing.T) {
	md := `See [covers::Feature X](feature.md) and [depends::Other](other.md).`
	links := ParseTraceLinks("test.md", md)
	testutil.Len(t, links, 2)
	testutil.Equal(t, links[0].EdgeName, "covers")
	testutil.Equal(t, links[0].TargetPath, "feature.md")
	testutil.Equal(t, links[1].EdgeName, "depends")
	testutil.Equal(t, links[1].DisplayText, "Other")
}

func TestParseTraceLinks_CodeBlocksIgnored(t *testing.T) {
	md := "```\n[covers::X](y.md)\n```\n[covers::Z](z.md)"
	links := ParseTraceLinks("test.md", md)
	testutil.Len(t, links, 1)
	testutil.Equal(t, links[0].DisplayText, "Z")
}

func TestParseTraceLinks_ExternalURLs(t *testing.T) {
	md := `No links here, just [covers::Docs](https://example.com)`
	links := ParseTraceLinks("test.md", md)
	testutil.Len(t, links, 1)
	testutil.Equal(t, links[0].TargetPath, "https://example.com")
}

func TestParseTraceLinks_FragmentStripped(t *testing.T) {
	md := `[covers::Section](doc.md#heading)`
	links := ParseTraceLinks("test.md", md)
	testutil.Len(t, links, 1)
	testutil.Equal(t, links[0].TargetPath, "doc.md")
}

func TestParseTraceLinks_EmptyDisplayText(t *testing.T) {
	md := `[covers:: ](doc.md)`
	links := ParseTraceLinks("test.md", md)
	testutil.Len(t, links, 0)
}

func TestParseTraceLinks_LineNumbers(t *testing.T) {
	md := "first line\n[covers::X](x.md)\nthird line\n[depends::Y](y.md)"
	links := ParseTraceLinks("test.md", md)
	testutil.Len(t, links, 2)
	testutil.Equal(t, links[0].SourceLine, 2)
	testutil.Equal(t, links[1].SourceLine, 4)
}

// --- validateLink ---

func TestValidateLink(t *testing.T) {
	cfg := &config.TraceConfig{
		Types: []string{"goal", "feature"},
		Edges: map[string]config.TraceEdge{
			"covers": {From: "feature", To: "goal"},
		},
	}
	docs := map[string]TypedDocument{
		"feat.md": {Path: "feat.md", Type: "feature"},
		"goal.md": {Path: "goal.md", Type: "goal"},
	}

	t.Run("valid link", func(t *testing.T) {
		link := TraceLink{SourcePath: "feat.md", SourceLine: 1, EdgeName: "covers", TargetPath: "goal.md"}
		edges, errs := validateLink(link, cfg, docs)
		testutil.Len(t, errs, 0)
		testutil.Len(t, edges, 1)
		testutil.Equal(t, edges[0].Source, "feat.md")
		testutil.Equal(t, edges[0].Target, "goal.md")
	})
	t.Run("unknown edge", func(t *testing.T) {
		link := TraceLink{SourcePath: "feat.md", SourceLine: 1, EdgeName: "unknown", TargetPath: "goal.md"}
		_, errs := validateLink(link, cfg, docs)
		testutil.Len(t, errs, 1)
		testutil.Contains(t, errs[0].Message, "unknown edge name")
	})
	t.Run("type mismatch", func(t *testing.T) {
		link := TraceLink{SourcePath: "goal.md", SourceLine: 1, EdgeName: "covers", TargetPath: "feat.md"}
		_, errs := validateLink(link, cfg, docs)
		testutil.Len(t, errs, 1)
		testutil.Contains(t, errs[0].Message, "type mismatch")
	})
	t.Run("dangling target", func(t *testing.T) {
		link := TraceLink{SourcePath: "feat.md", SourceLine: 1, EdgeName: "covers", TargetPath: "missing.md"}
		_, errs := validateLink(link, cfg, docs)
		testutil.Len(t, errs, 1)
		testutil.Contains(t, errs[0].Message, "dangling reference")
	})
	t.Run("self-loop", func(t *testing.T) {
		selfDocs := map[string]TypedDocument{
			"feat.md": {Path: "feat.md", Type: "feature"},
		}
		selfCfg := &config.TraceConfig{
			Types: []string{"feature"},
			Edges: map[string]config.TraceEdge{"covers": {From: "feature", To: "feature"}},
		}
		link := TraceLink{SourcePath: "feat.md", SourceLine: 1, EdgeName: "covers", TargetPath: "feat.md"}
		_, errs := validateLink(link, selfCfg, selfDocs)
		testutil.Len(t, errs, 1)
		testutil.Contains(t, errs[0].Message, "self-loop")
	})
	t.Run("untyped source", func(t *testing.T) {
		untypedDocs := map[string]TypedDocument{
			"untyped.md": {Path: "untyped.md", Type: ""},
			"goal.md":    {Path: "goal.md", Type: "goal"},
		}
		link := TraceLink{SourcePath: "untyped.md", SourceLine: 1, EdgeName: "covers", TargetPath: "goal.md"}
		_, errs := validateLink(link, cfg, untypedDocs)
		testutil.Len(t, errs, 1)
		testutil.Contains(t, errs[0].Message, "no type")
	})
	t.Run("untyped target", func(t *testing.T) {
		mixedDocs := map[string]TypedDocument{
			"feat.md":    {Path: "feat.md", Type: "feature"},
			"untyped.md": {Path: "untyped.md", Type: ""},
		}
		link := TraceLink{SourcePath: "feat.md", SourceLine: 1, EdgeName: "covers", TargetPath: "untyped.md"}
		_, errs := validateLink(link, cfg, mixedDocs)
		testutil.Len(t, errs, 1)
		testutil.Contains(t, errs[0].Message, "no type")
	})
	t.Run("target type mismatch", func(t *testing.T) {
		wrongTypeDocs := map[string]TypedDocument{
			"feat.md":  {Path: "feat.md", Type: "feature"},
			"feat2.md": {Path: "feat2.md", Type: "feature"},
		}
		link := TraceLink{SourcePath: "feat.md", SourceLine: 1, EdgeName: "covers", TargetPath: "feat2.md"}
		_, errs := validateLink(link, cfg, wrongTypeDocs)
		testutil.Len(t, errs, 1)
		testutil.Contains(t, errs[0].Message, "type mismatch")
	})
}

// --- resolveTraceLinks ---

func TestResolveTraceLinks(t *testing.T) {
	t.Run("resolves relative path", func(t *testing.T) {
		md := `[covers::X](../other.md)`
		links, errs := resolveTraceLinks("specs/test.md", md)
		testutil.Len(t, errs, 0)
		testutil.Len(t, links, 1)
		testutil.Equal(t, links[0].TargetPath, "other.md")
	})
	t.Run("rejects escape outside root", func(t *testing.T) {
		md := `[covers::X](../../outside.md)`
		_, errs := resolveTraceLinks("specs/test.md", md)
		testutil.Len(t, errs, 1)
		testutil.Contains(t, errs[0].Message, "resolves outside project root")
	})
	t.Run("skips external URLs", func(t *testing.T) {
		md := `[covers::X](https://example.com)`
		links, errs := resolveTraceLinks("test.md", md)
		testutil.Len(t, errs, 0)
		testutil.Len(t, links, 0)
	})
	t.Run("same directory", func(t *testing.T) {
		md := `[covers::X](other.md)`
		links, errs := resolveTraceLinks("specs/test.md", md)
		testutil.Len(t, errs, 0)
		testutil.Len(t, links, 1)
		testutil.Equal(t, links[0].TargetPath, "specs/other.md")
	})
}

// --- isIgnored ---

func TestIsIgnored(t *testing.T) {
	t.Run("simple match", func(t *testing.T) {
		testutil.True(t, isIgnored("README.md", []string{"README.md"}))
	})
	t.Run("no match", func(t *testing.T) {
		testutil.False(t, isIgnored("specs/test.md", []string{"README.md"}))
	})
	t.Run("glob match", func(t *testing.T) {
		testutil.True(t, isIgnored("docs/api/guide.md", []string{"docs/**/*.md"}))
	})
	t.Run("empty patterns", func(t *testing.T) {
		testutil.False(t, isIgnored("test.md", nil))
	})
}

// --- matchGlob ---

func TestMatchGlob(t *testing.T) {
	t.Run("simple pattern", func(t *testing.T) {
		testutil.True(t, matchGlob("*.md", "readme.md"))
	})
	t.Run("double star", func(t *testing.T) {
		testutil.True(t, matchGlob("docs/**/*.md", "docs/api/guide.md"))
	})
	t.Run("non-match", func(t *testing.T) {
		testutil.False(t, matchGlob("*.txt", "readme.md"))
	})
	t.Run("double star prefix only", func(t *testing.T) {
		testutil.True(t, matchGlob("docs/**", "docs/any/deep/path.md"))
	})
	t.Run("double star with trailing slash", func(t *testing.T) {
		testutil.True(t, matchGlob("docs/**/", "docs/sub/file.md"))
	})
	t.Run("no prefix double star", func(t *testing.T) {
		testutil.True(t, matchGlob("**/*.md", "deep/nested/file.md"))
	})
	t.Run("exact file no glob", func(t *testing.T) {
		testutil.True(t, matchGlob("file.md", "file.md"))
	})
	t.Run("exact file mismatch", func(t *testing.T) {
		testutil.False(t, matchGlob("file.md", "other.md"))
	})
}

// --- detectCycles ---

func TestDetectCycles(t *testing.T) {
	t.Run("no cycle", func(t *testing.T) {
		edges := []Edge{
			{Source: "a", Target: "b", EdgeName: "e"},
			{Source: "b", Target: "c", EdgeName: "e"},
		}
		testutil.Len(t, detectCycles(edges), 0)
	})
	t.Run("simple cycle", func(t *testing.T) {
		edges := []Edge{
			{Source: "a", Target: "b", EdgeName: "e"},
			{Source: "b", Target: "a", EdgeName: "e"},
		}
		testutil.Len(t, detectCycles(edges), 1)
	})
	t.Run("long cycle", func(t *testing.T) {
		edges := []Edge{
			{Source: "a", Target: "b", EdgeName: "e"},
			{Source: "b", Target: "c", EdgeName: "e"},
			{Source: "c", Target: "a", EdgeName: "e"},
		}
		testutil.Len(t, detectCycles(edges), 1)
	})
	t.Run("empty edges", func(t *testing.T) {
		testutil.Len(t, detectCycles(nil), 0)
	})
}

// --- computeTransitiveClosure ---

func TestComputeTransitiveClosure(t *testing.T) {
	t.Run("chain", func(t *testing.T) {
		edges := []Edge{
			{Source: "a", Target: "b", EdgeName: "dep"},
			{Source: "b", Target: "c", EdgeName: "dep"},
		}
		transitive := computeTransitiveClosure(edges, "dep")
		testutil.Len(t, transitive, 1)
		testutil.Equal(t, transitive[0].Source, "a")
		testutil.Equal(t, transitive[0].Target, "c")
	})
	t.Run("diamond", func(t *testing.T) {
		edges := []Edge{
			{Source: "a", Target: "b", EdgeName: "dep"},
			{Source: "a", Target: "c", EdgeName: "dep"},
			{Source: "b", Target: "d", EdgeName: "dep"},
			{Source: "c", Target: "d", EdgeName: "dep"},
		}
		transitive := computeTransitiveClosure(edges, "dep")
		testutil.Len(t, transitive, 1)
		testutil.Equal(t, transitive[0].Source, "a")
		testutil.Equal(t, transitive[0].Target, "d")
	})
	t.Run("ignores other edge names", func(t *testing.T) {
		edges := []Edge{
			{Source: "a", Target: "b", EdgeName: "dep"},
			{Source: "b", Target: "c", EdgeName: "other"},
		}
		transitive := computeTransitiveClosure(edges, "dep")
		testutil.Len(t, transitive, 0)
	})
	t.Run("no edges", func(t *testing.T) {
		testutil.Len(t, computeTransitiveClosure(nil, "dep"), 0)
	})
}

// --- satisfiesMultiplicity ---

func TestValidateCardinality(t *testing.T) {
	t.Run("satisfied", func(t *testing.T) {
		testutil.True(t, satisfiesMultiplicity(1, config.Multiplicity{Min: 1, Max: -1}))
	})
	t.Run("violated min", func(t *testing.T) {
		testutil.False(t, satisfiesMultiplicity(0, config.Multiplicity{Min: 1, Max: -1}))
	})
	t.Run("violated max", func(t *testing.T) {
		testutil.False(t, satisfiesMultiplicity(3, config.Multiplicity{Min: 0, Max: 2}))
	})
	t.Run("zero allowed", func(t *testing.T) {
		testutil.True(t, satisfiesMultiplicity(0, config.Multiplicity{Min: 0, Max: -1}))
	})
	t.Run("exact match", func(t *testing.T) {
		testutil.True(t, satisfiesMultiplicity(1, config.Multiplicity{Min: 1, Max: 1}))
	})
}

// --- formatMultiplicity ---

func TestFormatMultiplicity(t *testing.T) {
	for _, tc := range []struct {
		m    config.Multiplicity
		want string
	}{
		{config.Multiplicity{Min: 0, Max: -1}, "0..*"},
		{config.Multiplicity{Min: 1, Max: -1}, "1..*"},
		{config.Multiplicity{Min: 1, Max: 1}, "1"},
		{config.Multiplicity{Min: 0, Max: 1}, "0..1"},
		{config.Multiplicity{Min: 2, Max: 5}, "2..5"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			testutil.Equal(t, formatMultiplicity(tc.m), tc.want)
		})
	}
}

// --- countEdgesForDoc ---

func TestCountEdgesForDoc(t *testing.T) {
	edges := []Edge{
		{Source: "a", Target: "b", EdgeName: "e"},
		{Source: "a", Target: "c", EdgeName: "e"},
		{Source: "b", Target: "a", EdgeName: "e"},
	}
	testutil.Equal(t, countEdgesForDoc(edges, "a", true), 2)  // outgoing from a
	testutil.Equal(t, countEdgesForDoc(edges, "a", false), 1) // incoming to a
	testutil.Equal(t, countEdgesForDoc(edges, "c", true), 0)
	testutil.Equal(t, countEdgesForDoc(edges, "c", false), 1)
}

// --- filterEdgesByName ---

func TestFilterEdgesByName(t *testing.T) {
	edges := []Edge{
		{Source: "a", Target: "b", EdgeName: "covers"},
		{Source: "a", Target: "c", EdgeName: "depends"},
		{Source: "b", Target: "c", EdgeName: "covers"},
	}
	testutil.Len(t, filterEdgesByName(edges, "covers"), 2)
	testutil.Len(t, filterEdgesByName(edges, "depends"), 1)
	testutil.Len(t, filterEdgesByName(edges, "unknown"), 0)
}

// --- deduplicateEdges ---

func TestDeduplicateEdges(t *testing.T) {
	edges := []Edge{
		{Source: "a", Target: "b", EdgeName: "e"},
		{Source: "a", Target: "b", EdgeName: "e"},
		{Source: "a", Target: "c", EdgeName: "e"},
	}
	testutil.Len(t, deduplicateEdges(edges), 2)
}

// --- checkCardinality ---

func TestCheckCardinality(t *testing.T) {
	docs := []TypedDocument{
		{Path: "a.md", Type: "spec"},
		{Path: "b.md", Type: "spec"},
		{Path: "g.md", Type: "goal"},
	}
	edges := []Edge{
		{Source: "a.md", Target: "g.md", EdgeName: "covers"},
	}

	t.Run("no count constraint", func(t *testing.T) {
		errs := checkCardinality(docs, edges, "covers", config.TraceEdge{From: "spec", To: "goal"})
		testutil.Len(t, errs, 0)
	})
	t.Run("violated source min", func(t *testing.T) {
		// b.md has 0 outgoing covers edges but requires 1..* (right side in UML notation)
		errs := checkCardinality(docs, edges, "covers", config.TraceEdge{
			From: "spec", To: "goal", Count: "0..* -> 1..*",
		})
		testutil.True(t, len(errs) > 0)
		testutil.Contains(t, errs[0].Message, "b.md")
	})
	t.Run("satisfied", func(t *testing.T) {
		fullEdges := []Edge{
			{Source: "a.md", Target: "g.md", EdgeName: "covers"},
			{Source: "b.md", Target: "g.md", EdgeName: "covers"},
		}
		errs := checkCardinality(docs, fullEdges, "covers", config.TraceEdge{
			From: "spec", To: "goal", Count: "0..* -> 1..*",
		})
		testutil.Len(t, errs, 0)
	})
}

// --- checkCycles ---

func TestCheckCycles(t *testing.T) {
	t.Run("not acyclic — no check", func(t *testing.T) {
		edges := []Edge{{Source: "a", Target: "b"}, {Source: "b", Target: "a"}}
		errs := checkCycles(edges, edges, "e", config.TraceEdge{Acyclic: false})
		testutil.Len(t, errs, 0)
	})
	t.Run("acyclic with cycle", func(t *testing.T) {
		edges := []Edge{
			{Source: "a", Target: "b", EdgeName: "e"},
			{Source: "b", Target: "a", EdgeName: "e"},
		}
		errs := checkCycles(edges, edges, "e", config.TraceEdge{Acyclic: true})
		testutil.True(t, len(errs) > 0)
		testutil.Contains(t, errs[0].Message, "cycle detected")
	})
	t.Run("acyclic no cycle", func(t *testing.T) {
		edges := []Edge{
			{Source: "a", Target: "b", EdgeName: "e"},
			{Source: "b", Target: "c", EdgeName: "e"},
		}
		errs := checkCycles(edges, edges, "e", config.TraceEdge{Acyclic: true})
		testutil.Len(t, errs, 0)
	})
	t.Run("transitive closure reveals cycle", func(t *testing.T) {
		edges := []Edge{
			{Source: "a", Target: "b", EdgeName: "e"},
			{Source: "b", Target: "c", EdgeName: "e"},
			{Source: "c", Target: "a", EdgeName: "e"},
		}
		errs := checkCycles(edges, edges, "e", config.TraceEdge{Acyclic: true, Transitive: true})
		testutil.True(t, len(errs) > 0)
	})
}

// --- validateGraph ---

func TestValidateGraph(t *testing.T) {
	docs := []TypedDocument{
		{Path: "s.md", Type: "spec"},
		{Path: "g.md", Type: "goal"},
	}
	edges := []Edge{{Source: "s.md", Target: "g.md", EdgeName: "covers"}}
	cfg := &config.TraceConfig{
		Types: []string{"spec", "goal"},
		Edges: map[string]config.TraceEdge{
			"covers": {From: "spec", To: "goal", Acyclic: true},
		},
	}
	errs := validateGraph(docs, edges, cfg)
	testutil.Len(t, errs, 0)
}

// --- Validate (integration) ---

func TestValidateIntegration(t *testing.T) {
	root := t.TempDir()
	// Create spec and goal documents.
	specsDir := filepath.Join(root, "specs")
	testutil.NilErr(t, os.MkdirAll(specsDir, 0o755))

	specContent := "---\ntype: spec\n---\n# My Spec\n\n[covers::Goal](goal.md)\n"
	goalContent := "---\ntype: goal\n---\n# My Goal\n"
	testutil.NilErr(t, os.WriteFile(filepath.Join(specsDir, "test.md"), []byte(specContent), 0o644))
	testutil.NilErr(t, os.WriteFile(filepath.Join(specsDir, "goal.md"), []byte(goalContent), 0o644))

	cfg := &config.TraceConfig{
		Types: []string{"spec", "goal"},
		Edges: map[string]config.TraceEdge{
			"covers": {From: "spec", To: "goal"},
		},
	}

	graph, errs := Validate(root, cfg)
	testutil.Len(t, errs, 0)
	testutil.True(t, len(graph.Documents) >= 2)
	testutil.True(t, len(graph.DirectEdges) >= 1)
}

func TestValidateNilConfig(t *testing.T) {
	graph, errs := Validate(t.TempDir(), nil)
	testutil.Len(t, errs, 0)
	testutil.Len(t, graph.Documents, 0)
}

func TestValidateUndeclaredType(t *testing.T) {
	root := t.TempDir()
	content := "---\ntype: unknown\n---\n# Doc\n"
	testutil.NilErr(t, os.WriteFile(filepath.Join(root, "doc.md"), []byte(content), 0o644))

	cfg := &config.TraceConfig{
		Types: []string{"spec"},
		Edges: map[string]config.TraceEdge{
			"covers": {From: "spec", To: "spec"},
		},
	}
	_, errs := Validate(root, cfg)
	testutil.True(t, len(errs) > 0)

	found := false
	for _, e := range errs {
		if e.Message != "" {
			found = true
		}
	}
	testutil.True(t, found)
}

func TestValidateDanglingLink(t *testing.T) {
	root := t.TempDir()
	content := "---\ntype: spec\n---\n# Doc\n\n[covers::Missing](missing.md)\n"
	testutil.NilErr(t, os.WriteFile(filepath.Join(root, "doc.md"), []byte(content), 0o644))

	cfg := &config.TraceConfig{
		Types: []string{"spec"},
		Edges: map[string]config.TraceEdge{
			"covers": {From: "spec", To: "spec"},
		},
	}
	_, errs := Validate(root, cfg)
	testutil.True(t, len(errs) > 0)
}

func TestValidateIgnoresHiddenDirs(t *testing.T) {
	root := t.TempDir()
	hidden := filepath.Join(root, ".hidden")
	testutil.NilErr(t, os.MkdirAll(hidden, 0o755))
	testutil.NilErr(t, os.WriteFile(filepath.Join(hidden, "doc.md"), []byte("---\ntype: bad\n---\n"), 0o644))

	cfg := &config.TraceConfig{
		Types: []string{"spec"},
		Edges: map[string]config.TraceEdge{"covers": {From: "spec", To: "spec"}},
	}
	_, errs := Validate(root, cfg)
	testutil.Len(t, errs, 0)
}
