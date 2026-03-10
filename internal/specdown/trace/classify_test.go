package trace

import "testing"

func TestClassifyEmpty(t *testing.T) {
	c := Classify(Graph{})
	if c.Class != ClassTree {
		t.Fatalf("expected tree, got %s", c.Class)
	}
}

func TestClassifyLinearChain(t *testing.T) {
	g := Graph{
		Documents: []TypedDocument{
			{Path: "a.md"}, {Path: "b.md"}, {Path: "c.md"},
		},
		DirectEdges: []Edge{
			{Source: "a.md", Target: "b.md"},
			{Source: "b.md", Target: "c.md"},
		},
	}
	c := Classify(g)
	if c.Class != ClassTree {
		t.Fatalf("expected tree, got %s", c.Class)
	}
}

func TestClassifyTree(t *testing.T) {
	g := Graph{
		Documents: []TypedDocument{
			{Path: "root.md"}, {Path: "a.md"}, {Path: "b.md"},
			{Path: "a1.md"}, {Path: "a2.md"},
		},
		DirectEdges: []Edge{
			{Source: "root.md", Target: "a.md"},
			{Source: "root.md", Target: "b.md"},
			{Source: "a.md", Target: "a1.md"},
			{Source: "a.md", Target: "a2.md"},
		},
	}
	c := Classify(g)
	if c.Class != ClassTree {
		t.Fatalf("expected tree, got %s", c.Class)
	}
}

func TestClassifyForest(t *testing.T) {
	g := Graph{
		Documents: []TypedDocument{
			{Path: "a.md"}, {Path: "a1.md"}, {Path: "b.md"}, {Path: "b1.md"},
		},
		DirectEdges: []Edge{
			{Source: "a.md", Target: "a1.md"},
			{Source: "b.md", Target: "b1.md"},
		},
	}
	c := Classify(g)
	if c.Class != ClassTree {
		t.Fatalf("expected tree (forest), got %s", c.Class)
	}
}

func TestClassifyDAG(t *testing.T) {
	// Diamond: A → B, A → C, B → D, C → D (D has 2 parents)
	g := Graph{
		Documents: []TypedDocument{
			{Path: "a.md"}, {Path: "b.md"}, {Path: "c.md"}, {Path: "d.md"},
		},
		DirectEdges: []Edge{
			{Source: "a.md", Target: "b.md"},
			{Source: "a.md", Target: "c.md"},
			{Source: "b.md", Target: "d.md"},
			{Source: "c.md", Target: "d.md"},
		},
	}
	c := Classify(g)
	if c.Class != ClassDAG {
		t.Fatalf("expected dag, got %s", c.Class)
	}
}

func TestClassifyCyclic(t *testing.T) {
	g := Graph{
		Documents: []TypedDocument{
			{Path: "a.md"}, {Path: "b.md"},
		},
		DirectEdges: []Edge{
			{Source: "a.md", Target: "b.md"},
			{Source: "b.md", Target: "a.md"},
		},
	}
	c := Classify(g)
	if c.Class != ClassCyclic {
		t.Fatalf("expected cyclic, got %s", c.Class)
	}
}
