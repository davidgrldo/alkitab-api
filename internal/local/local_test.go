package local

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidgrldo/alkitab-api/internal/bible"
)

func TestLocalLoadsEmbedded(t *testing.T) {
	l, err := New("")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	trans := l.Translations()
	if len(trans) != 1 || trans[0].ID != "kjv" {
		t.Fatalf("translations = %+v", trans)
	}
}

func TestLocalBooks(t *testing.T) {
	l, _ := New("")
	books, err := l.Books("kjv")
	if err != nil {
		t.Fatalf("Books: %v", err)
	}
	if len(books) != 2 {
		t.Fatalf("want 2 books in sample, got %d", len(books))
	}
	if _, err := l.Books("nope"); !errors.Is(err, bible.ErrUnsupportedVersion) {
		t.Errorf("unknown version want ErrUnsupportedVersion, got %v", err)
	}
}

func TestLocalChapter(t *testing.T) {
	l, _ := New("")
	c, err := l.Chapter("kjv", "3john", 1)
	if err != nil {
		t.Fatalf("Chapter: %v", err)
	}
	if len(c.Verses) != 14 {
		t.Errorf("3john has %d verses, want 14", len(c.Verses))
	}
	if _, err := l.Chapter("kjv", "3john", 9); !errors.Is(err, bible.ErrNotFound) {
		t.Errorf("missing chapter want ErrNotFound, got %v", err)
	}
}

func TestLocalRuntimeDirOverrides(t *testing.T) {
	dir := t.TempDir()
	custom := `{"translation":{"id":"kjv","name":"Custom","language":"en"},"books":[]}`
	if err := os.WriteFile(filepath.Join(dir, "kjv.json"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	trans := l.Translations()
	if trans[0].Name != "Custom" {
		t.Errorf("runtime override failed: %+v", trans)
	}
}

func TestLocalAllVerses(t *testing.T) {
	l, _ := New("")
	all, err := l.AllVerses("kjv")
	if err != nil {
		t.Fatalf("AllVerses: %v", err)
	}
	// 3 John (14) + Philemon (25) = 39
	if len(all) != 39 {
		t.Errorf("want 39 verses in sample corpus, got %d", len(all))
	}
	for _, h := range all {
		if h.Translation != "kjv" {
			t.Errorf("hit translation = %q", h.Translation)
		}
	}
}
