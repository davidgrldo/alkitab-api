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
