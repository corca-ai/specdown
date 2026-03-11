package trace

import (
	"testing"

	"github.com/corca-ai/specdown/internal/specdown/config"
)

func TestParseTraceLinks_SimpleLinks(t *testing.T) {
	md := `See [covers::Feature X](feature.md) and [depends::Other](other.md).`
	links := ParseTraceLinks("test.md", md)
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}
	if links[0].EdgeName != "covers" || links[0].TargetPath != "feature.md" {
		t.Errorf("link[0] = %+v", links[0])
	}
	if links[1].EdgeName != "depends" || links[1].DisplayText != "Other" {
		t.Errorf("link[1] = %+v", links[1])
	}
}

func TestParseTraceLinks_CodeBlocksIgnored(t *testing.T) {
	md := "```\n[covers::X](y.md)\n```\n[covers::Z](z.md)"
	links := ParseTraceLinks("test.md", md)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].DisplayText != "Z" {
		t.Errorf("expected display text Z, got %q", links[0].DisplayText)
	}
}

func TestParseTraceLinks_ExternalURLs(t *testing.T) {
	md := `No links here, just [covers::Docs](https://example.com)`
	links := ParseTraceLinks("test.md", md)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].TargetPath != "https://example.com" {
		t.Errorf("expected raw URL, got %q", links[0].TargetPath)
	}
}

func TestParseTraceLinks_FragmentStripped(t *testing.T) {
	md := `[covers::Section](doc.md#heading)`
	links := ParseTraceLinks("test.md", md)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].TargetPath != "doc.md" {
		t.Errorf("expected doc.md, got %q", links[0].TargetPath)
	}
}

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
		if len(errs) != 0 {
			t.Fatalf("expected no errors, got %v", errs)
		}
		if len(edges) != 1 || edges[0].Source != "feat.md" || edges[0].Target != "goal.md" {
			t.Errorf("unexpected edges: %+v", edges)
		}
	})

	t.Run("unknown edge", func(t *testing.T) {
		link := TraceLink{SourcePath: "feat.md", SourceLine: 1, EdgeName: "unknown", TargetPath: "goal.md"}
		_, errs := validateLink(link, cfg, docs)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d", len(errs))
		}
	})

	t.Run("type mismatch", func(t *testing.T) {
		link := TraceLink{SourcePath: "goal.md", SourceLine: 1, EdgeName: "covers", TargetPath: "feat.md"}
		_, errs := validateLink(link, cfg, docs)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d", len(errs))
		}
	})

	t.Run("dangling target", func(t *testing.T) {
		link := TraceLink{SourcePath: "feat.md", SourceLine: 1, EdgeName: "covers", TargetPath: "missing.md"}
		_, errs := validateLink(link, cfg, docs)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d", len(errs))
		}
	})

	t.Run("self-loop", func(t *testing.T) {
		selfDocs := map[string]TypedDocument{
			"feat.md": {Path: "feat.md", Type: "feature"},
		}
		selfCfg := &config.TraceConfig{
			Types: []string{"feature"},
			Edges: map[string]config.TraceEdge{
				"covers": {From: "feature", To: "feature"},
			},
		}
		link := TraceLink{SourcePath: "feat.md", SourceLine: 1, EdgeName: "covers", TargetPath: "feat.md"}
		_, errs := validateLink(link, selfCfg, selfDocs)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d", len(errs))
		}
	})
}

func TestDetectCycles(t *testing.T) {
	t.Run("no cycle", func(t *testing.T) {
		edges := []Edge{
			{Source: "a", Target: "b", EdgeName: "e"},
			{Source: "b", Target: "c", EdgeName: "e"},
		}
		cycles := detectCycles(edges)
		if len(cycles) != 0 {
			t.Errorf("expected no cycles, got %v", cycles)
		}
	})

	t.Run("simple cycle", func(t *testing.T) {
		edges := []Edge{
			{Source: "a", Target: "b", EdgeName: "e"},
			{Source: "b", Target: "a", EdgeName: "e"},
		}
		cycles := detectCycles(edges)
		if len(cycles) != 1 {
			t.Fatalf("expected 1 cycle, got %d", len(cycles))
		}
	})

	t.Run("long cycle", func(t *testing.T) {
		edges := []Edge{
			{Source: "a", Target: "b", EdgeName: "e"},
			{Source: "b", Target: "c", EdgeName: "e"},
			{Source: "c", Target: "a", EdgeName: "e"},
		}
		cycles := detectCycles(edges)
		if len(cycles) != 1 {
			t.Fatalf("expected 1 cycle, got %d", len(cycles))
		}
	})
}

func TestComputeTransitiveClosure(t *testing.T) {
	t.Run("chain", func(t *testing.T) {
		edges := []Edge{
			{Source: "a", Target: "b", EdgeName: "dep"},
			{Source: "b", Target: "c", EdgeName: "dep"},
		}
		transitive := computeTransitiveClosure(edges, "dep")
		if len(transitive) != 1 {
			t.Fatalf("expected 1 transitive edge, got %d", len(transitive))
		}
		if transitive[0].Source != "a" || transitive[0].Target != "c" {
			t.Errorf("expected a->c, got %+v", transitive[0])
		}
	})

	t.Run("diamond", func(t *testing.T) {
		edges := []Edge{
			{Source: "a", Target: "b", EdgeName: "dep"},
			{Source: "a", Target: "c", EdgeName: "dep"},
			{Source: "b", Target: "d", EdgeName: "dep"},
			{Source: "c", Target: "d", EdgeName: "dep"},
		}
		transitive := computeTransitiveClosure(edges, "dep")
		if len(transitive) != 1 {
			t.Fatalf("expected 1 transitive edge (a->d), got %d", len(transitive))
		}
		if transitive[0].Source != "a" || transitive[0].Target != "d" {
			t.Errorf("expected a->d, got %+v", transitive[0])
		}
	})
}

func TestMatchGlob(t *testing.T) {
	t.Run("simple pattern", func(t *testing.T) {
		if !matchGlob("*.md", "readme.md") {
			t.Error("expected match")
		}
	})

	t.Run("double star", func(t *testing.T) {
		if !matchGlob("docs/**/*.md", "docs/api/guide.md") {
			t.Error("expected match")
		}
	})

	t.Run("non-match", func(t *testing.T) {
		if matchGlob("*.txt", "readme.md") {
			t.Error("expected no match")
		}
	})
}

func TestValidateCardinality(t *testing.T) {
	t.Run("satisfied", func(t *testing.T) {
		if !satisfiesMultiplicity(1, config.Multiplicity{Min: 1, Max: -1}) {
			t.Error("expected satisfied")
		}
	})

	t.Run("violated min", func(t *testing.T) {
		if satisfiesMultiplicity(0, config.Multiplicity{Min: 1, Max: -1}) {
			t.Error("expected violation")
		}
	})

	t.Run("violated max", func(t *testing.T) {
		if satisfiesMultiplicity(3, config.Multiplicity{Min: 0, Max: 2}) {
			t.Error("expected violation")
		}
	})
}
