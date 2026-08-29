package bible

import (
	"errors"
	"testing"
)

func TestParseVerseSpecSingle(t *testing.T) {
	got, err := ParseVerseSpec("4")
	if err != nil {
		t.Fatalf("ParseVerseSpec: %v", err)
	}
	if got.From != 4 || got.To != 4 {
		t.Errorf("got %+v, want From=To=4", got)
	}
}

func TestParseVerseSpecRange(t *testing.T) {
	got, err := ParseVerseSpec("4-6")
	if err != nil {
		t.Fatalf("ParseVerseSpec: %v", err)
	}
	if got.From != 4 || got.To != 6 {
		t.Errorf("got %+v, want 4-6", got)
	}
}

func TestParseVerseSpecRejected(t *testing.T) {
	for _, in := range []string{"", "abc", "4-", "-6", "6-4", "0", "1-0", "1-2-3", "4–6"} {
		if _, err := ParseVerseSpec(in); !errors.Is(err, ErrInvalidRef) {
			t.Errorf("ParseVerseSpec(%q) err=%v, want ErrInvalidRef", in, err)
		}
	}
}

func TestFilterVersesInclusiveRange(t *testing.T) {
	in := []Verse{
		{Number: 3, Content: "c", Type: "content"},
		{Number: 4, Content: "d", Type: "content"},
		{Number: 4, Content: "heading", Type: "title"},
		{Number: 5, Content: "e", Type: "content"},
		{Number: 6, Content: "f", Type: "content"},
	}
	got := FilterVerses(in, VerseSpec{From: 4, To: 5})
	if len(got) != 3 || got[0].Content != "d" || got[1].Type != "title" || got[2].Content != "e" {
		t.Errorf("got %+v", got)
	}
}

func TestParsePassage(t *testing.T) {
	got, err := ParsePassage("3john 1:4-6")
	if err != nil {
		t.Fatalf("ParsePassage: %v", err)
	}
	if got.BookID != "3john" || got.Chapter != 1 || got.Verses.From != 4 || got.Verses.To != 6 {
		t.Errorf("got %+v", got)
	}
}

func TestParsePassageSpacedBook(t *testing.T) {
	got, err := ParsePassage("3 John 1:4")
	if err != nil {
		t.Fatalf("ParsePassage: %v", err)
	}
	if got.BookID != "3john" || got.Chapter != 1 || got.Verses.From != 4 || got.Verses.To != 4 {
		t.Errorf("got %+v", got)
	}
}

func TestParsePassageIndonesian(t *testing.T) {
	got, err := ParsePassage("Yohanes 3:16")
	if err != nil {
		t.Fatalf("ParsePassage: %v", err)
	}
	if got.BookID != "john" || got.Chapter != 3 || got.Verses.From != 16 {
		t.Errorf("got %+v", got)
	}
}

func TestParsePassageRejected(t *testing.T) {
	if _, err := ParsePassage("3john 1"); !errors.Is(err, ErrInvalidRef) {
		t.Errorf("missing colon: %v", err)
	}
	if _, err := ParsePassage("nope 1:1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown book: %v", err)
	}
}

