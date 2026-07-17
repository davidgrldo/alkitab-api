package bible

import (
	"fmt"
	"testing"
	"time"
)

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

func TestEngineDailyDeterministic(t *testing.T) {
	all := []VerseHit{
		{Translation: "kjv", Book: "3john", Chapter: 1, Verse: Verse{Number: 1, Content: "a", Type: "content"}},
		{Translation: "kjv", Book: "3john", Chapter: 1, Verse: Verse{Number: 2, Content: "b", Type: "content"}},
		{Translation: "kjv", Book: "phlm", Chapter: 1, Verse: Verse{Number: 1, Content: "c", Type: "content"}},
	}
	src := struct {
		*fakeSource
		*fakeCorpus
	}{newFake(), &fakeCorpus{all}}
	e := New(src)

	d := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)
	h1, err := e.DailyVerse("kjv", d)
	if err != nil {
		t.Fatalf("DailyVerse: %v", err)
	}
	h2, _ := e.DailyVerse("kjv", d)
	if *h1 != *h2 {
		t.Errorf("same date must yield same verse: %v vs %v", h1, h2)
	}
	// different date should very likely differ over a few samples (not asserted strictly)
	_ = h1
}

func TestEngineRandomInBounds(t *testing.T) {
	all := []VerseHit{
		{Translation: "kjv", Book: "3john", Chapter: 1, Verse: Verse{Number: 1, Content: "a", Type: "content"}},
	}
	src := struct {
		*fakeSource
		*fakeCorpus
	}{newFake(), &fakeCorpus{all}}
	e := New(src)
	for i := 0; i < 50; i++ {
		h, err := e.RandomVerse("kjv")
		if err != nil {
			t.Fatalf("RandomVerse: %v", err)
		}
		if h == nil {
			t.Fatal("nil hit")
		}
	}
}

func TestEngineDailyUnsupported(t *testing.T) {
	e := New(newFake())
	if _, err := e.DailyVerse("kjv", time.Now()); err != ErrUnsupportedFeature {
		t.Errorf("want ErrUnsupportedFeature, got %v", err)
	}
}

func TestChainFallback(t *testing.T) {
	primary := newFake() // has kjv:3john:1 only
	missSrc := &fakeSource{
		trans: []Translation{{ID: "tb", Name: "TB", Language: "id"}},
		chaps: map[string]*Chapter{
			"tb:3john:1": {Translation: "tb", Book: "3john", Number: 1,
				Verses: []Verse{{Number: 1, Content: "Kepala Jemaat", Type: "content"}}},
		},
	}
	ch := NewChain(primary, missSrc)

	// hit on primary
	c, err := ch.Chapter("kjv", "3john", 1)
	if err != nil || c.Translation != "kjv" {
		t.Errorf("primary miss/err: %v %v", c, err)
	}
	// miss on primary (kjv:genesis:1), hit on secondary
	c, err = ch.Chapter("tb", "3john", 1)
	if err != nil || c.Translation != "tb" {
		t.Errorf("fallback err: %v %v", c, err)
	}
	// all miss
	_, err = ch.Chapter("x", "y", 1)
	if err != ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestChainTranslationsMerged(t *testing.T) {
	ch := NewChain(
		&fakeSource{trans: []Translation{{ID: "kjv"}}},
		&fakeSource{trans: []Translation{{ID: "tb"}}},
	)
	got := ch.Translations()
	if len(got) != 2 {
		t.Errorf("want 2 translations merged, got %d", len(got))
	}
}

// unsupportedFakeSource models a source that owns one version and rejects
// every other version with ErrUnsupportedVersion (mirrors local.Local's
// behavior for versions it does not carry). Supported versions look up
// chapters by key, returning ErrNotFound on a genuine miss.
type unsupportedFakeSource struct {
	trans       []Translation
	supportedID string
	chaps       map[string]*Chapter
}

func (f *unsupportedFakeSource) Translations() []Translation { return f.trans }

func (f *unsupportedFakeSource) Books(version string) ([]Book, error) {
	if version != f.supportedID {
		return nil, ErrUnsupportedVersion
	}
	return nil, nil
}

func (f *unsupportedFakeSource) Chapter(version, book string, chapter int) (*Chapter, error) {
	if version != f.supportedID {
		return nil, ErrUnsupportedVersion
	}
	c, ok := f.chaps[fmt.Sprintf("%s:%s:%d", version, book, chapter)]
	if !ok {
		return nil, ErrNotFound
	}
	return c, nil
}

func TestChainFallsThroughOnUnsupportedVersion(t *testing.T) {
	primary := &unsupportedFakeSource{
		trans:       []Translation{{ID: "kjv", Name: "KJV", Language: "en"}},
		supportedID: "kjv",
		chaps: map[string]*Chapter{
			"kjv:3john:1": {Translation: "kjv", Book: "3john", Number: 1,
				Verses: []Verse{{Number: 1, Content: "The elder", Type: "content"}}},
		},
	}
	secondary := &fakeSource{
		trans: []Translation{{ID: "tb", Name: "TB", Language: "id"}},
		chaps: map[string]*Chapter{
			"tb:3john:1": {Translation: "tb", Book: "3john", Number: 1,
				Verses: []Verse{{Number: 1, Content: "Kepala Jemaat", Type: "content"}}},
		},
	}
	ch := NewChain(primary, secondary)

	// Primary returns ErrUnsupportedVersion for "tb"; Chain must fall through
	// to the secondary source instead of propagating the error.
	c, err := ch.Chapter("tb", "3john", 1)
	if err != nil {
		t.Fatalf("Chapter tb/3john/1: %v (expected fallthrough to secondary)", err)
	}
	if c.Translation != "tb" || c.Verses[0].Content != "Kepala Jemaat" {
		t.Errorf("expected secondary source chapter, got %+v", c)
	}

	// Sanity: a version both sources reject still yields ErrNotFound.
	if _, err := ch.Chapter("zzz", "3john", 1); err != ErrNotFound {
		t.Errorf("want ErrNotFound for fully-unsupported version, got %v", err)
	}
}
