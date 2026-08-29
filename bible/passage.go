package bible

import (
	"strconv"
	"strings"
)

// MaxVerse is an inclusive upper bound meaning “through the end of the chapter”
// when the real last verse is not yet known (cross-chapter start span).
const MaxVerse = 1 << 30

// VerseSpec is an inclusive verse number range within one chapter.
type VerseSpec struct {
	From, To int
}

// ChapterSpan is one chapter inside a parsed passage, with optional verse filters.
// An empty Verses slice means the whole chapter.
type ChapterSpan struct {
	Chapter int
	Verses  []VerseSpec
}

// PassageRef is a book-local selection: one chapter, a comma list, or a chapter span.
type PassageRef struct {
	BookID  string
	Chapter int
	Verses  VerseSpec
	Spans   []ChapterSpan
}

func (s VerseSpec) contains(n int) bool {
	return n >= s.From && n <= s.To
}

func specsContain(specs []VerseSpec, n int) bool {
	if len(specs) == 0 {
		return true
	}
	for _, s := range specs {
		if s.contains(n) {
			return true
		}
	}
	return false
}

func specBounds(specs []VerseSpec) (from, to int) {
	if len(specs) == 0 {
		return 0, 0
	}
	from, to = specs[0].From, specs[0].To
	for _, s := range specs[1:] {
		if s.From < from {
			from = s.From
		}
		if s.To > to {
			to = s.To
		}
	}
	if to > MaxVerse/2 {
		to = MaxVerse
	}
	return from, to
}

// ParseVerseSpec accepts "4" or "4-6" (ASCII hyphen). From and To are ≥ 1 and From ≤ To.
func ParseVerseSpec(s string) (VerseSpec, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return VerseSpec{}, ErrInvalidRef
	}
	fromS, toS, ok := strings.Cut(s, "-")
	if !ok {
		n, err := parseVerseNum(s)
		if err != nil {
			return VerseSpec{}, err
		}
		return VerseSpec{From: n, To: n}, nil
	}
	if strings.Contains(toS, "-") {
		return VerseSpec{}, ErrInvalidRef
	}
	from, err := parseVerseNum(fromS)
	if err != nil {
		return VerseSpec{}, err
	}
	to, err := parseVerseNum(toS)
	if err != nil {
		return VerseSpec{}, err
	}
	if from > to {
		return VerseSpec{}, ErrInvalidRef
	}
	return VerseSpec{From: from, To: to}, nil
}

// ParseVerseList accepts a single spec or a comma-separated list ("16,17", "16-18,20").
func ParseVerseList(s string) ([]VerseSpec, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, ErrInvalidRef
	}
	parts := strings.Split(s, ",")
	out := make([]VerseSpec, 0, len(parts))
	for _, p := range parts {
		spec, err := ParseVerseSpec(p)
		if err != nil {
			return nil, err
		}
		out = append(out, spec)
	}
	return out, nil
}

func parseVerseNum(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 {
		return 0, ErrInvalidRef
	}
	return n, nil
}

func normalizeDashes(s string) string {
	s = strings.ReplaceAll(s, "\u2013", "-") // en dash
	s = strings.ReplaceAll(s, "\u2014", "-") // em dash
	return s
}

func parseChapterVerse(s string) (ch, v int, err error) {
	s = strings.TrimSpace(s)
	left, right, ok := strings.Cut(s, ":")
	if !ok {
		return 0, 0, ErrInvalidRef
	}
	ch, err = parseVerseNum(left)
	if err != nil {
		return 0, 0, err
	}
	v, err = parseVerseNum(right)
	if err != nil {
		return 0, 0, err
	}
	return ch, v, nil
}

func expandSpans(bookID string, fromCh, fromV, toCh, toV int) ([]ChapterSpan, error) {
	meta, ok := lookupByID(bookID)
	if !ok {
		return nil, ErrNotFound
	}
	if fromCh < 1 || toCh < 1 || fromCh > toCh || toCh > meta.Chapters {
		return nil, ErrInvalidRef
	}
	if fromCh == toCh && fromV > toV {
		return nil, ErrInvalidRef
	}
	var spans []ChapterSpan
	for ch := fromCh; ch <= toCh; ch++ {
		switch {
		case fromCh == toCh:
			spans = append(spans, ChapterSpan{Chapter: ch, Verses: []VerseSpec{{From: fromV, To: toV}}})
		case ch == fromCh:
			spans = append(spans, ChapterSpan{Chapter: ch, Verses: []VerseSpec{{From: fromV, To: MaxVerse}}})
		case ch == toCh:
			spans = append(spans, ChapterSpan{Chapter: ch, Verses: []VerseSpec{{From: 1, To: toV}}})
		default:
			spans = append(spans, ChapterSpan{Chapter: ch})
		}
	}
	return spans, nil
}

func firstSpec(spans []ChapterSpan) VerseSpec {
	if len(spans) == 0 || len(spans[0].Verses) == 0 {
		return VerseSpec{}
	}
	return spans[0].Verses[0]
}

// ParsePassage accepts "3john 1:4-6", "3 John 1:4", "Yohanes 3:16",
// "John 3:16,17", or "Matt 5:1-7:29".
func ParsePassage(s string) (PassageRef, error) {
	s = strings.Join(strings.Fields(normalizeDashes(s)), " ")
	colon := strings.Index(s, ":")
	if colon <= 0 || colon == len(s)-1 {
		return PassageRef{}, ErrInvalidRef
	}
	left, rest := s[:colon], s[colon+1:]
	space := strings.LastIndex(left, " ")
	if space <= 0 || space == len(left)-1 {
		return PassageRef{}, ErrInvalidRef
	}
	bookPart, chStr := left[:space], left[space+1:]
	ch, err := parseVerseNum(chStr)
	if err != nil {
		return PassageRef{}, err
	}
	id, err := ResolveBookID(bookPart)
	if err != nil {
		return PassageRef{}, err
	}

	// Cross-chapter: "1-7:29" after the first colon of "5:1-7:29".
	if strings.Contains(rest, ":") {
		hy := strings.LastIndex(rest, "-")
		if hy <= 0 || hy == len(rest)-1 {
			return PassageRef{}, ErrInvalidRef
		}
		fromV, err := parseVerseNum(rest[:hy])
		if err != nil {
			return PassageRef{}, err
		}
		toCh, toV, err := parseChapterVerse(rest[hy+1:])
		if err != nil {
			return PassageRef{}, err
		}
		spans, err := expandSpans(id, ch, fromV, toCh, toV)
		if err != nil {
			return PassageRef{}, err
		}
		return PassageRef{BookID: id, Chapter: ch, Verses: firstSpec(spans), Spans: spans}, nil
	}

	specs, err := ParseVerseList(rest)
	if err != nil {
		return PassageRef{}, err
	}
	return PassageRef{
		BookID:  id,
		Chapter: ch,
		Verses:  specs[0],
		Spans:   []ChapterSpan{{Chapter: ch, Verses: specs}},
	}, nil
}

// FilterVerses returns items whose Number is inside any spec, preserving order.
// With no specs, all verses are returned.
func FilterVerses(verses []Verse, specs ...VerseSpec) []Verse {
	if len(specs) == 0 {
		return verses
	}
	out := make([]Verse, 0)
	for _, v := range verses {
		if specsContain(specs, v.Number) {
			out = append(out, v)
		}
	}
	return out
}
