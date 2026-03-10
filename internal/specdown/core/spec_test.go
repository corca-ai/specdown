package core

import (
	"os"
	"path/filepath"
	"strings"
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
	// Entry is now included as the first document.
	if len(docs) != 3 {
		t.Fatalf("expected 3 docs (entry + 2 specs), got %d", len(docs))
	}
	if docs[0].Title != "My Specs" {
		t.Fatalf("expected first doc to be entry (My Specs), got %q", docs[0].Title)
	}
	if docs[1].Title != "Pocket Card" {
		t.Fatalf("expected second doc to be Pocket Card, got %q", docs[1].Title)
	}
	if docs[2].Title != "Pocket Board" {
		t.Fatalf("expected third doc to be Pocket Board, got %q", docs[2].Title)
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
	// Entry + one deduplicated spec.
	if len(docs) != 2 {
		t.Fatalf("expected 2 docs after dedup, got %d", len(docs))
	}
}

func TestDiscoverFromEntryNoH1UsesFallback(t *testing.T) {
	root := t.TempDir()
	entryPath := filepath.Join(root, "index.spec.md")
	if err := os.WriteFile(entryPath, []byte("No heading here\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	title, _, err := DiscoverFromEntry(root, "index.spec.md", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ParseDocument falls back to relativePath when no H1 is found.
	if title != "index.spec.md" {
		t.Fatalf("expected fallback title %q, got %q", "index.spec.md", title)
	}
}

func TestDiscoverFromEntryNoLinksReturnsEntryOnly(t *testing.T) {
	root := t.TempDir()
	entryPath := filepath.Join(root, "index.spec.md")
	if err := os.WriteFile(entryPath, []byte("# Title\n\nNo links.\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, docs, err := DiscoverFromEntry(root, "index.spec.md", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc (entry only), got %d", len(docs))
	}
}

func TestDiscoverFromEntryRecursiveCrawl(t *testing.T) {
	root := t.TempDir()

	specsDir := filepath.Join(root, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// entry → a → b (recursive discovery)
	if err := os.WriteFile(filepath.Join(specsDir, "index.spec.md"), []byte("# Index\n\n[A](a.spec.md)\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specsDir, "a.spec.md"), []byte("# A\n\n[B](b.md)\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specsDir, "b.md"), []byte("# B\n\nPlain markdown.\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, docs, err := DiscoverFromEntry(root, "specs/index.spec.md", nil)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(docs) != 3 {
		t.Fatalf("expected 3 docs, got %d", len(docs))
	}
	if docs[2].Title != "B" {
		t.Fatalf("expected transitive doc B, got %q", docs[2].Title)
	}
}

func TestDiscoverFromEntryBrokenLinkInsideErrors(t *testing.T) {
	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "index.spec.md"), []byte("# X\n\n[Missing](missing.md)\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, _, err := DiscoverFromEntry(root, "index.spec.md", nil)
	if err == nil {
		t.Fatal("expected error for broken link")
	}
	if !strings.Contains(err.Error(), "broken link") {
		t.Fatalf("expected broken link error, got: %v", err)
	}
}

func TestDiscoverFromEntryOutsideLinkWarns(t *testing.T) {
	root := t.TempDir()

	specsDir := filepath.Join(root, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "outside.md"), []byte("# Outside\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	entry := "# Index\n\n[Outside](../outside.md)\n"
	if err := os.WriteFile(filepath.Join(specsDir, "index.spec.md"), []byte(entry), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, docs, err := DiscoverFromEntry(root, "specs/index.spec.md", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should warn but not error. Entry only.
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	if len(docs[0].Warnings) == 0 {
		t.Fatal("expected warning about outside link")
	}
}

func TestDiscoverFromEntryCycleHandling(t *testing.T) {
	root := t.TempDir()

	// a → b → a (cycle)
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("# A\n\n[B](b.md)\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.md"), []byte("# B\n\n[A](a.md)\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, docs, err := DiscoverFromEntry(root, "a.md", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 docs (cycle resolved), got %d", len(docs))
	}
}
