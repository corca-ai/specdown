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

// BuildTraceSamples returns realistic sample graphs covering tree, forest,
// DAG, and cyclic structures.
func BuildTraceSamples() []TraceSample {
	samples := []TraceSample{
		buildTreeSample(),
		buildForestSample(),
		buildDAGSample(),
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
			`<li><a href="%s.html"><strong>%s</strong></a> — %s</li>`,
			slug, s.Name, s.Description)
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
		body.WriteString(renderTraceGraph(&tg))

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
	}
}

func sampleTraceErrors(s TraceSample) []string {
	// Only add sample errors for the cyclic graph.
	if s.Class.Class == trace.ClassCyclic {
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

func buildTreeSample() TraceSample {
	return TraceSample{
		Name:        "Tree",
		Description: "A product spec hierarchy: theme → epics → user stories → acceptance tests.",
		Graph: trace.Graph{
			Documents: []trace.TypedDocument{
				{Path: "user-management.md", Type: "theme"},
				{Path: "registration.spec.md", Type: "epic"},
				{Path: "profile.spec.md", Type: "epic"},
				{Path: "user-can-sign-up-with-email-and-password.spec.md", Type: "story"},
				{Path: "user-can-sign-up-with-google-oauth.spec.md", Type: "story"},
				{Path: "user-can-reset-forgotten-password-via-email.spec.md", Type: "story"},
				{Path: "user-can-update-display-name-and-avatar.spec.md", Type: "story"},
				{Path: "user-can-change-password-from-settings.spec.md", Type: "story"},
				{Path: "user-can-delete-account-and-export-data.spec.md", Type: "story"},
				{Path: "verify-email-signup-flow.spec.md", Type: "at"},
				{Path: "verify-oauth-redirect-and-token-exchange.spec.md", Type: "at"},
				{Path: "verify-password-reset-email-delivery.spec.md", Type: "at"},
				{Path: "verify-avatar-upload-and-resize.spec.md", Type: "at"},
				{Path: "verify-account-deletion-purges-all-data.spec.md", Type: "at"},
				{Path: "registration-guide.md", Type: "guide"},
			},
			DirectEdges: []trace.Edge{
				{Source: "user-management.md", Target: "registration.spec.md", EdgeName: "owns"},
				{Source: "user-management.md", Target: "profile.spec.md", EdgeName: "owns"},
				{Source: "registration.spec.md", Target: "user-can-sign-up-with-email-and-password.spec.md", EdgeName: "covers"},
				{Source: "registration.spec.md", Target: "user-can-sign-up-with-google-oauth.spec.md", EdgeName: "covers"},
				{Source: "registration.spec.md", Target: "user-can-reset-forgotten-password-via-email.spec.md", EdgeName: "covers"},
				{Source: "registration.spec.md", Target: "registration-guide.md", EdgeName: "explains"},
				{Source: "profile.spec.md", Target: "user-can-update-display-name-and-avatar.spec.md", EdgeName: "covers"},
				{Source: "profile.spec.md", Target: "user-can-change-password-from-settings.spec.md", EdgeName: "covers"},
				{Source: "profile.spec.md", Target: "user-can-delete-account-and-export-data.spec.md", EdgeName: "covers"},
				{Source: "user-can-sign-up-with-email-and-password.spec.md", Target: "verify-email-signup-flow.spec.md", EdgeName: "tests"},
				{Source: "user-can-sign-up-with-google-oauth.spec.md", Target: "verify-oauth-redirect-and-token-exchange.spec.md", EdgeName: "tests"},
				{Source: "user-can-reset-forgotten-password-via-email.spec.md", Target: "verify-password-reset-email-delivery.spec.md", EdgeName: "tests"},
				{Source: "user-can-update-display-name-and-avatar.spec.md", Target: "verify-avatar-upload-and-resize.spec.md", EdgeName: "tests"},
				{Source: "user-can-delete-account-and-export-data.spec.md", Target: "verify-account-deletion-purges-all-data.spec.md", EdgeName: "tests"},
			},
		},
	}
}

func buildForestSample() TraceSample {
	return TraceSample{
		Name:        "Forest",
		Description: "Independent subsystems with no cross-links between them.",
		Graph: trace.Graph{
			Documents: []trace.TypedDocument{
				{Path: "authentication.spec.md", Type: "epic"},
				{Path: "user-can-log-in-with-email.spec.md", Type: "story"},
				{Path: "user-can-log-in-with-sso.spec.md", Type: "story"},
				{Path: "session-expires-after-30-minutes-of-inactivity.spec.md", Type: "story"},
				{Path: "sso-integration-guide.md", Type: "guide"},
				{Path: "billing.spec.md", Type: "epic"},
				{Path: "user-can-subscribe-to-a-paid-plan.spec.md", Type: "story"},
				{Path: "user-can-cancel-subscription-and-get-prorated-refund.spec.md", Type: "story"},
				{Path: "system-sends-invoice-on-each-billing-cycle.spec.md", Type: "story"},
				{Path: "verify-stripe-webhook-handles-payment-failure.spec.md", Type: "at"},
				{Path: "notifications.spec.md", Type: "epic"},
				{Path: "user-receives-email-on-new-comment.spec.md", Type: "story"},
				{Path: "user-can-mute-notifications-per-channel.spec.md", Type: "story"},
			},
			DirectEdges: []trace.Edge{
				{Source: "authentication.spec.md", Target: "user-can-log-in-with-email.spec.md", EdgeName: "covers"},
				{Source: "authentication.spec.md", Target: "user-can-log-in-with-sso.spec.md", EdgeName: "covers"},
				{Source: "authentication.spec.md", Target: "session-expires-after-30-minutes-of-inactivity.spec.md", EdgeName: "covers"},
				{Source: "authentication.spec.md", Target: "sso-integration-guide.md", EdgeName: "explains"},
				{Source: "billing.spec.md", Target: "user-can-subscribe-to-a-paid-plan.spec.md", EdgeName: "covers"},
				{Source: "billing.spec.md", Target: "user-can-cancel-subscription-and-get-prorated-refund.spec.md", EdgeName: "covers"},
				{Source: "billing.spec.md", Target: "system-sends-invoice-on-each-billing-cycle.spec.md", EdgeName: "covers"},
				{Source: "system-sends-invoice-on-each-billing-cycle.spec.md", Target: "verify-stripe-webhook-handles-payment-failure.spec.md", EdgeName: "tests"},
				{Source: "notifications.spec.md", Target: "user-receives-email-on-new-comment.spec.md", EdgeName: "covers"},
				{Source: "notifications.spec.md", Target: "user-can-mute-notifications-per-channel.spec.md", EdgeName: "covers"},
			},
		},
	}
}

func buildDAGSample() TraceSample {
	return TraceSample{
		Name:        "DAG",
		Description: "Shared infrastructure specs depended on by multiple features, forming diamonds.",
		Graph: trace.Graph{
			Documents: []trace.TypedDocument{
				{Path: "platform-overview.md", Type: "guide"},
				{Path: "authentication.spec.md", Type: "spec"},
				{Path: "database.spec.md", Type: "spec"},
				{Path: "api-gateway.spec.md", Type: "spec"},
				{Path: "background-worker.spec.md", Type: "spec"},
				{Path: "logging-and-observability.spec.md", Type: "spec"},
				{Path: "user-can-query-api-with-valid-token.spec.md", Type: "story"},
				{Path: "worker-retries-failed-jobs-up-to-3-times.spec.md", Type: "story"},
				{Path: "integration-test-suite.spec.md", Type: "at"},
			},
			DirectEdges: []trace.Edge{
				{Source: "platform-overview.md", Target: "authentication.spec.md", EdgeName: "covers"},
				{Source: "platform-overview.md", Target: "database.spec.md", EdgeName: "covers"},
				{Source: "platform-overview.md", Target: "logging-and-observability.spec.md", EdgeName: "covers"},
				{Source: "authentication.spec.md", Target: "api-gateway.spec.md", EdgeName: "depends"},
				{Source: "database.spec.md", Target: "api-gateway.spec.md", EdgeName: "depends"},
				{Source: "database.spec.md", Target: "background-worker.spec.md", EdgeName: "depends"},
				{Source: "logging-and-observability.spec.md", Target: "api-gateway.spec.md", EdgeName: "depends"},
				{Source: "logging-and-observability.spec.md", Target: "background-worker.spec.md", EdgeName: "depends"},
				{Source: "api-gateway.spec.md", Target: "user-can-query-api-with-valid-token.spec.md", EdgeName: "covers"},
				{Source: "background-worker.spec.md", Target: "worker-retries-failed-jobs-up-to-3-times.spec.md", EdgeName: "covers"},
				{Source: "api-gateway.spec.md", Target: "integration-test-suite.spec.md", EdgeName: "tests"},
				{Source: "background-worker.spec.md", Target: "integration-test-suite.spec.md", EdgeName: "tests"},
			},
		},
	}
}

func buildCyclicSample() TraceSample {
	return TraceSample{
		Name:        "Cyclic",
		Description: "Mutual dependencies between specs that form cycles.",
		Graph: trace.Graph{
			Documents: []trace.TypedDocument{
				{Path: "authentication.spec.md", Type: "spec"},
				{Path: "session-management.spec.md", Type: "spec"},
				{Path: "role-based-access-control.spec.md", Type: "spec"},
				{Path: "audit-trail.spec.md", Type: "spec"},
				{Path: "security-guide.md", Type: "guide"},
			},
			DirectEdges: []trace.Edge{
				{Source: "authentication.spec.md", Target: "session-management.spec.md", EdgeName: "depends"},
				{Source: "session-management.spec.md", Target: "authentication.spec.md", EdgeName: "depends"},
				{Source: "authentication.spec.md", Target: "role-based-access-control.spec.md", EdgeName: "depends"},
				{Source: "role-based-access-control.spec.md", Target: "audit-trail.spec.md", EdgeName: "depends"},
				{Source: "audit-trail.spec.md", Target: "security-guide.md", EdgeName: "explains"},
				{Source: "audit-trail.spec.md", Target: "session-management.spec.md", EdgeName: "depends"},
			},
		},
	}
}
