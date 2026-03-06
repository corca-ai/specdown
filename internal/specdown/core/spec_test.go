package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverFindsSpecDocumentsFromIncludePatterns(t *testing.T) {
	root := t.TempDir()

	specPath := filepath.Join(root, "specs", "pocket-board.spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(specPath, []byte("# Pocket Board\n\nPlain prose only.\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "specs", "notes.md"), []byte("# Ignore me\n"), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}

	docs, err := Discover(root, []string{"specs/**/*.spec.md"})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(docs))
	}

	if docs[0].Title != "Pocket Board" {
		t.Fatalf("unexpected title: %q", docs[0].Title)
	}
	if docs[0].RelativeTo != "specs/pocket-board.spec.md" {
		t.Fatalf("unexpected relative path: %q", docs[0].RelativeTo)
	}
}

func TestDiscoverReturnsNoSpecsWhenPatternsDoNotMatch(t *testing.T) {
	root := t.TempDir()

	docs, err := Discover(root, []string{"specs/**/*.spec.md"})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("expected no docs, got %d", len(docs))
	}
}
