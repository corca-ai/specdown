package trace

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/corca-ai/specdown/internal/specdown/config"
	"github.com/corca-ai/specdown/internal/specdown/core"
)

// TraceLink represents a single trace link found in a document.
type TraceLink struct {
	SourcePath  string // relative path of source document
	SourceLine  int    // 1-based line number
	EdgeName    string // the edge name (e.g. "covers")
	TargetPath  string // resolved relative path of target
	DisplayText string // display text after "::"
}

// TypedDocument represents a discovered document with its type.
type TypedDocument struct {
	Path string // relative path from project root
	Type string // frontmatter type, empty if untyped
}

// TraceError represents a validation error in the trace graph.
type TraceError struct {
	File    string // source file (or "GRAPH:" for graph-level)
	Line    int    // 0 for graph-level errors
	Edge    string // edge name
	Message string
}

func (e TraceError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: [%s] %s", e.File, e.Line, e.Edge, e.Message)
	}
	if e.File == "GRAPH" {
		return fmt.Sprintf("GRAPH: [%s] %s", e.Edge, e.Message)
	}
	return fmt.Sprintf("%s: [%s] %s", e.File, e.Edge, e.Message)
}

// Edge represents a direct edge in the trace graph.
type Edge struct {
	Source   string // source document path
	Target   string // target document path
	EdgeName string // edge type name
}

// Graph represents the full trace graph.
type Graph struct {
	Documents       []TypedDocument
	DirectEdges     []Edge
	TransitiveEdges []Edge // edges derived from transitive closure (not direct)
}

// traceLinkPattern matches [edgeName::display text](target) in markdown links.
var traceLinkPattern = regexp.MustCompile(`\[([a-z][a-z0-9_-]*)::([^\]]+)\]\(([^)]+)\)`)

// fencedCodeBlockPattern strips fenced code blocks from markdown.
var fencedCodeBlockPattern = regexp.MustCompile("(?m)^```[^\n]*\n(?s:.*?)\n```\\s*$")

// ParseTraceLinks extracts trace links from markdown content.
func ParseTraceLinks(sourcePath, markdown string) []TraceLink {
	stripped := fencedCodeBlockPattern.ReplaceAllStringFunc(markdown, func(block string) string {
		return strings.Repeat("\n", strings.Count(block, "\n"))
	})

	var links []TraceLink
	lines := strings.Split(stripped, "\n")
	for lineNum, line := range lines {
		for _, match := range traceLinkPattern.FindAllStringSubmatch(line, -1) {
			displayText := strings.TrimSpace(match[2])
			if displayText == "" {
				continue
			}
			rawTarget := match[3]
			if idx := strings.Index(rawTarget, "#"); idx >= 0 {
				rawTarget = rawTarget[:idx]
			}
			links = append(links, TraceLink{
				SourcePath:  sourcePath,
				SourceLine:  lineNum + 1,
				EdgeName:    match[1],
				TargetPath:  rawTarget,
				DisplayText: displayText,
			})
		}
	}
	return links
}

// Validate performs full trace validation.
//
//nolint:gocognit // orchestrator function
func Validate(baseDir string, traceConfig *config.TraceConfig) (Graph, []TraceError) {
	if traceConfig == nil {
		return Graph{}, nil
	}

	docs, links, discoveryErrs := discover(baseDir, traceConfig)
	if len(discoveryErrs) > 0 {
		return Graph{}, discoveryErrs
	}

	docByPath := make(map[string]TypedDocument, len(docs))
	for _, doc := range docs {
		docByPath[doc.Path] = doc
	}

	var validEdges []Edge
	var linkErrs []TraceError
	for _, link := range links {
		edges, errs := validateLink(link, traceConfig, docByPath)
		linkErrs = append(linkErrs, errs...)
		validEdges = append(validEdges, edges...)
	}
	var dedupErrs []TraceError
	validEdges, dedupErrs = deduplicateEdges(validEdges)

	graphErrs := validateGraph(docs, validEdges, traceConfig)

	var allErrs []TraceError
	allErrs = append(allErrs, linkErrs...)
	allErrs = append(allErrs, dedupErrs...)
	allErrs = append(allErrs, graphErrs...)

	var transitiveEdges []Edge
	for edgeName, edgeCfg := range traceConfig.Edges {
		if edgeCfg.Transitive {
			transitiveEdges = append(transitiveEdges, computeTransitiveClosure(validEdges, edgeName)...)
		}
	}

	return Graph{
		Documents:       docs,
		DirectEdges:     validEdges,
		TransitiveEdges: transitiveEdges,
	}, allErrs
}

// discoverResult accumulates discovery state.
type discoverResult struct {
	docs  []TypedDocument
	links []TraceLink
	errs  []TraceError
}

// discover scans the directory tree and finds all .md files with types and trace links.
func discover(baseDir string, traceConfig *config.TraceConfig) ([]TypedDocument, []TraceLink, []TraceError) {
	typeSet := make(map[string]struct{}, len(traceConfig.Types))
	for _, t := range traceConfig.Types {
		typeSet[t] = struct{}{}
	}

	result := &discoverResult{}

	err := filepath.Walk(baseDir, func(absPath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil //nolint:nilerr // skip walk errors
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(info.Name(), ".md") {
			discoverFile(absPath, baseDir, traceConfig.Ignore, typeSet, result)
		}
		return nil
	})
	if err != nil {
		result.errs = append(result.errs, TraceError{
			File:    baseDir,
			Message: fmt.Sprintf("directory scan failed: %v", err),
		})
	}

	return result.docs, result.links, result.errs
}

func discoverFile(absPath, baseDir string, ignorePatterns []string, typeSet map[string]struct{}, result *discoverResult) {
	relPath, relErr := filepath.Rel(baseDir, absPath)
	if relErr != nil {
		return
	}
	relPath = filepath.ToSlash(relPath)
	if isIgnored(relPath, ignorePatterns) {
		return
	}

	body, readErr := os.ReadFile(absPath)
	if readErr != nil {
		return
	}
	markdown := string(body)

	doc, _ := core.ParseDocument(relPath, markdown, nil)
	docType := doc.Frontmatter.Type

	if docType != "" {
		if _, ok := typeSet[docType]; !ok {
			result.errs = append(result.errs, TraceError{
				File:    relPath,
				Message: fmt.Sprintf("undeclared type %q (not in trace.types)", docType),
			})
		}
	}

	result.docs = append(result.docs, TypedDocument{Path: relPath, Type: docType})
	resolved, resolveErrs := resolveTraceLinks(relPath, markdown)
	result.links = append(result.links, resolved...)
	result.errs = append(result.errs, resolveErrs...)
}

// resolveTraceLinks parses trace links from a document and resolves their target paths.
func resolveTraceLinks(relPath, markdown string) ([]TraceLink, []TraceError) {
	docLinks := ParseTraceLinks(relPath, markdown)
	var resolved []TraceLink
	var errs []TraceError

	sourceDir := path.Dir(relPath)
	for _, link := range docLinks {
		if strings.Contains(link.TargetPath, "://") {
			continue
		}
		target := path.Clean(path.Join(sourceDir, link.TargetPath))
		if strings.HasPrefix(target, "..") {
			errs = append(errs, TraceError{
				File:    relPath,
				Line:    link.SourceLine,
				Edge:    link.EdgeName,
				Message: fmt.Sprintf("target %q resolves outside project root", link.TargetPath),
			})
			continue
		}
		link.TargetPath = target
		resolved = append(resolved, link)
	}
	return resolved, errs
}

// isIgnored checks if a path matches any ignore pattern.
func isIgnored(relPath string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, relPath); matched {
			return true
		}
		if matchGlob(pattern, relPath) {
			return true
		}
	}
	return false
}

// validateLink checks a single trace link against the config and document registry.
func validateLink(link TraceLink, traceConfig *config.TraceConfig, docByPath map[string]TypedDocument) ([]Edge, []TraceError) {
	edgeCfg, ok := traceConfig.Edges[link.EdgeName]
	if !ok {
		return nil, []TraceError{{
			File: link.SourcePath, Line: link.SourceLine, Edge: link.EdgeName,
			Message: fmt.Sprintf("unknown edge name %q", link.EdgeName),
		}}
	}

	sourceDoc, ok := docByPath[link.SourcePath]
	if !ok || sourceDoc.Type == "" {
		return nil, []TraceError{{
			File: link.SourcePath, Line: link.SourceLine, Edge: link.EdgeName,
			Message: "source document has no type (trace links require a typed document)",
		}}
	}

	if sourceDoc.Type != edgeCfg.From {
		return nil, []TraceError{{
			File: link.SourcePath, Line: link.SourceLine, Edge: link.EdgeName,
			Message: fmt.Sprintf("type mismatch — source type %q cannot use edge %q (expected %q)", sourceDoc.Type, link.EdgeName, edgeCfg.From),
		}}
	}

	if err := validateLinkTarget(link, edgeCfg, docByPath); err != nil {
		return nil, []TraceError{*err}
	}

	return []Edge{{Source: link.SourcePath, Target: link.TargetPath, EdgeName: link.EdgeName}}, nil
}

// validateLinkTarget checks the target side of a trace link.
func validateLinkTarget(link TraceLink, edgeCfg config.TraceEdge, docByPath map[string]TypedDocument) *TraceError {
	targetDoc, ok := docByPath[link.TargetPath]
	if !ok {
		return &TraceError{
			File: link.SourcePath, Line: link.SourceLine, Edge: link.EdgeName,
			Message: fmt.Sprintf("dangling reference — target %q does not exist", link.TargetPath),
		}
	}
	if targetDoc.Type == "" {
		return &TraceError{
			File: link.SourcePath, Line: link.SourceLine, Edge: link.EdgeName,
			Message: fmt.Sprintf("target %q has no type", link.TargetPath),
		}
	}
	if targetDoc.Type != edgeCfg.To {
		return &TraceError{
			File: link.SourcePath, Line: link.SourceLine, Edge: link.EdgeName,
			Message: fmt.Sprintf("type mismatch — target type %q does not match edge %q (expected %q)", targetDoc.Type, link.EdgeName, edgeCfg.To),
		}
	}
	if link.SourcePath == link.TargetPath {
		return &TraceError{
			File: link.SourcePath, Line: link.SourceLine, Edge: link.EdgeName,
			Message: "self-loop — source and target are the same document",
		}
	}
	return nil
}

// validateGraph performs graph-level validation: cardinality and cycle detection.
func validateGraph(docs []TypedDocument, edges []Edge, traceConfig *config.TraceConfig) []TraceError {
	var errs []TraceError
	for edgeName, edgeCfg := range traceConfig.Edges {
		edgeTypeEdges := filterEdgesByName(edges, edgeName)
		errs = append(errs, checkCardinality(docs, edgeTypeEdges, edgeName, edgeCfg)...)
		errs = append(errs, checkCycles(edges, edgeTypeEdges, edgeName, edgeCfg)...)
	}
	return errs
}

// filterEdgesByName returns edges matching the given edge name.
func filterEdgesByName(edges []Edge, name string) []Edge {
	var result []Edge
	for _, e := range edges {
		if e.EdgeName == name {
			result = append(result, e)
		}
	}
	return result
}

// checkCardinality validates cardinality constraints for an edge type.
func checkCardinality(docs []TypedDocument, edges []Edge, edgeName string, edgeCfg config.TraceEdge) []TraceError {
	if edgeCfg.Count == "" {
		return nil
	}
	sourceMult, targetMult, err := config.ParseCount(edgeCfg.Count)
	if err != nil {
		return nil
	}

	// UML convention: left (source) mult = how many sources each target has;
	// right (target) mult = how many targets each source has.
	var errs []TraceError
	errs = append(errs, checkSideCardinality(docs, edges, edgeName, edgeCfg.From, targetMult, true)...)
	errs = append(errs, checkSideCardinality(docs, edges, edgeName, edgeCfg.To, sourceMult, false)...)
	return errs
}

// checkSideCardinality checks source-side or target-side cardinality.
func checkSideCardinality(docs []TypedDocument, edges []Edge, edgeName, docType string, mult config.Multiplicity, isSource bool) []TraceError {
	var errs []TraceError
	for _, doc := range docs {
		if doc.Type != docType {
			continue
		}
		count := countEdgesForDoc(edges, doc.Path, isSource)
		if !satisfiesMultiplicity(count, mult) {
			direction := "incoming"
			if isSource {
				direction = "outgoing"
			}
			errs = append(errs, TraceError{
				File:    "GRAPH",
				Edge:    edgeName,
				Message: fmt.Sprintf("cardinality — %s %q has %d %s %q edges (expected %s)", docType, doc.Path, count, direction, edgeName, formatMultiplicity(mult)),
			})
		}
	}
	return errs
}

func countEdgesForDoc(edges []Edge, docPath string, isSource bool) int {
	count := 0
	for _, e := range edges {
		if isSource && e.Source == docPath {
			count++
		} else if !isSource && e.Target == docPath {
			count++
		}
	}
	return count
}

// checkCycles runs cycle detection for an acyclic edge type.
func checkCycles(allEdges, edgeTypeEdges []Edge, edgeName string, edgeCfg config.TraceEdge) []TraceError {
	if !edgeCfg.Acyclic {
		return nil
	}
	var checkEdges []Edge
	if edgeCfg.Transitive {
		checkEdges = append(checkEdges, edgeTypeEdges...)
		checkEdges = append(checkEdges, computeTransitiveClosure(allEdges, edgeName)...)
	} else {
		checkEdges = edgeTypeEdges
	}

	var errs []TraceError
	for _, cycle := range detectCycles(checkEdges) {
		errs = append(errs, TraceError{
			File:    "GRAPH",
			Edge:    edgeName,
			Message: fmt.Sprintf("cycle detected — %s", strings.Join(cycle, " → ")),
		})
	}
	return errs
}

func satisfiesMultiplicity(count int, m config.Multiplicity) bool {
	if count < m.Min {
		return false
	}
	return m.Max < 0 || count <= m.Max
}

func formatMultiplicity(m config.Multiplicity) string {
	if m.Max < 0 {
		if m.Min == 0 {
			return "0..*"
		}
		return fmt.Sprintf("%d..*", m.Min)
	}
	if m.Min == m.Max {
		return fmt.Sprintf("%d", m.Min)
	}
	return fmt.Sprintf("%d..%d", m.Min, m.Max)
}

func deduplicateEdges(edges []Edge) ([]Edge, []TraceError) {
	type edgeKey struct{ name, source, target string }
	counts := make(map[edgeKey]int)
	for _, e := range edges {
		counts[edgeKey{e.EdgeName, e.Source, e.Target}]++
	}

	var errs []TraceError
	seen := make(map[edgeKey]struct{})
	var result []Edge
	for _, e := range edges {
		k := edgeKey{e.EdgeName, e.Source, e.Target}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		result = append(result, e)
		if n := counts[k]; n > 1 {
			errs = append(errs, TraceError{
				File:    e.Source,
				Edge:    e.EdgeName,
				Message: fmt.Sprintf("duplicate link to %s (%d occurrences)", e.Target, n),
			})
		}
	}
	return result, errs
}

// detectCycles finds cycles in the given edges using DFS.
func detectCycles(edges []Edge) [][]string {
	adj := make(map[string][]string)
	nodeSet := make(map[string]struct{})
	for _, e := range edges {
		adj[e.Source] = append(adj[e.Source], e.Target)
		nodeSet[e.Source] = struct{}{}
		nodeSet[e.Target] = struct{}{}
	}

	color := make(map[string]int) // 0=white, 1=gray, 2=black
	parent := make(map[string]string)
	var cycles [][]string
	seen := make(map[string]bool)

	var dfs func(node string)
	dfs = func(node string) {
		color[node] = 1 // gray
		for _, next := range adj[node] {
			switch color[next] {
			case 1: // gray — cycle found
				cycles = appendCycle(cycles, seen, parent, node, next)
			case 0: // white — recurse
				parent[next] = node
				dfs(next)
			}
		}
		color[node] = 2 // black
	}

	for _, node := range sortedKeys(nodeSet) {
		if color[node] == 0 {
			dfs(node)
		}
	}
	return cycles
}

func appendCycle(cycles [][]string, seen map[string]bool, parent map[string]string, node, next string) [][]string {
	var cyclePath []string
	cyclePath = append(cyclePath, next)
	cur := node
	for cur != next {
		cyclePath = append(cyclePath, cur)
		cur = parent[cur]
	}
	cyclePath = append(cyclePath, next)
	for i, j := 0, len(cyclePath)-1; i < j; i, j = i+1, j-1 {
		cyclePath[i], cyclePath[j] = cyclePath[j], cyclePath[i]
	}
	key := strings.Join(cyclePath, "|")
	if !seen[key] {
		seen[key] = true
		cycles = append(cycles, cyclePath)
	}
	return cycles
}

// computeTransitiveClosure computes indirect edges for a given edge type.
func computeTransitiveClosure(allEdges []Edge, edgeName string) []Edge {
	directSet := make(map[string]struct{})
	nodeSet := make(map[string]struct{})
	reachable := make(map[string]map[string]bool)

	for _, e := range allEdges {
		if e.EdgeName != edgeName {
			continue
		}
		if reachable[e.Source] == nil {
			reachable[e.Source] = make(map[string]bool)
		}
		reachable[e.Source][e.Target] = true
		directSet[e.Source+"|"+e.Target] = struct{}{}
		nodeSet[e.Source] = struct{}{}
		nodeSet[e.Target] = struct{}{}
	}

	floydWarshall(reachable, sortedKeys(nodeSet))
	return collectTransitiveEdges(reachable, directSet, edgeName)
}

//nolint:gocognit // Floyd-Warshall is inherently triple-nested
func floydWarshall(reachable map[string]map[string]bool, nodes []string) {
	// Ensure all nodes have entries
	for _, n := range nodes {
		if reachable[n] == nil {
			reachable[n] = make(map[string]bool)
		}
	}
	for _, k := range nodes {
		for _, i := range nodes {
			if !reachable[i][k] {
				continue
			}
			for _, j := range nodes {
				if reachable[k][j] {
					reachable[i][j] = true
				}
			}
		}
	}
}

func collectTransitiveEdges(reachable map[string]map[string]bool, directSet map[string]struct{}, edgeName string) []Edge {
	var transitive []Edge
	for src, targets := range reachable {
		for tgt, ok := range targets {
			if !ok {
				continue
			}
			if _, isDirect := directSet[src+"|"+tgt]; !isDirect {
				transitive = append(transitive, Edge{Source: src, Target: tgt, EdgeName: edgeName})
			}
		}
	}
	sort.Slice(transitive, func(i, j int) bool {
		if transitive[i].Source != transitive[j].Source {
			return transitive[i].Source < transitive[j].Source
		}
		return transitive[i].Target < transitive[j].Target
	})
	return transitive
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// matchGlob handles ** glob patterns.
func matchGlob(pattern, filePath string) bool {
	if !strings.Contains(pattern, "**") {
		matched, _ := filepath.Match(pattern, filePath)
		return matched
	}

	parts := strings.SplitN(pattern, "**", 2)
	prefix := parts[0]
	suffix := ""
	if len(parts) > 1 {
		suffix = parts[1]
	}

	if prefix != "" && !strings.HasPrefix(filePath, prefix) {
		return false
	}
	if suffix == "" || suffix == "/" {
		return true
	}

	suffix = strings.TrimPrefix(suffix, "/")
	remaining := strings.TrimPrefix(filePath, prefix)
	segments := strings.Split(remaining, "/")
	for i := range segments {
		tail := strings.Join(segments[i:], "/")
		if matched, _ := filepath.Match(suffix, tail); matched {
			return true
		}
		if matched, _ := filepath.Match(suffix, segments[i]); matched {
			return true
		}
	}
	return false
}
