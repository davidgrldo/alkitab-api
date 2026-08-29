package bible

import (
	"strings"
	"unicode"
)

func contentHits(all []VerseHit) []VerseHit {
	out := make([]VerseHit, 0, len(all))
	for _, h := range all {
		if h.Verse.Type == "" || h.Verse.Type == "content" {
			out = append(out, h)
		}
	}
	return out
}

func matchHit(h VerseHit, book, testament string) bool {
	if book != "" && h.Book != book {
		return false
	}
	if testament != "" {
		b, ok := lookupByID(h.Book)
		if !ok || !strings.EqualFold(b.Testament, testament) {
			return false
		}
	}
	return true
}

func filterHits(all []VerseHit, book, testament string) []VerseHit {
	all = contentHits(all)
	if book == "" && testament == "" {
		return all
	}
	out := make([]VerseHit, 0)
	for _, h := range all {
		if matchHit(h, book, testament) {
			out = append(out, h)
		}
	}
	return out
}

func textMatches(content, query string, wholeWord bool) bool {
	hay := strings.ToLower(content)
	needle := strings.ToLower(query)
	if needle == "" {
		return false
	}
	if !wholeWord {
		return strings.Contains(hay, needle)
	}
	return containsWord(hay, needle)
}

func containsWord(hay, needle string) bool {
	for i := 0; i <= len(hay)-len(needle); {
		j := strings.Index(hay[i:], needle)
		if j < 0 {
			return false
		}
		start := i + j
		end := start + len(needle)
		if wordBoundary(hay, start, end) {
			return true
		}
		i = start + 1
	}
	return false
}

func wordBoundary(s string, start, end int) bool {
	if start > 0 {
		r, _ := decodeLastRune(s[:start])
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	if end < len(s) {
		r, _ := decodeRune(s[end:])
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func decodeRune(s string) (rune, int) {
	for _, r := range s {
		return r, 1
	}
	return 0, 0
}

func decodeLastRune(s string) (rune, int) {
	var last rune
	for _, r := range s {
		last = r
	}
	return last, 1
}

func tokenize(s string) []string {
	var out []string
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		if b.Len() > 0 {
			out = append(out, b.String())
			b.Reset()
		}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}

// HighlightSnippet returns a window around the first case-insensitive match,
// wrapping the match in ** for clients that highlight search hits.
func HighlightSnippet(content, query string, radius int) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return content
	}
	if radius < 0 {
		radius = 40
	}
	orig := []rune(content)
	lower := []rune(strings.ToLower(content))
	needle := []rune(strings.ToLower(q))
	pos := -1
	for i := 0; i+len(needle) <= len(lower); i++ {
		match := true
		for j := range needle {
			if lower[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			pos = i
			break
		}
	}
	if pos < 0 {
		return content
	}
	start := pos - radius
	if start < 0 {
		start = 0
	}
	end := pos + len(needle) + radius
	if end > len(orig) {
		end = len(orig)
	}
	var b strings.Builder
	if start > 0 {
		b.WriteRune('…')
	}
	b.WriteString(string(orig[start:pos]))
	b.WriteString("**")
	b.WriteString(string(orig[pos : pos+len(needle)]))
	b.WriteString("**")
	b.WriteString(string(orig[pos+len(needle) : end]))
	if end < len(orig) {
		b.WriteRune('…')
	}
	return b.String()
}
