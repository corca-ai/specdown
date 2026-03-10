package trace

// GraphClass identifies the structural class of a trace graph.
type GraphClass string

const (
	ClassTree   GraphClass = "tree"   // acyclic, each node has ≤1 parent (includes linear chains, stars, forests)
	ClassDAG    GraphClass = "dag"    // acyclic, some nodes have multiple parents
	ClassCyclic GraphClass = "cyclic" // has cycles
)

// Classification holds the detected graph class.
type Classification struct {
	Class GraphClass `json:"class"`
}

// Classify analyzes a Graph and returns its structural classification.
func Classify(g Graph) Classification {
	if len(g.DirectEdges) == 0 {
		return Classification{Class: ClassTree}
	}

	if !isAcyclic(g.DirectEdges) {
		return Classification{Class: ClassCyclic}
	}

	inDeg := make(map[string]int)
	for _, e := range g.DirectEdges {
		inDeg[e.Target]++
	}
	for _, v := range inDeg {
		if v > 1 {
			return Classification{Class: ClassDAG}
		}
	}

	return Classification{Class: ClassTree}
}

// isAcyclic returns true if the edges form a DAG (no cycles).
func isAcyclic(edges []Edge) bool {
	return len(detectCycles(edges)) == 0
}
