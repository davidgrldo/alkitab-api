// Package bible is the core of alkitab-api: domain types, the Source and
// Corpus contracts, a caching Engine, and a fallback Chain for composing
// sources. Pair it with an adapter such as local (embedded/BYOD JSON) or
// scrape (runtime proxy) — dependencies always point inward.
package bible

import "errors"

type Verse struct {
	Number  int    `json:"verse"`
	Content string `json:"content"`
	Type    string `json:"type"`
	Order   int    `json:"order"`
}

type Chapter struct {
	Translation string  `json:"version"`
	Book        string  `json:"book"`
	Number      int     `json:"chapter"`
	Verses      []Verse `json:"verses"`
}

type Book struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Abbreviation string `json:"abbr"`
	Testament    string `json:"testament"`
	Chapters     int    `json:"chapters"`
}

type Translation struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Language string `json:"language"`
}

type VerseHit struct {
	Translation string `json:"version"`
	Book        string `json:"book"`
	Chapter     int    `json:"chapter"`
	Verse       Verse  `json:"verse"`
}

var (
	ErrNotFound           = errors.New("bible: not found")
	ErrUnsupportedVersion = errors.New("bible: unsupported version")
	ErrUnsupportedFeature = errors.New("bible: feature not supported by active source")
)

// Source is the mandatory contract every adapter implements.
type Source interface {
	Translations() []Translation
	Books(version string) ([]Book, error)
	Chapter(version, book string, chapter int) (*Chapter, error)
}

// Corpus is an optional capability for adapters with a fully loaded in-memory
// corpus. The Engine uses it to power Search, DailyVerse, and RandomVerse.
// Network-only adapters (scrape) do not implement it.
type Corpus interface {
	AllVerses(version string) ([]VerseHit, error)
}
