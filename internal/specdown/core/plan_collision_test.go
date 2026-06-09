package core

import (
	"strings"
	"testing"
)

func TestDetectExtensionCollisions(t *testing.T) {
	docs := []Document{
		{RelativeTo: "specs/index.md"},
		{RelativeTo: "specs/feature.md"},
		{RelativeTo: "specs/feature.spec.md"}, // collides with feature.md
		{RelativeTo: "specs/other.spec.md"},
	}

	warnings := detectExtensionCollisions(docs)
	if len(warnings) != 1 {
		t.Fatalf("got %d warnings, want 1: %v", len(warnings), warnings)
	}
	w := warnings[0]
	if !strings.Contains(w, "specs/feature.md") || !strings.Contains(w, "specs/feature.spec.md") {
		t.Fatalf("warning should name both colliding files: %q", w)
	}
	if !strings.Contains(w, "feature.html") || !strings.Contains(w, "legacy") {
		t.Fatalf("warning should mention the report page and legacy guidance: %q", w)
	}
}

func TestDetectExtensionCollisionsNoneWhenDistinct(t *testing.T) {
	docs := []Document{
		{RelativeTo: "specs/index.md"},
		{RelativeTo: "specs/a.md"},
		{RelativeTo: "specs/b.spec.md"},
	}
	if warnings := detectExtensionCollisions(docs); len(warnings) != 0 {
		t.Fatalf("expected no collisions, got %v", warnings)
	}
}
