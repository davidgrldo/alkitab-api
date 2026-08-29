package bible

import (
	"sort"
	"strings"
)

type searchIndex struct {
	hits   []VerseHit
	lower  []string
	tokens map[string][]int
}

func buildIndex(hits []VerseHit) *searchIndex {
	ix := &searchIndex{
		hits:   hits,
		lower:  make([]string, len(hits)),
		tokens: map[string][]int{},
	}
	for i, h := range hits {
		ix.lower[i] = strings.ToLower(h.Verse.Content)
		seen := map[string]bool{}
		for _, tok := range tokenize(h.Verse.Content) {
			if seen[tok] {
				continue
			}
			seen[tok] = true
			ix.tokens[tok] = append(ix.tokens[tok], i)
		}
	}
	return ix
}

func (ix *searchIndex) lookup(query string, f SearchFilter) []VerseHit {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" || ix == nil {
		return nil
	}
	var cand []int
	switch {
	case f.WholeWord:
		toks := tokenize(q)
		if len(toks) == 0 {
			return nil
		}
		cand = append([]int(nil), ix.tokens[toks[0]]...)
		for _, t := range toks[1:] {
			cand = intersectSorted(cand, ix.tokens[t])
		}
	case strings.ContainsAny(q, " \t"):
		for i, low := range ix.lower {
			if strings.Contains(low, q) {
				cand = append(cand, i)
			}
		}
	default:
		for tok, idxs := range ix.tokens {
			if strings.Contains(tok, q) {
				cand = append(cand, idxs...)
			}
		}
		cand = uniqueSorted(cand)
	}
	out := make([]VerseHit, 0)
	for _, i := range cand {
		if i < 0 || i >= len(ix.hits) {
			continue
		}
		h := ix.hits[i]
		if !matchHit(h, f.Book, f.Testament) {
			continue
		}
		if textMatches(h.Verse.Content, query, f.WholeWord) {
			out = append(out, h)
		}
	}
	return out
}

func uniqueSorted(a []int) []int {
	if len(a) == 0 {
		return a
	}
	sort.Ints(a)
	out := a[:1]
	for _, v := range a[1:] {
		if v != out[len(out)-1] {
			out = append(out, v)
		}
	}
	return out
}

func intersectSorted(a, b []int) []int {
	a, b = uniqueSorted(append([]int(nil), a...)), uniqueSorted(append([]int(nil), b...))
	var out []int
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			out = append(out, a[i])
			i++
			j++
		case a[i] < b[j]:
			i++
		default:
			j++
		}
	}
	return out
}
