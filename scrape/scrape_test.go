package scrape

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/davidgrldo/alkitab-api/bible"
)

func TestScrapeChapter(t *testing.T) {
	// The test handler serves the fixture regardless of path, so the scrape
	// adapter's URL construction (which uses the Indonesian book name) does not
	// matter for parsing correctness.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "testdata/3john.html")
	}))
	defer srv.Close()

	s := New(srv.URL)
	c, err := s.Chapter("tb", "3john", 1)
	if err != nil {
		t.Fatalf("Chapter: %v", err)
	}
	// hidden and loading <p> are skipped; 3 items remain (1 content + 1 title + 1 content)
	if len(c.Verses) != 3 {
		t.Fatalf("want 3 items, got %d: %+v", len(c.Verses), c.Verses)
	}
	// second item is the title "Greeting" with verse == lastVerse+1 == 2
	title := c.Verses[1]
	if title.Type != "title" || title.Content != "Greeting" || title.Number != 2 {
		t.Errorf("title item wrong: %+v", title)
	}
}

func TestScrapeTranslationsAndBooks(t *testing.T) {
	s := New("https://alkitab.mobi")
	trans := s.Translations()
	if len(trans) == 0 {
		t.Fatal("want at least one static translation (tb)")
	}
	books, err := s.Books("tb")
	if err != nil {
		t.Fatalf("Books: %v", err)
	}
	if len(books) != 66 {
		t.Errorf("want 66 canonical books, got %d", len(books))
	}
}

func TestScrapeUnknownVersion(t *testing.T) {
	s := New("https://alkitab.mobi")
	if _, err := s.Books("nonsense"); err != bible.ErrUnsupportedVersion {
		t.Errorf("want ErrUnsupportedVersion, got %v", err)
	}
}
