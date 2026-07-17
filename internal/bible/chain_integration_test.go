package bible_test

import (
	"testing"

	"github.com/davidgrldo/alkitab-api/internal/bible"
	"github.com/davidgrldo/alkitab-api/internal/local"
)

// fakeScrape is a minimal bible.Source standing in for the scrape adapter.
// It serves a single TB chapter so the integration test can prove the Chain
// routes to it after local returns ErrUnsupportedVersion for "tb".
type fakeScrape struct{}

func (fakeScrape) Translations() []bible.Translation {
	return []bible.Translation{{ID: "tb", Name: "Terjemahan Baru", Language: "id"}}
}

func (fakeScrape) Books(string) ([]bible.Book, error) { return bible.CanonicalBooks(), nil }

func (fakeScrape) Chapter(version, book string, chapter int) (*bible.Chapter, error) {
	if version == "tb" && book == "3john" && chapter == 1 {
		return &bible.Chapter{
			Translation: version, Book: book, Number: chapter,
			Verses: []bible.Verse{{Number: 1, Content: "scraped TB verse", Type: "content", Order: 0}},
		}, nil
	}
	return nil, bible.ErrNotFound
}

// TestChainFallsThroughLocalToScrape wires the real local adapter (which only
// carries kjv and returns ErrUnsupportedVersion for "tb") in front of a fake
// scrape source. This is the integration proof for C1: requesting a chapter in
// a version local does not own must reach scrape instead of 404ing at Chain.
func TestChainFallsThroughLocalToScrape(t *testing.T) {
	realLocal, err := local.New("")
	if err != nil {
		t.Fatalf("local.New: %v", err)
	}
	chain := bible.NewChain(realLocal, fakeScrape{})

	c, err := chain.Chapter("tb", "3john", 1)
	if err != nil {
		t.Fatalf("chain.Chapter tb/3john/1: %v (local ErrUnsupportedVersion did not fall through)", err)
	}
	if c.Translation != "tb" || c.Verses[0].Content != "scraped TB verse" {
		t.Errorf("expected fake scrape chapter, got %+v", c)
	}

	var hasKJV, hasTB bool
	for _, tr := range chain.Translations() {
		switch tr.ID {
		case "kjv":
			hasKJV = true
		case "tb":
			hasTB = true
		}
	}
	if !hasKJV || !hasTB {
		t.Errorf("Translations() must merge kjv and tb, got %v", chain.Translations())
	}
}
