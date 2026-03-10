package trace

import "testing"

func TestClassifyEmpty(t *testing.T) {
	g := Graph{}
	c := Classify(g)
	if c.Class != ClassGeneral {
		t.Fatalf("expected general, got %s", c.Class)
	}
	if c.Layout != LayoutMatrix {
		t.Fatalf("expected matrix layout, got %s", c.Layout)
	}
}

func TestClassifyLinearChain(t *testing.T) {
	g := Graph{
		Documents: []TypedDocument{
			{Path: "a.md", Type: "spec"},
			{Path: "b.md", Type: "spec"},
			{Path: "c.md", Type: "spec"},
		},
		DirectEdges: []Edge{
			{Source: "a.md", Target: "b.md", EdgeName: "depends"},
			{Source: "b.md", Target: "c.md", EdgeName: "depends"},
		},
	}
	c := Classify(g)
	if c.Class != ClassLinearChain {
		t.Fatalf("expected linear-chain, got %s", c.Class)
	}
	if c.Layout != LayoutLinear {
		t.Fatalf("expected linear layout, got %s", c.Layout)
	}
}

func TestClassifyFlatStar(t *testing.T) {
	g := Graph{
		Documents: []TypedDocument{
			{Path: "root.md", Type: "theme"},
			{Path: "a.md", Type: "epic"},
			{Path: "b.md", Type: "epic"},
			{Path: "c.md", Type: "epic"},
		},
		DirectEdges: []Edge{
			{Source: "root.md", Target: "a.md", EdgeName: "owns"},
			{Source: "root.md", Target: "b.md", EdgeName: "owns"},
			{Source: "root.md", Target: "c.md", EdgeName: "owns"},
		},
	}
	c := Classify(g)
	if c.Class != ClassFlatStar {
		t.Fatalf("expected flat-star, got %s", c.Class)
	}
	if c.Layout != LayoutRadial {
		t.Fatalf("expected radial layout, got %s", c.Layout)
	}
}

func TestClassifyTree(t *testing.T) {
	g := Graph{
		Documents: []TypedDocument{
			{Path: "root.md", Type: "theme"},
			{Path: "a.md", Type: "epic"},
			{Path: "b.md", Type: "epic"},
			{Path: "a1.md", Type: "story"},
			{Path: "a2.md", Type: "story"},
		},
		DirectEdges: []Edge{
			{Source: "root.md", Target: "a.md", EdgeName: "owns"},
			{Source: "root.md", Target: "b.md", EdgeName: "owns"},
			{Source: "a.md", Target: "a1.md", EdgeName: "owns"},
			{Source: "a.md", Target: "a2.md", EdgeName: "owns"},
		},
	}
	c := Classify(g)
	if c.Class != ClassTree {
		t.Fatalf("expected tree, got %s", c.Class)
	}
	if c.Layout != LayoutDendrogram {
		t.Fatalf("expected dendrogram layout, got %s", c.Layout)
	}
}

func TestClassifyForest(t *testing.T) {
	g := Graph{
		Documents: []TypedDocument{
			{Path: "a.md", Type: "spec"},
			{Path: "a1.md", Type: "spec"},
			{Path: "b.md", Type: "spec"},
			{Path: "b1.md", Type: "spec"},
		},
		DirectEdges: []Edge{
			{Source: "a.md", Target: "a1.md", EdgeName: "owns"},
			{Source: "b.md", Target: "b1.md", EdgeName: "owns"},
		},
	}
	c := Classify(g)
	if c.Class != ClassForest {
		t.Fatalf("expected forest, got %s", c.Class)
	}
	if c.Layout != LayoutDendrogram {
		t.Fatalf("expected dendrogram layout, got %s", c.Layout)
	}
}

func TestClassifyLayeredDAG(t *testing.T) {
	g := Graph{
		Documents: []TypedDocument{
			{Path: "theme.md", Type: "theme"},
			{Path: "epic1.md", Type: "epic"},
			{Path: "epic2.md", Type: "epic"},
			{Path: "story1.md", Type: "story"},
			{Path: "story2.md", Type: "story"},
		},
		DirectEdges: []Edge{
			{Source: "theme.md", Target: "epic1.md", EdgeName: "owns"},
			{Source: "theme.md", Target: "epic2.md", EdgeName: "owns"},
			{Source: "epic1.md", Target: "story1.md", EdgeName: "covers"},
			{Source: "epic2.md", Target: "story2.md", EdgeName: "covers"},
			// story2 also covered by epic1 — multiple parents, but layered by type
			{Source: "epic1.md", Target: "story2.md", EdgeName: "covers"},
		},
	}
	c := Classify(g)
	if c.Class != ClassLayeredDAG {
		t.Fatalf("expected layered-dag, got %s", c.Class)
	}
	if c.Layout != LayoutGrid {
		t.Fatalf("expected grid layout, got %s", c.Layout)
	}
	if len(c.Layers) != 3 {
		t.Fatalf("expected 3 layers, got %d: %v", len(c.Layers), c.Layers)
	}
	// Layers should be topologically ordered: theme before epic before story
	if c.Layers[0] != "theme" || c.Layers[1] != "epic" || c.Layers[2] != "story" {
		t.Fatalf("unexpected layer order: %v", c.Layers)
	}
}

func TestClassifyDiamondDAG(t *testing.T) {
	// Diamond: A → B, A → C, B → D, C → D (D has 2 parents, same type)
	g := Graph{
		Documents: []TypedDocument{
			{Path: "a.md", Type: "spec"},
			{Path: "b.md", Type: "spec"},
			{Path: "c.md", Type: "spec"},
			{Path: "d.md", Type: "spec"},
		},
		DirectEdges: []Edge{
			{Source: "a.md", Target: "b.md", EdgeName: "depends"},
			{Source: "a.md", Target: "c.md", EdgeName: "depends"},
			{Source: "b.md", Target: "d.md", EdgeName: "depends"},
			{Source: "c.md", Target: "d.md", EdgeName: "depends"},
		},
	}
	c := Classify(g)
	if c.Class != ClassDiamondDAG {
		t.Fatalf("expected diamond-dag, got %s", c.Class)
	}
	if c.Layout != LayoutSugiyama {
		t.Fatalf("expected sugiyama layout, got %s", c.Layout)
	}
}

func TestClassifyCyclic(t *testing.T) {
	g := Graph{
		Documents: []TypedDocument{
			{Path: "a.md", Type: "spec"},
			{Path: "b.md", Type: "spec"},
		},
		DirectEdges: []Edge{
			{Source: "a.md", Target: "b.md", EdgeName: "depends"},
			{Source: "b.md", Target: "a.md", EdgeName: "depends"},
		},
	}
	c := Classify(g)
	if c.Class != ClassGeneral {
		t.Fatalf("expected general, got %s", c.Class)
	}
	if c.Layout != LayoutMatrix {
		t.Fatalf("expected matrix layout, got %s", c.Layout)
	}
}

func TestClassifySingleEdge(t *testing.T) {
	g := Graph{
		Documents: []TypedDocument{
			{Path: "a.md", Type: "spec"},
			{Path: "b.md", Type: "spec"},
		},
		DirectEdges: []Edge{
			{Source: "a.md", Target: "b.md", EdgeName: "depends"},
		},
	}
	c := Classify(g)
	if c.Class != ClassLinearChain {
		t.Fatalf("expected linear-chain for single edge, got %s", c.Class)
	}
}

func TestClassifyDisconnectedChains(t *testing.T) {
	// Two separate chains: a→b and c→d
	g := Graph{
		Documents: []TypedDocument{
			{Path: "a.md", Type: "spec"},
			{Path: "b.md", Type: "spec"},
			{Path: "c.md", Type: "spec"},
			{Path: "d.md", Type: "spec"},
		},
		DirectEdges: []Edge{
			{Source: "a.md", Target: "b.md", EdgeName: "depends"},
			{Source: "c.md", Target: "d.md", EdgeName: "depends"},
		},
	}
	c := Classify(g)
	if c.Class != ClassForest {
		t.Fatalf("expected forest for disconnected chains, got %s", c.Class)
	}
}

func TestClassifyIntraTypeEdgesNotLayered(t *testing.T) {
	// Edges within same type prevent layered-dag; intra-type diamond → diamond-dag
	g := Graph{
		Documents: []TypedDocument{
			{Path: "a.md", Type: "spec"},
			{Path: "b.md", Type: "spec"},
			{Path: "c.md", Type: "spec"},
			{Path: "d.md", Type: "guide"},
		},
		DirectEdges: []Edge{
			{Source: "a.md", Target: "b.md", EdgeName: "depends"},
			{Source: "a.md", Target: "c.md", EdgeName: "depends"},
			{Source: "b.md", Target: "d.md", EdgeName: "explains"},
			{Source: "c.md", Target: "d.md", EdgeName: "explains"},
		},
	}
	c := Classify(g)
	// d has 2 parents (b,c) → not tree/forest; intra-type spec→spec → not layered
	if c.Class != ClassDiamondDAG {
		t.Fatalf("expected diamond-dag, got %s", c.Class)
	}
}
