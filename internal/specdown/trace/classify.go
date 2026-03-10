package trace

// GraphClass identifies the structural class of a trace graph.
type GraphClass string

const (
	ClassLinearChain GraphClass = "linear-chain" // acyclic, each node has ≤1 in and ≤1 out
	ClassFlatStar    GraphClass = "flat-star"    // acyclic, single root fans to all other nodes
	ClassTree        GraphClass = "tree"         // acyclic, each node has ≤1 parent, connected
	ClassForest      GraphClass = "forest"       // acyclic, each node has ≤1 parent, disconnected
	ClassLayeredDAG  GraphClass = "layered-dag"  // acyclic, types form layers (edges only go from one type to another)
	ClassDiamondDAG  GraphClass = "diamond-dag"  // acyclic, nodes can have multiple parents
	ClassGeneral     GraphClass = "general"      // cyclic or unconstrained
)

// Layout is the recommended visualization strategy for a graph class.
type Layout string

const (
	LayoutLinear     Layout = "linear"
	LayoutRadial     Layout = "radial"
	LayoutDendrogram Layout = "dendrogram"
	LayoutGrid       Layout = "grid"
	LayoutSugiyama   Layout = "sugiyama"
	LayoutMatrix     Layout = "matrix"
)

// Classification holds the detected graph class and recommended layout.
type Classification struct {
	Class  GraphClass `json:"class"`
	Layout Layout     `json:"layout"`
	Layers []string   `json:"layers,omitempty"` // ordered type layers for layered-dag / grid layout
}

// Classify analyzes a Graph and returns its structural classification.
func Classify(g Graph) Classification {
	if len(g.DirectEdges) == 0 {
		return Classification{Class: ClassGeneral, Layout: LayoutMatrix}
	}

	acyclic := isAcyclic(g.DirectEdges)

	if !acyclic {
		return Classification{Class: ClassGeneral, Layout: LayoutMatrix}
	}

	inDeg, outDeg := degrees(g.DirectEdges)
	nodeCount := countNodes(g)

	// Linear chain: every node has ≤1 in and ≤1 out, and it's connected
	if allAtMost(inDeg, 1) && allAtMost(outDeg, 1) {
		if isConnected(g) {
			return Classification{Class: ClassLinearChain, Layout: LayoutLinear}
		}
		// Disconnected chains are forests
		return Classification{Class: ClassForest, Layout: LayoutDendrogram}
	}

	// Flat star: single root (inDeg 0), all others are leaves (outDeg 0), root connects to all
	roots := nodesWithDegree(inDeg, outDeg, g, true)
	leaves := nodesWithDegree(inDeg, outDeg, g, false)
	if len(roots) == 1 && len(leaves) == nodeCount-1 {
		root := roots[0]
		if outDeg[root] == nodeCount-1 {
			return Classification{Class: ClassFlatStar, Layout: LayoutRadial}
		}
	}

	// Tree/Forest: each node has ≤1 parent (inDeg ≤ 1)
	if allAtMost(inDeg, 1) {
		if isConnected(g) {
			return Classification{Class: ClassTree, Layout: LayoutDendrogram}
		}
		return Classification{Class: ClassForest, Layout: LayoutDendrogram}
	}

	// Layered DAG: types form layers where edges only go from one type to another
	if layers := detectLayers(g); len(layers) > 1 {
		return Classification{Class: ClassLayeredDAG, Layout: LayoutGrid, Layers: layers}
	}

	// Diamond DAG: acyclic with multiple parents
	return Classification{Class: ClassDiamondDAG, Layout: LayoutSugiyama}
}

// isAcyclic returns true if the edges form a DAG (no cycles).
func isAcyclic(edges []Edge) bool {
	return len(detectCycles(edges)) == 0
}

// degrees computes in-degree and out-degree maps for all edge endpoints.
func degrees(edges []Edge) (inDeg, outDeg map[string]int) {
	inDeg = make(map[string]int)
	outDeg = make(map[string]int)
	for _, e := range edges {
		outDeg[e.Source]++
		inDeg[e.Target]++
	}
	return
}

// countNodes returns the number of unique nodes in the graph (from Documents or edges).
func countNodes(g Graph) int {
	if len(g.Documents) > 0 {
		return len(g.Documents)
	}
	nodes := make(map[string]struct{})
	for _, e := range g.DirectEdges {
		nodes[e.Source] = struct{}{}
		nodes[e.Target] = struct{}{}
	}
	return len(nodes)
}

// allAtMost returns true if all values in the map are ≤ max.
func allAtMost(m map[string]int, max int) bool {
	for _, v := range m {
		if v > max {
			return false
		}
	}
	return true
}

// nodesWithDegree returns nodes that are roots (inDeg=0, outDeg>0) or leaves (outDeg=0, inDeg>0).
func nodesWithDegree(inDeg, outDeg map[string]int, g Graph, wantRoots bool) []string {
	nodes := allNodes(g)
	var result []string
	for _, n := range nodes {
		if wantRoots {
			if inDeg[n] == 0 && outDeg[n] > 0 {
				result = append(result, n)
			}
		} else {
			if outDeg[n] == 0 && inDeg[n] > 0 {
				result = append(result, n)
			}
		}
	}
	return result
}

// allNodes returns all unique node identifiers from the graph.
func allNodes(g Graph) []string {
	seen := make(map[string]struct{})
	for _, d := range g.Documents {
		seen[d.Path] = struct{}{}
	}
	for _, e := range g.DirectEdges {
		seen[e.Source] = struct{}{}
		seen[e.Target] = struct{}{}
	}
	result := make([]string, 0, len(seen))
	for n := range seen {
		result = append(result, n)
	}
	return result
}

// isConnected returns true if the graph is weakly connected (treating edges as undirected).
func isConnected(g Graph) bool {
	nodes := allNodes(g)
	if len(nodes) <= 1 {
		return true
	}

	adj := make(map[string][]string)
	for _, e := range g.DirectEdges {
		adj[e.Source] = append(adj[e.Source], e.Target)
		adj[e.Target] = append(adj[e.Target], e.Source)
	}

	visited := make(map[string]bool)
	queue := []string{nodes[0]}
	visited[nodes[0]] = true

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range adj[cur] {
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}

	// All nodes in edges must be visited; isolated documents don't break connectedness
	// if they have no edges at all.
	edgeNodes := make(map[string]struct{})
	for _, e := range g.DirectEdges {
		edgeNodes[e.Source] = struct{}{}
		edgeNodes[e.Target] = struct{}{}
	}
	for n := range edgeNodes {
		if !visited[n] {
			return false
		}
	}
	return true
}

// detectLayers attempts to assign types to layers via topological ordering.
// Returns the ordered layer names if all edges go strictly from earlier layers to later ones.
// Returns nil if the graph is not cleanly layered by type.
func detectLayers(g Graph) []string {
	// Build type-level DAG: which types have edges to which other types
	typeEdges := make(map[string]map[string]struct{})
	typeSet := make(map[string]struct{})
	docType := make(map[string]string)

	for _, d := range g.Documents {
		if d.Type != "" {
			typeSet[d.Type] = struct{}{}
			docType[d.Path] = d.Type
		}
	}

	// Need at least 2 types for layering
	if len(typeSet) < 2 {
		return nil
	}

	for _, e := range g.DirectEdges {
		srcType := docType[e.Source]
		tgtType := docType[e.Target]
		if srcType == "" || tgtType == "" {
			continue
		}
		if srcType == tgtType {
			// Intra-type edges break layering
			return nil
		}
		if typeEdges[srcType] == nil {
			typeEdges[srcType] = make(map[string]struct{})
		}
		typeEdges[srcType][tgtType] = struct{}{}
	}

	// Topological sort on types
	return topoSortTypes(typeSet, typeEdges)
}

// topoSortTypes performs a topological sort on the type-level DAG.
// Returns nil if cycles exist.
func topoSortTypes(typeSet map[string]struct{}, edges map[string]map[string]struct{}) []string {
	inDeg := make(map[string]int)
	for t := range typeSet {
		inDeg[t] = 0
	}
	for _, targets := range edges {
		for t := range targets {
			inDeg[t]++
		}
	}

	var queue []string
	for t := range typeSet {
		if inDeg[t] == 0 {
			queue = append(queue, t)
		}
	}

	var order []string
	for len(queue) > 0 {
		// Sort queue for deterministic output
		sortStrings(queue)
		cur := queue[0]
		queue = queue[1:]
		order = append(order, cur)
		for t := range edges[cur] {
			inDeg[t]--
			if inDeg[t] == 0 {
				queue = append(queue, t)
			}
		}
	}

	if len(order) != len(typeSet) {
		return nil // cycle in type-level DAG
	}
	return order
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
