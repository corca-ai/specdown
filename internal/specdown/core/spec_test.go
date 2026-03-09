package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverFromEntryFindsSpecDocumentsInLinkOrder(t *testing.T) {
	root := t.TempDir()

	specsDir := filepath.Join(root, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specsDir, "pocket-board.spec.md"), []byte("# Pocket Board\n\nPlain prose only.\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specsDir, "pocket-card.spec.md"), []byte("# Pocket Card\n\nCard spec.\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	entry := "# My Specs\n\n- [Card](pocket-card.spec.md)\n- [Board](pocket-board.spec.md)\n"
	if err := os.WriteFile(filepath.Join(specsDir, "index.spec.md"), []byte(entry), 0o644); err != nil {
		t.Fatalf("write entry: %v", err)
	}

	title, docs, err := DiscoverFromEntry(root, "specs/index.spec.md", nil)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if title != "My Specs" {
		t.Fatalf("unexpected title %q", title)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(docs))
	}
	if docs[0].Title != "Pocket Card" {
		t.Fatalf("expected first doc to be Pocket Card, got %q", docs[0].Title)
	}
	if docs[1].Title != "Pocket Board" {
		t.Fatalf("expected second doc to be Pocket Board, got %q", docs[1].Title)
	}
}

func TestDiscoverFromEntryDeduplicatesLinks(t *testing.T) {
	root := t.TempDir()

	specsDir := filepath.Join(root, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specsDir, "a.spec.md"), []byte("# A\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	entry := "# Title\n\n- [A](a.spec.md)\n- [A again](a.spec.md)\n"
	if err := os.WriteFile(filepath.Join(specsDir, "index.spec.md"), []byte(entry), 0o644); err != nil {
		t.Fatalf("write entry: %v", err)
	}

	_, docs, err := DiscoverFromEntry(root, "specs/index.spec.md", nil)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc after dedup, got %d", len(docs))
	}
}

func TestDiscoverFromEntryErrorsOnMissingH1(t *testing.T) {
	root := t.TempDir()
	entryPath := filepath.Join(root, "index.spec.md")
	if err := os.WriteFile(entryPath, []byte("No heading here\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, _, err := DiscoverFromEntry(root, "index.spec.md", nil)
	if err == nil {
		t.Fatal("expected error for missing H1")
	}
}

func TestDiscoverFromEntryErrorsOnNoLinks(t *testing.T) {
	root := t.TempDir()
	entryPath := filepath.Join(root, "index.spec.md")
	if err := os.WriteFile(entryPath, []byte("# Title\n\nNo links.\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, _, err := DiscoverFromEntry(root, "index.spec.md", nil)
	if err == nil {
		t.Fatal("expected error for no links")
	}
}
