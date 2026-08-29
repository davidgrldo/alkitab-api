package bible

import (
	"strconv"
	"strings"
)

// VerseSpec is an inclusive verse number range within one chapter.
type VerseSpec struct {
	From, To int
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

func parseVerseNum(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, ErrInvalidRef
	}
	return n, nil
}

// PassageRef is a single-chapter verse range in canonical book id form.
type PassageRef struct {
	BookID  string
	Chapter int
	Verses  VerseSpec
}

// ParsePassage accepts "3john 1:4-6", "3 John 1:4", or "Yohanes 3:16".
func ParsePassage(s string) (PassageRef, error) {
	s = strings.Join(strings.Fields(s), " ")
	colon := strings.LastIndex(s, ":")
	if colon <= 0 || colon == len(s)-1 {
		return PassageRef{}, ErrInvalidRef
	}
	left, versePart := s[:colon], s[colon+1:]
	space := strings.LastIndex(left, " ")
	if space <= 0 || space == len(left)-1 {
		return PassageRef{}, ErrInvalidRef
	}
	bookPart, chStr := left[:space], left[space+1:]
	ch, err := parseVerseNum(chStr)
	if err != nil {
		return PassageRef{}, err
	}
	spec, err := ParseVerseSpec(versePart)
	if err != nil {
		return PassageRef{}, err
	}
	id, err := ResolveBookID(bookPart)
	if err != nil {
		return PassageRef{}, err
	}
	return PassageRef{BookID: id, Chapter: ch, Verses: spec}, nil
}

// FilterVerses returns items whose Number is inside spec, preserving order.
func FilterVerses(verses []Verse, spec VerseSpec) []Verse {
	out := make([]Verse, 0)
	for _, v := range verses {
		if v.Number >= spec.From && v.Number <= spec.To {
			out = append(out, v)
		}
	}
	return out
}
