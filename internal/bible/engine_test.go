package bible

import "testing"

func newFake() *fakeSource {
	return &fakeSource{
		trans: []Translation{{ID: "kjv", Name: "KJV", Language: "en"}},
		books: []Book{{ID: "3john", Name: "3 John", Testament: "NT", Chapters: 1}},
		chaps: map[string]*Chapter{
			"kjv:3john:1": {
				Translation: "kjv", Book: "3john", Number: 1,
				Verses: []Verse{{Number: 1, Content: "The elder", Type: "content", Order: 0}},
			},
		},
	}
}

func TestEngineChapterLookup(t *testing.T) {
	f := newFake()
	e := New(f)
	c, err := e.Chapter("kjv", "3john", 1)
	if err != nil {
		t.Fatalf("Chapter: %v", err)
	}
	if c.Verses[0].Content != "The elder" {
		t.Errorf("got %q", c.Verses[0].Content)
	}
}

func TestEngineChapterNotFound(t *testing.T) {
	e := New(newFake())
	if _, err := e.Chapter("kjv", "3john", 99); err != ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestEngineChapterCache(t *testing.T) {
	f := newFake()
	e := New(f)
	_, _ = e.Chapter("kjv", "3john", 1)
	_, _ = e.Chapter("kjv", "3john", 1)
	if f.calls != 1 {
		t.Errorf("source called %d times, want 1 (cached)", f.calls)
	}
}

type fakeCorpus struct{ all []VerseHit }

func (f *fakeCorpus) AllVerses(version string) ([]VerseHit, error) { return f.all, nil }

func TestEngineSearchUnsupported(t *testing.T) {
	e := New(newFake()) // fakeSource is not a Corpus
	if _, err := e.Search("kjv", "love"); err != ErrUnsupportedFeature {
		t.Errorf("want ErrUnsupportedFeature, got %v", err)
	}
}

func TestEngineSearch(t *testing.T) {
	f := newFake()
	all := []VerseHit{
		{Translation: "kjv", Book: "3john", Chapter: 1, Verse: Verse{Number: 1, Content: "God is love", Type: "content"}},
		{Translation: "kjv", Book: "3john", Chapter: 1, Verse: Verse{Number: 2, Content: "Walk in truth", Type: "content"}},
	}
	// attach corpus capability by composition
	src := struct {
		*fakeSource
		*fakeCorpus
	}{f, &fakeCorpus{all}}
	e := New(src)

	hits, err := e.Search("kjv", "LOVE")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 || hits[0].Verse.Content != "God is love" {
		t.Errorf("unexpected hits: %+v", hits)
	}
}
