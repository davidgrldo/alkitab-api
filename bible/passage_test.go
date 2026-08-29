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

func TestParseVerseListComma(t *testing.T) {
	got, err := ParseVerseList("16,17")
	if err != nil {
		t.Fatalf("ParseVerseList: %v", err)
	}
	if len(got) != 2 || got[0].From != 16 || got[0].To != 16 || got[1].From != 17 || got[1].To != 17 {
		t.Errorf("got %+v", got)
	}
}

func TestParseVerseListMixed(t *testing.T) {
	got, err := ParseVerseList("16-18,20")
	if err != nil {
		t.Fatalf("ParseVerseList: %v", err)
	}
	if len(got) != 2 || got[0].From != 16 || got[0].To != 18 || got[1].From != 20 {
		t.Errorf("got %+v", got)
	}
}

func TestFilterVersesCommaList(t *testing.T) {
	in := []Verse{
		{Number: 16, Content: "a", Type: "content"},
		{Number: 17, Content: "b", Type: "content"},
		{Number: 18, Content: "c", Type: "content"},
		{Number: 19, Content: "d", Type: "content"},
		{Number: 20, Content: "e", Type: "content"},
	}
	specs, err := ParseVerseList("16,18-20")
	if err != nil {
		t.Fatal(err)
	}
	got := FilterVerses(in, specs...)
	if len(got) != 4 || got[1].Number != 18 || got[3].Number != 20 {
		t.Errorf("got %+v", got)
	}
}

func TestParsePassageCommaVerses(t *testing.T) {
	got, err := ParsePassage("John 3:16,17")
	if err != nil {
		t.Fatalf("ParsePassage: %v", err)
	}
	if got.BookID != "john" || len(got.Spans) != 1 || got.Spans[0].Chapter != 3 {
		t.Fatalf("got %+v", got)
	}
	if len(got.Spans[0].Verses) != 2 || got.Spans[0].Verses[1].From != 17 {
		t.Errorf("verses %+v", got.Spans[0].Verses)
	}
}

func TestParsePassageCrossChapter(t *testing.T) {
	got, err := ParsePassage("Matt 5:1-7:29")
	if err != nil {
		t.Fatalf("ParsePassage: %v", err)
	}
	if got.BookID != "matt" || len(got.Spans) != 3 {
		t.Fatalf("spans %+v", got)
	}
	if got.Spans[0].Chapter != 5 || got.Spans[0].Verses[0].From != 1 || got.Spans[0].Verses[0].To != MaxVerse {
		t.Errorf("start %+v", got.Spans[0])
	}
	if got.Spans[1].Chapter != 6 || len(got.Spans[1].Verses) != 0 {
		t.Errorf("middle %+v", got.Spans[1])
	}
	if got.Spans[2].Chapter != 7 || got.Spans[2].Verses[0].From != 1 || got.Spans[2].Verses[0].To != 29 {
		t.Errorf("end %+v", got.Spans[2])
	}
}

func TestParsePassageCrossChapterEnDash(t *testing.T) {
	got, err := ParsePassage("Matthew 5:1–7:29")
	if err != nil {
		t.Fatalf("en-dash: %v", err)
	}
	if len(got.Spans) != 3 || got.Spans[2].Chapter != 7 {
		t.Errorf("got %+v", got)
	}
}

func TestParsePassageCrossChapterRejected(t *testing.T) {
	if _, err := ParsePassage("Matt 7:1-5:2"); !errors.Is(err, ErrInvalidRef) {
		t.Errorf("reversed chapters: %v", err)
	}
	if _, err := ParsePassage("3john 1:1-2:1"); !errors.Is(err, ErrInvalidRef) {
		t.Errorf("beyond book chapters: %v", err)
	}
}
