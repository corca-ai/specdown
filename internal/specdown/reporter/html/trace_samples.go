package html

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/corca-ai/specdown/internal/specdown/core"
	"github.com/corca-ai/specdown/internal/specdown/trace"
)

// TraceSample holds a named sample graph with its classification.
type TraceSample struct {
	Name        string
	Description string
	Graph       trace.Graph
	Class       trace.Classification
}

// BuildTraceSamples returns realistic sample graphs for every layout class.
func BuildTraceSamples() []TraceSample {
	samples := []TraceSample{
		buildLinearChainSample(),
		buildFlatStarSample(),
		buildTreeSample(),
		buildForestSample(),
		buildLayeredDAGSample(),
		buildDiamondDAGSample(),
		buildCyclicSample(),
	}
	for i := range samples {
		samples[i].Class = trace.Classify(samples[i].Graph)
	}
	return samples
}

// WriteTraceSampleGallery generates an HTML gallery page showing all layout types.
func WriteTraceSampleGallery(outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	// Write shared assets.
	if err := os.WriteFile(filepath.Join(outDir, "style.css"), []byte(styleCSS), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "script.js"), []byte(scriptJS), 0o644); err != nil {
		return err
	}

	samples := BuildTraceSamples()

	// Build index page with links to each sample.
	var indexBody strings.Builder
	indexBody.WriteString(`<h1>Trace Layout Gallery</h1>`)
	indexBody.WriteString(`<p>Visual test page for each graph class and layout algorithm. Generated automatically from sample data.</p>`)
	indexBody.WriteString(`<ul>`)
	for _, s := range samples {
		slug := slugify(s.Name)
		fmt.Fprintf(&indexBody,
			`<li><a href="%s.html"><strong>%s</strong></a> — %s<br><code>class=%s layout=%s</code></li>`,
			slug, s.Name, s.Description, s.Class.Class, s.Class.Layout)
	}
	indexBody.WriteString(`</ul>`)

	indexView := pageView{
		Title:     "Trace Layout Gallery",
		AssetRoot: ".",
		GlobalTOC: buildGalleryTOC(samples, -1),
		Body:      htmlSafe(indexBody.String()),
	}
	if err := writeHTMLFile(filepath.Join(outDir, "index.html"), indexView); err != nil {
		return err
	}

	// Write one page per sample.
	for i, s := range samples {
		tg := sampleToTraceGraphData(s)
		report := core.Report{
			GeneratedAt: time.Now(),
			TraceGraph:  &tg,
		}

		var body strings.Builder
		fmt.Fprintf(&body, `<h1>%s</h1>`, s.Name)
		fmt.Fprintf(&body, `<p>%s</p>`, s.Description)
		fmt.Fprintf(&body, `<p class="trace-meta">Class: <strong>%s</strong> · Layout: <strong>%s</strong>`,
			tg.Class, tg.Layout)
		fmt.Fprintf(&body, ` · %d documents · %d edges`, len(tg.Documents), len(tg.Edges))
		if len(tg.Layers) > 0 {
			fmt.Fprintf(&body, ` · Layers: %s`, strings.Join(tg.Layers, " → "))
		}
		body.WriteString(`</p>`)
		body.WriteString(renderTraceGraph(&tg))
		body.WriteString(renderTraceMatrix(&tg))

		// Add sample trace errors for visual testing.
		errs := sampleTraceErrors(s)
		if len(errs) > 0 {
			report.TraceErrors = errs
			body.WriteString(`<h2>Trace Errors</h2>`)
			body.WriteString(`<ul class="trace-error-list">`)
			for _, e := range errs {
				fmt.Fprintf(&body, `<li class="trace-error">%s</li>`, e)
			}
			body.WriteString(`</ul>`)
		}

		slug := slugify(s.Name)
		view := pageView{
			Title:     s.Name + " — Trace Layout Gallery",
			AssetRoot: ".",
			GlobalTOC: buildGalleryTOC(samples, i),
			Body:      htmlSafe(body.String()),
		}
		if err := writeHTMLFile(filepath.Join(outDir, slug+".html"), view); err != nil {
			return err
		}
	}

	return nil
}

func buildGalleryTOC(samples []TraceSample, currentIdx int) []globalTocEntry {
	toc := make([]globalTocEntry, 0, len(samples)+1)
	toc = append(toc, globalTocEntry{
		Title:   "Gallery Index",
		Href:    "index.html",
		Current: currentIdx == -1,
	})
	for i, s := range samples {
		entry := globalTocEntry{
			Title:   s.Name,
			Snippet: string(s.Class.Class),
			Href:    slugify(s.Name) + ".html",
			Current: i == currentIdx,
		}
		if entry.Current {
			entry.Href = ""
		}
		toc = append(toc, entry)
	}
	// Clear href for current index entry.
	if currentIdx == -1 {
		toc[0].Href = ""
	}
	return toc
}

func sampleToTraceGraphData(s TraceSample) core.TraceGraphData {
	docs := make([]core.TraceDocument, len(s.Graph.Documents))
	for i, d := range s.Graph.Documents {
		docs[i] = core.TraceDocument{Path: d.Path, Type: d.Type}
	}
	edges := make([]core.TraceEdge, len(s.Graph.DirectEdges))
	for i, e := range s.Graph.DirectEdges {
		edges[i] = core.TraceEdge{Source: e.Source, Target: e.Target, EdgeName: e.EdgeName}
	}
	transitive := make([]core.TraceEdge, len(s.Graph.TransitiveEdges))
	for i, e := range s.Graph.TransitiveEdges {
		transitive[i] = core.TraceEdge{Source: e.Source, Target: e.Target, EdgeName: e.EdgeName}
	}
	return core.TraceGraphData{
		Documents:       docs,
		Edges:           edges,
		TransitiveEdges: transitive,
		Class:           string(s.Class.Class),
		Layout:          string(s.Class.Layout),
		Layers:          s.Class.Layers,
	}
}

func sampleTraceErrors(s TraceSample) []string {
	// Only add sample errors for the cyclic graph.
	if s.Class.Class == trace.ClassGeneral {
		return []string{
			"GRAPH: [depends] cycle detected — auth.spec.md → session.spec.md → auth.spec.md",
			"GRAPH: [depends] cardinality — spec \"logging.spec.md\" has 0 outgoing \"depends\" edges (expected 1..*)",
		}
	}
	return nil
}

func slugify(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "(", "")
	s = strings.ReplaceAll(s, ")", "")
	s = strings.TrimRight(s, "-")
	return s
}

func htmlSafe(s string) template.HTML {
	return template.HTML(s) //nolint:gosec // internally generated
}

// ── Sample graph builders ──

func buildLinearChainSample() TraceSample {
	return TraceSample{
		Name:        "Linear Chain",
		Description: "A pipeline of specs where each depends on the previous. Common for sequential workflows like parse → compile → run → report.",
		Graph: trace.Graph{
			Documents: []trace.TypedDocument{
				{Path: "parse.spec.md", Type: "spec"},
				{Path: "compile.spec.md", Type: "spec"},
				{Path: "execute.spec.md", Type: "spec"},
				{Path: "report.spec.md", Type: "spec"},
			},
			DirectEdges: []trace.Edge{
				{Source: "parse.spec.md", Target: "compile.spec.md", EdgeName: "depends"},
				{Source: "compile.spec.md", Target: "execute.spec.md", EdgeName: "depends"},
				{Source: "execute.spec.md", Target: "report.spec.md", EdgeName: "depends"},
			},
		},
	}
}

func buildFlatStarSample() TraceSample {
	return TraceSample{
		Name:        "Flat Star",
		Description: "A single index document that fans out to all feature specs. Typical for a project overview page.",
		Graph: trace.Graph{
			Documents: []trace.TypedDocument{
				{Path: "index.spec.md", Type: "index"},
				{Path: "auth.spec.md", Type: "spec"},
				{Path: "billing.spec.md", Type: "spec"},
				{Path: "notifications.spec.md", Type: "spec"},
				{Path: "search.spec.md", Type: "spec"},
				{Path: "admin.spec.md", Type: "spec"},
			},
			DirectEdges: []trace.Edge{
				{Source: "index.spec.md", Target: "auth.spec.md", EdgeName: "covers"},
				{Source: "index.spec.md", Target: "billing.spec.md", EdgeName: "covers"},
				{Source: "index.spec.md", Target: "notifications.spec.md", EdgeName: "covers"},
				{Source: "index.spec.md", Target: "search.spec.md", EdgeName: "covers"},
				{Source: "index.spec.md", Target: "admin.spec.md", EdgeName: "covers"},
			},
		},
	}
}

func buildTreeSample() TraceSample {
	return TraceSample{
		Name:        "Tree",
		Description: "A hierarchical spec tree with a root branching into subsystems, each with leaf specs. No shared children.",
		Graph: trace.Graph{
			Documents: []trace.TypedDocument{
				{Path: "platform.spec.md", Type: "spec"},
				{Path: "api.spec.md", Type: "spec"},
				{Path: "ui.spec.md", Type: "spec"},
				{Path: "rest.spec.md", Type: "spec"},
				{Path: "graphql.spec.md", Type: "spec"},
				{Path: "dashboard.spec.md", Type: "spec"},
				{Path: "settings.spec.md", Type: "spec"},
				{Path: "onboarding.spec.md", Type: "spec"},
			},
			DirectEdges: []trace.Edge{
				{Source: "platform.spec.md", Target: "api.spec.md", EdgeName: "depends"},
				{Source: "platform.spec.md", Target: "ui.spec.md", EdgeName: "depends"},
				{Source: "api.spec.md", Target: "rest.spec.md", EdgeName: "depends"},
				{Source: "api.spec.md", Target: "graphql.spec.md", EdgeName: "depends"},
				{Source: "ui.spec.md", Target: "dashboard.spec.md", EdgeName: "depends"},
				{Source: "ui.spec.md", Target: "settings.spec.md", EdgeName: "depends"},
				{Source: "ui.spec.md", Target: "onboarding.spec.md", EdgeName: "depends"},
			},
		},
	}
}

func buildForestSample() TraceSample {
	return TraceSample{
		Name:        "Forest",
		Description: "Disconnected spec trees for independent subsystems. Auth and Payments have separate hierarchies with no cross-links.",
		Graph: trace.Graph{
			Documents: []trace.TypedDocument{
				{Path: "auth.spec.md", Type: "spec"},
				{Path: "login.spec.md", Type: "spec"},
				{Path: "signup.spec.md", Type: "spec"},
				{Path: "oauth.spec.md", Type: "spec"},
				{Path: "payments.spec.md", Type: "spec"},
				{Path: "checkout.spec.md", Type: "spec"},
				{Path: "refunds.spec.md", Type: "spec"},
			},
			DirectEdges: []trace.Edge{
				{Source: "auth.spec.md", Target: "login.spec.md", EdgeName: "depends"},
				{Source: "auth.spec.md", Target: "signup.spec.md", EdgeName: "depends"},
				{Source: "auth.spec.md", Target: "oauth.spec.md", EdgeName: "depends"},
				{Source: "payments.spec.md", Target: "checkout.spec.md", EdgeName: "depends"},
				{Source: "payments.spec.md", Target: "refunds.spec.md", EdgeName: "depends"},
			},
		},
	}
}

func buildLayeredDAGSample() TraceSample {
	return TraceSample{
		Name:        "Layered DAG",
		Description: "A Theme → Epic → Story → Acceptance Test hierarchy. Types form strict layers; stories can be covered by multiple epics.",
		Graph: trace.Graph{
			Documents: []trace.TypedDocument{
				{Path: "user-management.md", Type: "theme"},
				{Path: "registration.md", Type: "epic"},
				{Path: "profile.md", Type: "epic"},
				{Path: "email-signup.md", Type: "story"},
				{Path: "social-login.md", Type: "story"},
				{Path: "edit-avatar.md", Type: "story"},
				{Path: "change-password.md", Type: "story"},
				{Path: "verify-email-flow.md", Type: "at"},
				{Path: "verify-oauth-flow.md", Type: "at"},
				{Path: "verify-avatar-upload.md", Type: "at"},
				{Path: "verify-password-reset.md", Type: "at"},
			},
			DirectEdges: []trace.Edge{
				{Source: "user-management.md", Target: "registration.md", EdgeName: "owns"},
				{Source: "user-management.md", Target: "profile.md", EdgeName: "owns"},
				{Source: "registration.md", Target: "email-signup.md", EdgeName: "covers"},
				{Source: "registration.md", Target: "social-login.md", EdgeName: "covers"},
				{Source: "profile.md", Target: "edit-avatar.md", EdgeName: "covers"},
				{Source: "profile.md", Target: "change-password.md", EdgeName: "covers"},
				// Cross-link: registration also covers social-login's password aspect
				{Source: "profile.md", Target: "social-login.md", EdgeName: "covers"},
				{Source: "email-signup.md", Target: "verify-email-flow.md", EdgeName: "tests"},
				{Source: "social-login.md", Target: "verify-oauth-flow.md", EdgeName: "tests"},
				{Source: "edit-avatar.md", Target: "verify-avatar-upload.md", EdgeName: "tests"},
				{Source: "change-password.md", Target: "verify-password-reset.md", EdgeName: "tests"},
			},
		},
	}
}

func buildDiamondDAGSample() TraceSample {
	return TraceSample{
		Name:        "Diamond DAG",
		Description: "Specs with shared dependencies forming diamonds. Auth and Database are both needed by API and Worker, creating convergence points.",
		Graph: trace.Graph{
			Documents: []trace.TypedDocument{
				{Path: "config.spec.md", Type: "spec"},
				{Path: "auth.spec.md", Type: "spec"},
				{Path: "database.spec.md", Type: "spec"},
				{Path: "api.spec.md", Type: "spec"},
				{Path: "worker.spec.md", Type: "spec"},
				{Path: "integration.spec.md", Type: "spec"},
				{Path: "logging.spec.md", Type: "spec"},
			},
			DirectEdges: []trace.Edge{
				{Source: "config.spec.md", Target: "auth.spec.md", EdgeName: "depends"},
				{Source: "config.spec.md", Target: "database.spec.md", EdgeName: "depends"},
				{Source: "config.spec.md", Target: "logging.spec.md", EdgeName: "depends"},
				{Source: "auth.spec.md", Target: "api.spec.md", EdgeName: "depends"},
				{Source: "database.spec.md", Target: "api.spec.md", EdgeName: "depends"},
				{Source: "database.spec.md", Target: "worker.spec.md", EdgeName: "depends"},
				{Source: "logging.spec.md", Target: "api.spec.md", EdgeName: "depends"},
				{Source: "logging.spec.md", Target: "worker.spec.md", EdgeName: "depends"},
				{Source: "api.spec.md", Target: "integration.spec.md", EdgeName: "depends"},
				{Source: "worker.spec.md", Target: "integration.spec.md", EdgeName: "depends"},
			},
		},
	}
}

func buildCyclicSample() TraceSample {
	return TraceSample{
		Name:        "General (Cyclic)",
		Description: "Specs with mutual dependencies forming cycles. Auth depends on Session, Session depends on Auth. Falls back to matrix layout.",
		Graph: trace.Graph{
			Documents: []trace.TypedDocument{
				{Path: "auth.spec.md", Type: "spec"},
				{Path: "session.spec.md", Type: "spec"},
				{Path: "permissions.spec.md", Type: "spec"},
				{Path: "audit.spec.md", Type: "spec"},
				{Path: "logging.spec.md", Type: "spec"},
			},
			DirectEdges: []trace.Edge{
				// Cycle: auth ↔ session
				{Source: "auth.spec.md", Target: "session.spec.md", EdgeName: "depends"},
				{Source: "session.spec.md", Target: "auth.spec.md", EdgeName: "depends"},
				{Source: "auth.spec.md", Target: "permissions.spec.md", EdgeName: "depends"},
				{Source: "permissions.spec.md", Target: "audit.spec.md", EdgeName: "depends"},
				{Source: "audit.spec.md", Target: "logging.spec.md", EdgeName: "depends"},
				// Another cycle: audit → session
				{Source: "audit.spec.md", Target: "session.spec.md", EdgeName: "depends"},
			},
		},
	}
}
