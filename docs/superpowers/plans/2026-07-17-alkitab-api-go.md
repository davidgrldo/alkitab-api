# alkitab-api (Go) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Go Bible API server (REST) with a data-source-agnostic engine, a JSON-backed `local` adapter as the default, and an opt-in `scrape` adapter for `alkitab.mobi` — shipping no copyrighted text (BYOD).

**Architecture:** Three layers pointing inward: transport (`internal/server`) → engine (`internal/bible`, pure domain) ← adapters (`internal/local`, `internal/scrape`). The engine wraps a single `Source`; a `Chain` composes sources when scrape is enabled. Adapters with a loaded corpus implement `Corpus` to power search/daily/random.

**Tech Stack:** Go 1.22+ (stdlib `net/http` routing, `embed`, `testing`), one dependency `github.com/PuerkitoBio/goquery` for the scrape adapter.

## Global Constraints

- Module path: `github.com/davidgrldo/alkitab-api` (already in `go.mod`, `go 1.26.2`).
- No copyrighted translation text is ever committed. Only public-domain KJV sample data is embedded.
- Stdlib only, except `goquery` in the scrape adapter.
- Error mapping lives in the HTTP layer only; adapters return typed errors from `internal/bible`.
- JSON in/out; errors use `{"error":"<msg>"}`.
- Every non-trivial unit ships with `testing`-stdlib tests; `go test ./...` must pass before a task is done.

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/bible/types.go` | Domain types (`Verse`, `Chapter`, `Book`, `Translation`, `VerseHit`), typed errors, `Source` + `Corpus` interfaces |
| `internal/bible/metadata.go` | Canonical 66-book table (EN + Indonesian names), `ResolveBookID`, `IndonesianBookName`, `CanonicalBooks` |
| `internal/bible/engine.go` | `Engine` (Chapter cache, Search, DailyVerse, RandomVerse) + `Chain` composite source |
| `internal/bible/*_test.go` | Engine + metadata tests |
| `internal/local/local.go` | `local` adapter: embed JSON + optional data dir, `Source` + `Corpus` impl |
| `internal/local/data/kjv.json` | Embedded PD sample (3 John + Philemon, KJV) |
| `internal/scrape/scrape.go` | `scrape` adapter: `alkitab.mobi` HTML via goquery, `Source` impl only |
| `internal/server/server.go` | REST handlers + error mapping (Go 1.22 routing) |
| `cmd/alkitab-api/main.go` | Entrypoint: env wiring, build Chain, start HTTP |
| `README.md` | Usage, BYOD format, copyright note |

---

## Task 1: Project setup + domain types

**Files:**
- Create: `internal/bible/types.go`
- Create: `internal/bible/types_test.go`
- Modify: `go.mod` (ensure module path)

**Interfaces:**
- Consumes: nothing
- Produces: types `Verse, Chapter, Book, Translation, VerseHit`; errors `ErrNotFound, ErrUnsupportedVersion, ErrUnsupportedFeature`; interfaces `Source{Translations() []Translation; Books(version string) ([]Book, error); Chapter(version, book string, chapter int) (*Chapter, error)}` and `Corpus{AllVerses(version string) ([]VerseHit, error)}`.

- [ ] **Step 1: Initialize git and confirm module**

```bash
git init
go mod tidy
```
Expected: `go.mod` already contains `module github.com/davidgrldo/alkitab-api`.

- [ ] **Step 2: Write the failing test**

`internal/bible/types_test.go`:
```go
package bible

import (
	"errors"
	"fmt"
	"testing"
)

func TestTypesAndErrors(t *testing.T) {
	v := Verse{Number: 1, Content: "In the beginning", Type: "content", Order: 0}
	if v.Number != 1 || v.Type != "content" {
		t.Fatalf("verse not constructed: %+v", v)
	}
	if !errors.Is(ErrNotFound, ErrNotFound) {
		t.Fatal("ErrNotFound must support errors.Is")
	}
}

// fakeSource is reused by engine tests in later tasks.
type fakeSource struct {
	trans []Translation
	books []Book
	chaps map[string]*Chapter
	calls int
}

func (f *fakeSource) Translations() []Translation            { return f.trans }
func (f *fakeSource) Books(version string) ([]Book, error)   { return f.books, nil }
func (f *fakeSource) Chapter(version, book string, chapter int) (*Chapter, error) {
	f.calls++
	c, ok := f.chaps[fmt.Sprintf("%s:%s:%d", version, book, chapter)]
	if !ok {
		return nil, ErrNotFound
	}
	return c, nil
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/bible/ -run TestTypesAndErrors -v`
Expected: FAIL — `undefined: Verse` etc. (types.go not yet created).

- [ ] **Step 4: Write minimal implementation**

`internal/bible/types.go`:
```go
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
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/bible/ -run TestTypesAndErrors -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/bible/types.go internal/bible/types_test.go
git commit -m "feat(bible): add domain types, errors, and Source/Corpus interfaces"
```

---

## Task 2: Canonical book metadata

**Files:**
- Create: `internal/bible/metadata.go`
- Create: `internal/bible/metadata_test.go`

**Interfaces:**
- Consumes: `ErrNotFound` (Task 1)
- Produces: `func ResolveBookID(s string) (string, error)` returning canonical lowercase id; `func IndonesianBookName(id string) string`; `func CanonicalBooks() []Book` (66 entries).

- [ ] **Step 1: Write the failing test**

`internal/bible/metadata_test.go`:
```go
package bible

import "testing"

func TestResolveBookID(t *testing.T) {
	cases := []struct{ in, want string }{
		{"gen", "gen"},
		{"Genesis", "gen"},
		{"GENESIS", "gen"},
		{"Kejadian", "gen"},
		{"kej", "gen"},
		{"Mazmur", "ps"},
		{"Psalms", "ps"},
		{"3 john", "3john"},
		{"3Yoh", "3john"},
		{"phlm", "phlm"},
		{"Filemon", "phlm"},
	}
	for _, c := range cases {
		got, err := ResolveBookID(c.in)
		if err != nil || got != c.want {
			t.Errorf("ResolveBookID(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
	if _, err := ResolveBookID("nope"); !errorIs(err, ErrNotFound) {
		t.Errorf("unknown book want ErrNotFound, got %v", err)
	}
}

func TestCanonCounts(t *testing.T) {
	books := CanonicalBooks()
	if len(books) != 66 {
		t.Fatalf("canon has %d books, want 66", len(books))
	}
	ot, nt := 0, 0
	for _, b := range books {
		switch b.Testament {
		case "OT":
			ot++
		case "NT":
			nt++
		}
	}
	if ot != 39 || nt != 27 {
		t.Errorf("testaments OT=%d NT=%d; want 39/27", ot, nt)
	}
}

func TestIndonesianBookName(t *testing.T) {
	if got := IndonesianBookName("ps"); got != "Mazmur" {
		t.Errorf("IndonesianBookName(ps)=%q want Mazmur", got)
	}
	if got := IndonesianBookName("gen"); got != "Kejadian" {
		t.Errorf("IndonesianBookName(gen)=%q want Kejadian", got)
	}
}

func errorIs(err, target error) bool {
	return err == target
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/bible/ -run "TestResolveBookID|TestCanonCounts|TestIndonesianBookName" -v`
Expected: FAIL — `undefined: ResolveBookID`.

- [ ] **Step 3: Write minimal implementation**

`internal/bible/metadata.go`:
```go
package bible

import "strings"

type canonical struct {
	ID, EnName, EnAbbr, IdName, IdAbbr, Testament string
	Chapters                                     int
}

// canon: 66 books, English + Indonesian names and abbreviations, chapter counts.
var canon = []canonical{
	{"gen", "Genesis", "Gen", "Kejadian", "Kej", "OT", 50},
	{"exod", "Exodus", "Ex", "Keluaran", "Kel", "OT", 40},
	{"lev", "Leviticus", "Lev", "Imamat", "Im", "OT", 27},
	{"num", "Numbers", "Num", "Bilangan", "Bil", "OT", 36},
	{"deut", "Deuteronomy", "Deut", "Ulangan", "Ul", "OT", 34},
	{"josh", "Joshua", "Josh", "Yosua", "Yos", "OT", 24},
	{"judg", "Judges", "Judg", "Hakim-Hakim", "Hak", "OT", 21},
	{"ruth", "Ruth", "Ruth", "Rut", "Rut", "OT", 4},
	{"1sam", "1 Samuel", "1Sam", "1 Samuel", "1Sam", "OT", 31},
	{"2sam", "2 Samuel", "2Sam", "2 Samuel", "2Sam", "OT", 24},
	{"1kgs", "1 Kings", "1Kgs", "1 Raja-Raja", "1Raj", "OT", 22},
	{"2kgs", "2 Kings", "2Kgs", "2 Raja-Raja", "2Raj", "OT", 25},
	{"1chr", "1 Chronicles", "1Chr", "1 Tawarikh", "1Taw", "OT", 29},
	{"2chr", "2 Chronicles", "2Chr", "2 Tawarikh", "2Taw", "OT", 36},
	{"ezra", "Ezra", "Ezra", "Ezra", "Ezr", "OT", 10},
	{"neh", "Nehemiah", "Neh", "Nehemia", "Neh", "OT", 13},
	{"esth", "Esther", "Esth", "Ester", "Est", "OT", 10},
	{"job", "Job", "Job", "Ayub", "Ayub", "OT", 42},
	{"ps", "Psalms", "Ps", "Mazmur", "Maz", "OT", 150},
	{"prov", "Proverbs", "Prov", "Amsal", "Ams", "OT", 31},
	{"eccl", "Ecclesiastes", "Eccl", "Pengkhotbah", "Pkh", "OT", 12},
	{"song", "Song of Solomon", "Song", "Kidung Agung", "Kid", "OT", 8},
	{"isa", "Isaiah", "Isa", "Yesaya", "Yes", "OT", 66},
	{"jer", "Jeremiah", "Jer", "Yeremia", "Yer", "OT", 52},
	{"lam", "Lamentations", "Lam", "Ratapan", "Rat", "OT", 5},
	{"ezek", "Ezekiel", "Ezek", "Yehezkiel", "Yeh", "OT", 48},
	{"dan", "Daniel", "Dan", "Daniel", "Dan", "OT", 12},
	{"hos", "Hosea", "Hos", "Hosea", "Hos", "OT", 14},
	{"joel", "Joel", "Joel", "Yoel", "Yoel", "OT", 3},
	{"amos", "Amos", "Amos", "Amos", "Amos", "OT", 9},
	{"obad", "Obadiah", "Obad", "Obaja", "Oba", "OT", 1},
	{"jonah", "Jonah", "Jonah", "Yunus", "Yun", "OT", 4},
	{"mic", "Micah", "Mic", "Mikha", "Mik", "OT", 7},
	{"nah", "Nahum", "Nah", "Nahum", "Nah", "OT", 3},
	{"hab", "Habakkuk", "Hab", "Habakuk", "Hab", "OT", 3},
	{"zeph", "Zephaniah", "Zeph", "Zefanya", "Zef", "OT", 3},
	{"hag", "Haggai", "Hag", "Hagai", "Hag", "OT", 2},
	{"zech", "Zechariah", "Zech", "Zakharia", "Zak", "OT", 14},
	{"mal", "Malachi", "Mal", "Maleakhi", "Mal", "OT", 4},
	{"matt", "Matthew", "Matt", "Matius", "Mat", "NT", 28},
	{"mark", "Mark", "Mark", "Markus", "Mrk", "NT", 16},
	{"luke", "Luke", "Luke", "Lukas", "Luk", "NT", 24},
	{"john", "John", "John", "Yohanes", "Yoh", "NT", 21},
	{"acts", "Acts", "Acts", "Kisah Para Rasul", "Kis", "NT", 28},
	{"rom", "Romans", "Rom", "Roma", "Rom", "NT", 16},
	{"1cor", "1 Corinthians", "1Cor", "1 Korintus", "1Kor", "NT", 16},
	{"2cor", "2 Corinthians", "2Cor", "2 Korintus", "2Kor", "NT", 13},
	{"gal", "Galatians", "Gal", "Galatia", "Gal", "NT", 6},
	{"eph", "Ephesians", "Eph", "Efesus", "Ef", "NT", 6},
	{"phil", "Philippians", "Phil", "Filipi", "Flp", "NT", 4},
	{"col", "Colossians", "Col", "Kolose", "Kol", "NT", 4},
	{"1thess", "1 Thessalonians", "1Thess", "1 Tesalonika", "1Tes", "NT", 5},
	{"2thess", "2 Thessalonians", "2Thess", "2 Tesalonika", "2Tes", "NT", 3},
	{"1tim", "1 Timothy", "1Tim", "1 Timotius", "1Tim", "NT", 6},
	{"2tim", "2 Timothy", "2Tim", "2 Timotius", "2Tim", "NT", 4},
	{"titus", "Titus", "Titus", "Titus", "Tit", "NT", 3},
	{"phlm", "Philemon", "Phlm", "Filemon", "Flm", "NT", 1},
	{"heb", "Hebrews", "Heb", "Ibrani", "Ibr", "NT", 13},
	{"jas", "James", "Jas", "Yakobus", "Yak", "NT", 5},
	{"1pet", "1 Peter", "1Pet", "1 Petrus", "1Pet", "NT", 5},
	{"2pet", "2 Peter", "2Pet", "2 Petrus", "2Pet", "NT", 3},
	{"1john", "1 John", "1John", "1 Yohanes", "1Yoh", "NT", 5},
	{"2john", "2 John", "2John", "2 Yohanes", "2Yoh", "NT", 1},
	{"3john", "3 John", "3John", "3 Yohanes", "3Yoh", "NT", 1},
	{"jude", "Jude", "Jude", "Yudas", "Yud", "NT", 1},
	{"rev", "Revelation", "Rev", "Wahyu", "Wah", "NT", 22},
}

func lookupByID(id string) (canonical, bool) {
	for _, b := range canon {
		if b.ID == id {
			return b, true
		}
	}
	return canonical{}, false
}

// ResolveBookID resolves an id, English name/abbr, or Indonesian name/abbr
// (case-insensitive) to the canonical lowercase book id.
func ResolveBookID(s string) (string, error) {
	l := strings.ToLower(strings.TrimSpace(s))
	for _, b := range canon {
		if l == strings.ToLower(b.ID) ||
			l == strings.ToLower(b.EnName) ||
			l == strings.ToLower(b.EnAbbr) ||
			l == strings.ToLower(b.IdName) ||
			l == strings.ToLower(b.IdAbbr) {
			return b.ID, nil
		}
	}
	return "", ErrNotFound
}

// IndonesianBookName returns the Indonesian name for a canonical id (used by
// the scrape adapter to build alkitab.mobi URLs). Empty string if unknown.
func IndonesianBookName(id string) string {
	if b, ok := lookupByID(id); ok {
		return b.IdName
	}
	return ""
}

// CanonicalBooks returns all 66 books as domain Book values (English names).
func CanonicalBooks() []Book {
	out := make([]Book, 0, len(canon))
	for _, b := range canon {
		out = append(out, Book{
			ID: b.ID, Name: b.EnName, Abbreviation: b.EnAbbr,
			Testament: b.Testament, Chapters: b.Chapters,
		})
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/bible/ -run "TestResolveBookID|TestCanonCounts|TestIndonesianBookName" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/bible/metadata.go internal/bible/metadata_test.go
git commit -m "feat(bible): add canonical 66-book metadata and book id resolver"
```

---

## Task 3: Engine — Chapter lookup + cache

**Files:**
- Create: `internal/bible/engine.go`
- Create: `internal/bible/engine_test.go`

**Interfaces:**
- Consumes: `Source`, `Chapter`, `ErrNotFound` (Task 1)
- Produces: `func New(src Source) *Engine`; method `func (e *Engine) Chapter(version, book string, chapter int) (*Chapter, error)` with in-memory cache.

- [ ] **Step 1: Write the failing test**

`internal/bible/engine_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/bible/ -run TestEngine -v`
Expected: FAIL — `undefined: New`.

- [ ] **Step 3: Write minimal implementation**

`internal/bible/engine.go`:
```go
package bible

import (
	"fmt"
	"sync"
)

// Engine wraps a Source with a chapter cache and corpus-backed operations.
type Engine struct {
	src   Source
	mu    sync.RWMutex
	cache map[string]*Chapter
}

func New(src Source) *Engine {
	return &Engine{src: src, cache: make(map[string]*Chapter)}
}

func chapterKey(version, book string, chapter int) string {
	return fmt.Sprintf("%s:%s:%d", version, book, chapter)
}

func (e *Engine) Chapter(version, book string, chapter int) (*Chapter, error) {
	key := chapterKey(version, book, chapter)
	e.mu.RLock()
	if c, ok := e.cache[key]; ok {
		e.mu.RUnlock()
		return c, nil
	}
	e.mu.RUnlock()

	c, err := e.src.Chapter(version, book, chapter)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	e.cache[key] = c
	e.mu.Unlock()
	return c, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/bible/ -run TestEngine -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/bible/engine.go internal/bible/engine_test.go
git commit -m "feat(bible): add Engine with cached chapter lookup"
```

---

## Task 4: Engine — Search + Corpus capability

**Files:**
- Modify: `internal/bible/engine.go` (append Search)
- Modify: `internal/bible/engine_test.go` (append tests)

**Interfaces:**
- Consumes: `Corpus` (Task 1), `VerseHit`
- Produces: `func (e *Engine) Search(version, query string) ([]VerseHit, error)` — returns `ErrUnsupportedFeature` if the source is not a `Corpus`; else case-insensitive substring scan.

- [ ] **Step 1: Write the failing test**

Append to `internal/bible/engine_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/bible/ -run "TestEngineSearch" -v`
Expected: FAIL — `e.Search undefined`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/bible/engine.go`:
```go
import "strings" // add to existing import block

func (e *Engine) corpus() (Corpus, bool) {
	c, ok := e.src.(Corpus)
	return c, ok
}

// Search returns verses whose content contains query (case-insensitive).
func (e *Engine) Search(version, query string) ([]VerseHit, error) {
	c, ok := e.corpus()
	if !ok {
		return nil, ErrUnsupportedFeature
	}
	all, err := c.AllVerses(version)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var hits []VerseHit
	for _, h := range all {
		if strings.Contains(strings.ToLower(h.Verse.Content), q) {
			hits = append(hits, h)
		}
	}
	return hits, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/bible/ -run TestEngineSearch -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/bible/engine.go internal/bible/engine_test.go
git commit -m "feat(bible): add Engine.Search via Corpus capability"
```

---

## Task 5: Engine — DailyVerse + RandomVerse

**Files:**
- Modify: `internal/bible/engine.go` (append Daily/Random + helpers)
- Modify: `internal/bible/engine_test.go` (append tests)

**Interfaces:**
- Consumes: `Corpus`, `VerseHit`
- Produces: `func (e *Engine) DailyVerse(version string, t time.Time) (*VerseHit, error)` deterministic by date+version; `func (e *Engine) RandomVerse(version string) (*VerseHit, error)`.

- [ ] **Step 1: Write the failing test**

Append to `internal/bible/engine_test.go`:
```go
import "time" // add to imports

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
	if h1 != h2 {
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/bible/ -run "TestEngineDaily|TestEngineRandom" -v`
Expected: FAIL — `e.DailyVerse undefined`.

- [ ] **Step 3: Write minimal implementation**

Add imports `"hash/fnv"`, `"math/rand/v2"`, `"time"` to `engine.go`, then append:
```go
// DailyVerse returns a deterministic verse for the given date and version:
// seed = fnv(date+version) % len(corpus). Same date+version always agrees.
func (e *Engine) DailyVerse(version string, t time.Time) (*VerseHit, error) {
	c, ok := e.corpus()
	if !ok {
		return nil, ErrUnsupportedFeature
	}
	all, err := c.AllVerses(version)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, ErrNotFound
	}
	seed := hashSeed(fmt.Sprintf("%04d%02d%02d%s", t.Year(), int(t.Month()), t.Day(), version))
	h := all[seed%uint32(len(all))]
	return &h, nil
}

// RandomVerse returns an unpredictable verse. Auto-seeded via math/rand/v2.
func (e *Engine) RandomVerse(version string) (*VerseHit, error) {
	c, ok := e.corpus()
	if !ok {
		return nil, ErrUnsupportedFeature
	}
	all, err := c.AllVerses(version)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, ErrNotFound
	}
	h := all[rand.IntN(len(all))]
	return &h, nil
}

func hashSeed(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/bible/ -run "TestEngineDaily|TestEngineRandom" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/bible/engine.go internal/bible/engine_test.go
git commit -m "feat(bible): add deterministic DailyVerse and RandomVerse"
```

---

## Task 6: Chain composite source

**Files:**
- Modify: `internal/bible/engine.go` (append Chain type)
- Modify: `internal/bible/engine_test.go` (append Chain tests)

**Interfaces:**
- Consumes: `Source`, `Corpus`, `ErrNotFound`
- Produces: `func NewChain(sources ...Source) *Chain`; `Chain` implements `Source` (fallback on `ErrNotFound`, merged `Books`/`Translations`) and `Corpus` (delegates to the first member that implements `Corpus`).

- [ ] **Step 1: Write the failing test**

Append to `internal/bible/engine_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/bible/ -run TestChain -v`
Expected: FAIL — `undefined: NewChain`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/bible/engine.go`:
```go
// Chain is a Source that delegates to members in order, returning the first
// non-ErrNotFound result. Books/Translations are merged across members.
type Chain struct {
	sources []Source
}

func NewChain(sources ...Source) *Chain {
	return &Chain{sources: sources}
}

func (c *Chain) Translations() []Translation {
	var out []Translation
	seen := map[string]bool{}
	for _, s := range c.sources {
		for _, t := range s.Translations() {
			if !seen[t.ID] {
				seen[t.ID] = true
				out = append(out, t)
			}
		}
	}
	return out
}

func (c *Chain) Books(version string) ([]Book, error) {
	for _, s := range c.sources {
		b, err := s.Books(version)
		if err == nil {
			return b, nil
		}
	}
	return nil, ErrNotFound
}

func (c *Chain) Chapter(version, book string, chapter int) (*Chapter, error) {
	for _, s := range c.sources {
		ch, err := s.Chapter(version, book, chapter)
		if err == nil {
			return ch, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}
	return nil, ErrNotFound
}

// AllVerses delegates to the first member that implements Corpus, so that
// Search/Daily/Random keep working when local and scrape are chained.
func (c *Chain) AllVerses(version string) ([]VerseHit, error) {
	for _, s := range c.sources {
		if corp, ok := s.(Corpus); ok {
			return corp.AllVerses(version)
		}
	}
	return nil, ErrUnsupportedFeature
}
```

Add `"errors"` to the import block of `engine.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/bible/ -run TestChain -v`
Expected: PASS

Then run the whole package: `go test ./internal/bible/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/bible/engine.go internal/bible/engine_test.go
git commit -m "feat(bible): add Chain composite source with fallback and Corpus delegation"
```

---

## Task 7: local adapter — JSON load + Source

**Files:**
- Create: `internal/local/local.go`
- Create: `internal/local/local_test.go`
- Create: `internal/local/data/kjv.json`

**Interfaces:**
- Consumes: `bible.Source`, `bible.Book`, `bible.Chapter`, `bible.Translation`, `bible.Verse`, `bible.ErrNotFound` (Tasks 1–2)
- Produces: `func New(dataDir string) (*Local, error)` (loads `//go:embed data/*.json` plus optional runtime dir); `*Local` implements `bible.Source`.

- [ ] **Step 1: Create the embedded sample data**

`internal/local/data/kjv.json` (public-domain KJV subset — 3 John and Philemon):
```json
{
  "translation": {"id": "kjv", "name": "King James Version", "language": "en"},
  "books": [
    {
      "id": "3john", "name": "3 John", "abbr": "3John", "testament": "NT", "chapters": 1,
      "chapter_data": [
        {"number": 1, "verses": [
          {"verse": 1, "type": "content", "content": "The elder unto the wellbeloved Gaius, whom I love in the truth."},
          {"verse": 2, "type": "content", "content": "Beloved, I wish above all things that thou mayest prosper and be in health, even as thy soul prospereth."},
          {"verse": 3, "type": "content", "content": "For I rejoiced greatly, when the brethren came and testified of the truth that is in thee, even as thou walkest in the truth."},
          {"verse": 4, "type": "content", "content": "I have no greater joy than to hear that my children walk in truth."},
          {"verse": 5, "type": "content", "content": "Beloved, thou doest faithfully whatsoever thou doest to the brethren, and to strangers;"},
          {"verse": 6, "type": "content", "content": "Which have borne witness of thy charity before the church: whom if thou bring forward on their journey after a godly sort, thou shalt do well:"},
          {"verse": 7, "type": "content", "content": "Because that for his name's sake they went forth, taking nothing of the Gentiles."},
          {"verse": 8, "type": "content", "content": "We therefore ought to receive such, that we might be fellowhelpers to the truth."},
          {"verse": 9, "type": "content", "content": "I wrote unto the church: but Diotrephes, who loveth to have the preeminence among them, receiveth us not."},
          {"verse": 10, "type": "content", "content": "Wherefore, if I come, I will remember his deeds which he doeth, prating against us with malicious words: and not content therewith, neither doth he himself receive the brethren, and forbiddeth them that would, and casteth them out of the church."},
          {"verse": 11, "type": "content", "content": "Beloved, follow not that which is evil, but that which is good. He that doeth good is of God: but he that doeth evil hath not seen God."},
          {"verse": 12, "type": "content", "content": "Demetrius hath good report of all men, and of the truth itself: yea, and we also bear record; and ye know that our record is true."},
          {"verse": 13, "type": "content", "content": "I had many things to write, but I will not with ink and pen write unto thee:"},
          {"verse": 14, "type": "content", "content": "But I trust I shall shortly see thee, and we shall speak face to face. Peace be to thee. Our friends salute thee. Greet the friends by name."}
        ]}
      ]
    },
    {
      "id": "phlm", "name": "Philemon", "abbr": "Phlm", "testament": "NT", "chapters": 1,
      "chapter_data": [
        {"number": 1, "verses": [
          {"verse": 1, "type": "content", "content": "Paul, a prisoner of Jesus Christ, and Timothy our brother, unto Philemon our dearly beloved, and fellowlabourer,"},
          {"verse": 2, "type": "content", "content": "And to our beloved Apphia, and Archippus our fellowsoldier, and to the church in thy house:"},
          {"verse": 3, "type": "content", "content": "Grace to you, and peace, from God our Father and the Lord Jesus Christ."},
          {"verse": 4, "type": "content", "content": "I thank my God, making mention of thee always in my prayers,"},
          {"verse": 5, "type": "content", "content": "Hearing of thy love and faith, which thou hast toward the Lord Jesus, and toward all saints;"},
          {"verse": 6, "type": "content", "content": "That the communication of thy faith may become effectual by the acknowledging of every good thing which is in you in Christ Jesus."},
          {"verse": 7, "type": "content", "content": "For we have great joy and consolation in thy love, because the bowels of the saints are refreshed by thee, brother."},
          {"verse": 8, "type": "content", "content": "Wherefore, though I might be much bold in Christ to enjoin thee that which is convenient,"},
          {"verse": 9, "type": "content", "content": "Yet for love's sake I rather beseech thee, being such an one as Paul the aged, and now also a prisoner of Jesus Christ."},
          {"verse": 10, "type": "content", "content": "I beseech thee for my son Onesimus, whom I have begotten in my bonds:"},
          {"verse": 11, "type": "content", "content": "Which in time past was to thee unprofitable, but now profitable to thee and to me:"},
          {"verse": 12, "type": "content", "content": "Whom I have sent again: thou therefore receive him, that is, mine own bowels."},
          {"verse": 13, "type": "content", "content": "Whom I would have retained with me, that in thy stead he might have ministered unto me in the bonds of the gospel:"},
          {"verse": 14, "type": "content", "content": "But without thy mind would I do nothing; that thy benefit should not be as it were of necessity, but willingly."},
          {"verse": 15, "type": "content", "content": "For perhaps he therefore departed for a season, that thou shouldest receive him for ever;"},
          {"verse": 16, "type": "content", "content": "Not now as a servant, but above a servant, a brother beloved, specially to me, but how much more unto thee, both in the flesh, and in the Lord?"},
          {"verse": 17, "type": "content", "content": "If thou count me therefore a partner, receive him as myself."},
          {"verse": 18, "type": "content", "content": "If he have wronged thee, or oweth thee ought, put that on mine account;"},
          {"verse": 19, "type": "content", "content": "I Paul have written it with mine own hand, I will repay it: albeit I do not say to thee how thou owest unto me even thine own self besides."},
          {"verse": 20, "type": "content", "content": "Yea, brother, let me have joy of thee in the Lord: refresh my bowels in the Lord."},
          {"verse": 21, "type": "content", "content": "Having confidence in thy obedience I wrote unto thee, knowing that thou wilt also do more than I say."},
          {"verse": 22, "type": "content", "content": "But withal prepare me also a lodging: for I trust that through your prayers I shall be given unto you."},
          {"verse": 23, "type": "content", "content": "There salute thee Epaphras, my fellowprisoner in Christ Jesus;"},
          {"verse": 24, "type": "content", "content": "Marcus, Aristarchus, Demas, Lucas, my fellowlabourers."},
          {"verse": 25, "type": "content", "content": "The grace of our Lord Jesus Christ be with your spirit. Amen."}
        ]}
      ]
    }
  ]
}
```

- [ ] **Step 2: Write the failing test**

`internal/local/local_test.go`:
```go
package local

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidgrldo/alkitab-api/internal/bible"
)

func TestLocalLoadsEmbedded(t *testing.T) {
	l, err := New("")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	trans := l.Translations()
	if len(trans) != 1 || trans[0].ID != "kjv" {
		t.Fatalf("translations = %+v", trans)
	}
}

func TestLocalBooks(t *testing.T) {
	l, _ := New("")
	books, err := l.Books("kjv")
	if err != nil {
		t.Fatalf("Books: %v", err)
	}
	if len(books) != 2 {
		t.Fatalf("want 2 books in sample, got %d", len(books))
	}
	if _, err := l.Books("nope"); !errors.Is(err, bible.ErrUnsupportedVersion) {
		t.Errorf("unknown version want ErrUnsupportedVersion, got %v", err)
	}
}

func TestLocalChapter(t *testing.T) {
	l, _ := New("")
	c, err := l.Chapter("kjv", "3john", 1)
	if err != nil {
		t.Fatalf("Chapter: %v", err)
	}
	if len(c.Verses) != 14 {
		t.Errorf("3john has %d verses, want 14", len(c.Verses))
	}
	if _, err := l.Chapter("kjv", "3john", 9); !errors.Is(err, bible.ErrNotFound) {
		t.Errorf("missing chapter want ErrNotFound, got %v", err)
	}
}

func TestLocalRuntimeDirOverrides(t *testing.T) {
	dir := t.TempDir()
	custom := `{"translation":{"id":"kjv","name":"Custom","language":"en"},"books":[]}`
	if err := os.WriteFile(filepath.Join(dir, "kjv.json"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	trans := l.Translations()
	if trans[0].Name != "Custom" {
		t.Errorf("runtime override failed: %+v", trans)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/local/ -v`
Expected: FAIL — `undefined: New` (package doesn't exist yet).

- [ ] **Step 4: Write minimal implementation**

`internal/local/local.go`:
```go
package local

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidgrldo/alkitab-api/internal/bible"
)

//go:embed data/*.json
var embedded embed.FS

type fileData struct {
	Translation bible.Translation `json:"translation"`
	Books       []struct {
		bible.Book
		ChapterData []struct {
			Number int           `json:"number"`
			Verses []bible.Verse `json:"verses"`
		} `json:"chapter_data"`
	} `json:"books"`
}

type translationData struct {
	Translation bible.Translation
	Books       []bible.Book
	Chapters    map[string]*bible.Chapter // key: book:chapter
}

// Local is a bible.Source backed by JSON files (embedded + optional data dir).
type Local struct {
	data map[string]*translationData
}

func New(dataDir string) (*Local, error) {
	l := &Local{data: map[string]*translationData{}}

	// embedded defaults
	entries, err := embedded.ReadDir("data")
	if err != nil {
		return nil, fmt.Errorf("local: read embed: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		b, err := embedded.ReadFile(filepath.Join("data", e.Name()))
		if err != nil {
			return nil, err
		}
		if err := l.load(b); err != nil {
			return nil, fmt.Errorf("local: parse %s: %w", e.Name(), err)
		}
	}

	// runtime overrides
	if dataDir != "" {
		des, err := os.ReadDir(dataDir)
		if err != nil {
			return nil, fmt.Errorf("local: read data dir %q: %w", dataDir, err)
		}
		for _, e := range des {
			if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dataDir, e.Name()))
			if err != nil {
				return nil, err
			}
			if err := l.load(b); err != nil {
				return nil, fmt.Errorf("local: parse %s: %w", e.Name(), err)
			}
		}
	}
	return l, nil
}

func (l *Local) load(raw []byte) error {
	var fd fileData
	if err := json.Unmarshal(raw, &fd); err != nil {
		return err
	}
	td := &translationData{
		Translation: fd.Translation,
		Chapters:    map[string]*bible.Chapter{},
	}
	for _, bk := range fd.Books {
		b := bk.Book
		td.Books = append(td.Books, b)
		for _, ch := range bk.ChapterData {
			td.Chapters[fmt.Sprintf("%s:%d", b.ID, ch.Number)] = &bible.Chapter{
				Translation: fd.Translation.ID,
				Book:        b.ID,
				Number:      ch.Number,
				Verses:      ch.Verses,
			}
		}
	}
	l.data[fd.Translation.ID] = td // later load with same id overrides earlier
	return nil
}

func (l *Local) Translations() []bible.Translation {
	out := make([]bible.Translation, 0, len(l.data))
	for _, td := range l.data {
		out = append(out, td.Translation)
	}
	return out
}

func (l *Local) Books(version string) ([]bible.Book, error) {
	td, ok := l.data[version]
	if !ok {
		return nil, bible.ErrUnsupportedVersion
	}
	return td.Books, nil
}

func (l *Local) Chapter(version, book string, chapter int) (*bible.Chapter, error) {
	td, ok := l.data[version]
	if !ok {
		return nil, bible.ErrUnsupportedVersion
	}
	c, ok := td.Chapters[fmt.Sprintf("%s:%d", book, chapter)]
	if !ok {
		return nil, bible.ErrNotFound
	}
	return c, nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/local/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/local/ go.sum
git commit -m "feat(local): add JSON-backed local adapter with embedded PD sample"
```

---

## Task 8: local adapter — Corpus (AllVerses)

**Files:**
- Modify: `internal/local/local.go` (append `AllVerses`)
- Modify: `internal/local/local_test.go` (append test)

**Interfaces:**
- Consumes: `bible.Corpus`, `bible.VerseHit`
- Produces: `func (l *Local) AllVerses(version string) ([]bible.VerseHit, error)` — flattens the loaded corpus in canonical book/chapter/verse order.

- [ ] **Step 1: Write the failing test**

Append to `internal/local/local_test.go`:
```go
func TestLocalAllVerses(t *testing.T) {
	l, _ := New("")
	all, err := l.AllVerses("kjv")
	if err != nil {
		t.Fatalf("AllVerses: %v", err)
	}
	// 3 John (14) + Philemon (25) = 39
	if len(all) != 39 {
		t.Errorf("want 39 verses in sample corpus, got %d", len(all))
	}
	for _, h := range all {
		if h.Translation != "kjv" {
			t.Errorf("hit translation = %q", h.Translation)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/local/ -run TestLocalAllVerses -v`
Expected: FAIL — `l.AllVerses undefined`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/local/local.go`:
```go
// AllVerses flattens a translation's corpus into VerseHit order.
// Books are emitted in the order they appear in the JSON; within a chapter,
// verses are emitted in stored order.
func (l *Local) AllVerses(version string) ([]bible.VerseHit, error) {
	td, ok := l.data[version]
	if !ok {
		return nil, bible.ErrUnsupportedVersion
	}
	var out []bible.VerseHit
	for _, b := range td.Books {
		for ch := 1; ch <= b.Chapters; ch++ {
			c, ok := td.Chapters[fmt.Sprintf("%s:%d", b.ID, ch)]
			if !ok {
				continue
			}
			for _, v := range c.Verses {
				out = append(out, bible.VerseHit{
					Translation: version, Book: b.ID, Chapter: ch, Verse: v,
				})
			}
		}
	}
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/local/ -v`
Expected: PASS (all local tests).

- [ ] **Step 5: Commit**

```bash
git add internal/local/local.go internal/local/local_test.go
git commit -m "feat(local): implement Corpus with flattened AllVerses"
```

---

## Task 9: scrape adapter

**Files:**
- Create: `internal/scrape/scrape.go`
- Create: `internal/scrape/scrape_test.go`
- Create: `internal/scrape/testdata/3john.html` (fixture)

**Interfaces:**
- Consumes: `bible.Source`, `bible.CanonicalBooks`, `bible.IndonesianBookName`, `bible.Verse`, `bible.Chapter`, `bible.ErrNotFound`, `bible.ErrUnsupportedVersion`
- Produces: `func New(baseURL string) *Scrape`; `*Scrape` implements `bible.Source` (not `Corpus`). Base URL defaults to `https://alkitab.mobi` when passed empty.

- [ ] **Step 1: Add the dependency**

Run: `go get github.com/PuerkitoBio/goquery && go mod tidy`

- [ ] **Step 2: Create the HTML fixture**

`internal/scrape/testdata/3john.html` (a trimmed slice mirroring alkitab.mobi's `p > [data-begin]`, `.paragraphtitle`, `.reftext` structure):
```html
<html><body>
<p><span class="reftext"><a>1</a></span><span data-begin="1">The elder unto the wellbeloved Gaius.</span></p>
<p><span class="paragraphtitle">Greeting</span></p>
<p><span class="reftext"><a>2</a></span><span data-begin="2">I wish above all things that thou mayest prosper.</span></p>
<p hidden="hidden"><span class="reftext"><a>3</a></span>should be skipped</p>
<p class="loading"><span class="reftext"><a>4</a></span>should be skipped</p>
</body></html>
```

- [ ] **Step 3: Write the failing test**

`internal/scrape/scrape_test.go`:
```go
package scrape

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/davidgrldo/alkitab-api/internal/bible"
)

func TestScrapeChapter(t *testing.T) {
	// The test handler serves the fixture regardless of path, so the scrape
	// adapter's URL construction (which uses the Indonesian book name) does not
	// matter for parsing correctness.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "testdata/3john.html")
	}))
	defer srv.Close()

	s := New(srv.URL)
	c, err := s.Chapter("tb", "3john", 1)
	if err != nil {
		t.Fatalf("Chapter: %v", err)
	}
	// hidden and loading <p> are skipped; 3 items remain (1 content + 1 title + 1 content)
	if len(c.Verses) != 3 {
		t.Fatalf("want 3 items, got %d: %+v", len(c.Verses), c.Verses)
	}
	// second item is the title "Greeting" with verse == lastVerse+1 == 2
	title := c.Verses[1]
	if title.Type != "title" || title.Content != "Greeting" || title.Number != 2 {
		t.Errorf("title item wrong: %+v", title)
	}
}

func TestScrapeTranslationsAndBooks(t *testing.T) {
	s := New("https://alkitab.mobi")
	trans := s.Translations()
	if len(trans) == 0 {
		t.Fatal("want at least one static translation (tb)")
	}
	books, err := s.Books("tb")
	if err != nil {
		t.Fatalf("Books: %v", err)
	}
	if len(books) != 66 {
		t.Errorf("want 66 canonical books, got %d", len(books))
	}
}

func TestScrapeUnknownVersion(t *testing.T) {
	s := New("https://alkitab.mobi")
	if _, err := s.Books("nonsense"); err != bible.ErrUnsupportedVersion {
		t.Errorf("want ErrUnsupportedVersion, got %v", err)
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/scrape/ -v`
Expected: FAIL — `undefined: New`.

- [ ] **Step 5: Write minimal implementation**

`internal/scrape/scrape.go`:
```go
package scrape

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/davidgrldo/alkitab-api/internal/bible"
)

// staticVersions: the scrape adapter cannot enumerate alkitab.mobi's catalog
// without scraping, so it advertises a small known set.
var staticVersions = []bible.Translation{
	{ID: "tb", Name: "Terjemahan Baru", Language: "id"},
}

// Scrape is a bible.Source that fetches chapters from alkitab.mobi.
// It does NOT implement Corpus (per-query scrape search is infeasible).
type Scrape struct {
	base    string
	client  *http.Client
}

func New(baseURL string) *Scrape {
	if baseURL == "" {
		baseURL = "https://alkitab.mobi"
	}
	return &Scrape{base: strings.TrimRight(baseURL, "/"), client: http.DefaultClient}
}

func (s *Scrape) Translations() []bible.Translation { return staticVersions }

func (s *Scrape) Books(version string) ([]bible.Book, error) {
	if !versionSupported(version) {
		return nil, bible.ErrUnsupportedVersion
	}
	return bible.CanonicalBooks(), nil
}

func (s *Scrape) Chapter(version, book string, chapter int) (*bible.Chapter, error) {
	if !versionSupported(version) {
		return nil, bible.ErrUnsupportedVersion
	}
	bookName := bible.IndonesianBookName(book)
	if bookName == "" {
		return nil, bible.ErrNotFound
	}
	url := strings.Join([]string{s.base, version, bookName, strconv.Itoa(chapter)}, "/")
	res, err := s.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, bible.ErrNotFound
	}
	if res.StatusCode != http.StatusOK {
		return nil, errors.New("scrape: upstream status " + res.Status)
	}
	verses, err := parse(res.Body)
	if err != nil {
		return nil, err
	}
	if len(verses) == 0 {
		return nil, bible.ErrNotFound
	}
	return &bible.Chapter{Translation: version, Book: book, Number: chapter, Verses: verses}, nil
}

func parse(r io.Reader) ([]bible.Verse, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}
	var items []bible.Verse
	lastVerse := 0
	doc.Find("p").Each(func(i int, p *goquery.Selection) {
		if _, hidden := p.Attr("hidden"); hidden {
			return
		}
		if p.HasClass("loading") || p.HasClass("error") {
			return
		}
		content := strings.TrimSpace(p.Find("[data-begin]").First().Text())
		title := strings.TrimSpace(p.Find(".paragraphtitle").First().Text())
		verseText := strings.TrimSpace(p.Find(".reftext").Children().First().Text())
		verse := 0
		if verseText != "" {
			verse, _ = strconv.Atoi(verseText)
		}
		if title == "" && content == "" {
			p.Find(".reftext").Remove()
			content = strings.TrimSpace(p.Text())
		}
		var typ string
		switch {
		case title != "":
			typ = "title"
			content = title
			verse = lastVerse + 1
		case content != "":
			typ = "content"
			lastVerse = verse
		}
		if typ != "" {
			items = append(items, bible.Verse{Number: verse, Content: content, Type: typ, Order: i})
		}
	})
	return items, nil
}

func versionSupported(v string) bool {
	for _, t := range staticVersions {
		if t.ID == v {
			return true
		}
	}
	return false
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/scrape/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/scrape/
git commit -m "feat(scrape): add alkitab.mobi scrape adapter via goquery"
```

---

## Task 10: HTTP server — passage endpoints + error mapping

**Files:**
- Create: `internal/server/server.go`
- Create: `internal/server/server_test.go`

**Interfaces:**
- Consumes: `bible.Engine` (Tasks 3–5), `bible.ResolveBookID`, `bible.Book`, `bible.Chapter`, typed errors
- Produces: `func New(e *bible.Engine) *Server`; `func (s *Server) Handler() http.Handler` exposing `GET /v1/translations`, `GET /v1/{version}/books`, `GET /v1/{version}/{book}/{chapter}`, `GET /v1/{version}/{book}/{chapter}/{verse}`.

- [ ] **Step 1: Write the failing test**

`internal/server/server_test.go`:
```go
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/davidgrldo/alkitab-api/internal/bible"
	"github.com/davidgrldo/alkitab-api/internal/local"
)

func newServer(t *testing.T) *Server {
	t.Helper()
	l, err := local.New("")
	if err != nil {
		t.Fatal(err)
	}
	return New(bible.New(l))
}

func getJSON(t *testing.T, h http.Handler, path string, code int) map[string]any {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	h.ServeHTTP(rr, req)
	if rr.Code != code {
		t.Fatalf("GET %s: status %d, want %d; body=%s", path, rr.Code, code, rr.Body.String())
	}
	var m map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &m)
	return m
}

func TestTranslations(t *testing.T) {
	h := newServer(t).Handler()
	m := getJSON(t, h, "/v1/translations", 200)
	if _, ok := m["translations"]; !ok {
		t.Errorf("missing translations key: %v", m)
	}
}

func TestBooks(t *testing.T) {
	h := newServer(t).Handler()
	m := getJSON(t, h, "/v1/kjv/books", 200)
	books, _ := m["books"].([]any)
	if len(books) != 2 {
		t.Errorf("want 2 sample books, got %v", books)
	}
}

func TestChapterByName(t *testing.T) {
	h := newServer(t).Handler()
	m := getJSON(t, h, "/v1/kjv/3John/1", 200)
	verses, _ := m["verses"].([]any)
	if len(verses) != 14 {
		t.Errorf("want 14 verses, got %d", len(verses))
	}
}

func TestChapterNotFound(t *testing.T) {
	h := newServer(t).Handler()
	getJSON(t, h, "/v1/kjv/3john/9", 404)
}

func TestBadChapter(t *testing.T) {
	h := newServer(t).Handler()
	getJSON(t, h, "/v1/kjv/3john/abc", 400)
}

func TestSingleVerse(t *testing.T) {
	h := newServer(t).Handler()
	m := getJSON(t, h, "/v1/kjv/3john/1/4", 200)
	verses, _ := m["verses"].([]any)
	if len(verses) != 1 {
		t.Errorf("want 1 verse filtered, got %d", len(verses))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -v`
Expected: FAIL — `undefined: New`.

- [ ] **Step 3: Write minimal implementation**

`internal/server/server.go`:
```go
package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/davidgrldo/alkitab-api/internal/bible"
)

type Server struct {
	eng *bible.Engine
}

func New(e *bible.Engine) *Server { return &Server{eng: e} }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/translations", s.translations)
	mux.HandleFunc("GET /v1/{version}/books", s.books)
	mux.HandleFunc("GET /v1/{version}/{book}/{chapter}/{verse}", s.chapterVerse)
	mux.HandleFunc("GET /v1/{version}/{book}/{chapter}", s.chapter)
	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// httpError is a sentinel for HTTP-level errors (e.g. 400) that mapErr translates.
type httpError struct {
	status int
	msg    string
}

func (e *httpError) Error() string { return e.msg }

func badRequest(msg string) error { return &httpError{status: http.StatusBadRequest, msg: msg} }

func (s *Server) mapErr(w http.ResponseWriter, err error) {
	var he *httpError
	if errors.As(err, &he) {
		writeError(w, he.status, he.msg)
		return
	}
	switch {
	case errors.Is(err, bible.ErrNotFound), errors.Is(err, bible.ErrUnsupportedVersion):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, bible.ErrUnsupportedFeature):
		writeError(w, http.StatusNotImplemented, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func (s *Server) translations(w http.ResponseWriter, r *http.Request) {
	// The engine exposes Source via Chapter; list from the engine's source.
	writeJSON(w, map[string]any{"translations": s.eng.Source().Translations()})
}

func (s *Server) books(w http.ResponseWriter, r *http.Request) {
	version := r.PathValue("version")
	b, err := s.eng.Source().Books(version)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"books": b})
}

func (s *Server) chapter(w http.ResponseWriter, r *http.Request) {
	c, err := s.resolveChapter(r)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	writeJSON(w, c)
}

func (s *Server) chapterVerse(w http.ResponseWriter, r *http.Request) {
	c, err := s.resolveChapter(r)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	vn, _ := strconv.Atoi(r.PathValue("verse"))
	var filtered []bible.Verse
	for _, v := range c.Verses {
		if v.Number == vn {
			filtered = append(filtered, v)
		}
	}
	writeJSON(w, map[string]any{
		"version": c.Translation, "book": c.Book, "chapter": c.Number, "verses": filtered,
	})
}

func (s *Server) resolveChapter(r *http.Request) (*bible.Chapter, error) {
	version := r.PathValue("version")
	chap, err := strconv.Atoi(r.PathValue("chapter"))
	if err != nil {
		return nil, badRequest("invalid chapter")
	}
	bookID, err := bible.ResolveBookID(r.PathValue("book"))
	if err != nil {
		return nil, err
	}
	return s.eng.Chapter(version, bookID, chap)
}
```

The Engine needs an exported accessor for its source. Add to `engine.go` (Task 3 file):
```go
// Source returns the underlying source (used by the server for listings).
func (e *Engine) Source() Source { return e.src }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/server/ internal/bible/engine.go
git commit -m "feat(server): add REST passage endpoints with error mapping"
```

---

## Task 11: HTTP server — search / daily / random

**Files:**
- Modify: `internal/server/server.go` (add routes + handlers)
- Modify: `internal/server/server_test.go` (add tests)

**Interfaces:**
- Consumes: `bible.Engine.Search`, `.DailyVerse`, `.RandomVerse`
- Produces: routes `GET /v1/search`, `GET /v1/daily`, `GET /v1/random`.

- [ ] **Step 1: Write the failing test**

Append to `internal/server/server_test.go`:
```go
func TestSearch(t *testing.T) {
	h := newServer(t).Handler()
	m := getJSON(t, h, "/v1/search?q=truth&version=kjv", 200)
	hits, _ := m["hits"].([]any)
	if len(hits) == 0 {
		t.Error("want at least one hit for 'truth'")
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	h := newServer(t).Handler()
	getJSON(t, h, "/v1/search?q=&version=kjv", 400)
}

func TestDailyAndRandom(t *testing.T) {
	h := newServer(t).Handler()
	getJSON(t, h, "/v1/daily?version=kjv", 200)
	getJSON(t, h, "/v1/random?version=kjv", 200)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run "TestSearch|TestDaily" -v`
Expected: FAIL — 404 (routes not registered).

- [ ] **Step 3: Write minimal implementation**

In `Handler()`, add:
```go
mux.HandleFunc("GET /v1/search", s.search)
mux.HandleFunc("GET /v1/daily", s.daily)
mux.HandleFunc("GET /v1/random", s.random)
```
Add handlers:
```go
func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "missing query parameter 'q'")
		return
	}
	version := r.URL.Query().Get("version")
	hits, err := s.eng.Search(version, q)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"hits": hits})
}

func (s *Server) daily(w http.ResponseWriter, r *http.Request) {
	version := r.URL.Query().Get("version")
	h, err := s.eng.DailyVerse(version, time.Now())
	if err != nil {
		s.mapErr(w, err)
		return
	}
	writeJSON(w, h)
}

func (s *Server) random(w http.ResponseWriter, r *http.Request) {
	version := r.URL.Query().Get("version")
	h, err := s.eng.RandomVerse(version)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	writeJSON(w, h)
}
```
Add `"time"` to imports.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -v`
Expected: PASS (all server tests).

- [ ] **Step 5: Commit**

```bash
git add internal/server/server.go internal/server/server_test.go
git commit -m "feat(server): add search, daily, and random endpoints"
```

---

## Task 12: Entrypoint + env wiring

**Files:**
- Create: `cmd/alkitab-api/main.go`

**Interfaces:**
- Consumes: `local.New`, `scrape.New`, `bible.New`, `bible.NewChain`, `server.New`
- Produces: a runnable binary that reads env (`ALKITAB_PORT`, `ALKITAB_DATA_DIR`, `ALKITAB_SCRAPE`, `ALKITAB_BASE_URL`) and serves HTTP.

- [ ] **Step 1: Write the entrypoint**

`cmd/alkitab-api/main.go`:
```go
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/davidgrldo/alkitab-api/internal/bible"
	"github.com/davidgrldo/alkitab-api/internal/local"
	"github.com/davidgrldo/alkitab-api/internal/scrape"
	"github.com/davidgrldo/alkitab-api/internal/server"
)

func main() {
	port := getenv("ALKITAB_PORT", "3000")

	loc, err := local.New(os.Getenv("ALKITAB_DATA_DIR"))
	if err != nil {
		log.Fatalf("local: %v", err)
	}

	var src bible.Source = loc
	if os.Getenv("ALKITAB_SCRAPE") == "1" {
		sc := scrape.New(os.Getenv("ALKITAB_BASE_URL"))
		src = bible.NewChain(loc, sc)
	}

	srv := server.New(bible.New(src))
	log.Printf("alkitab-api listening on :%s (scrape=%v)", port, os.Getenv("ALKITAB_SCRAPE") == "1")
	log.Fatal(http.ListenAndServe(":"+port, srv.Handler()))
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
```

- [ ] **Step 2: Build the whole project**

Run: `go build ./...`
Expected: no output (success).

- [ ] **Step 3: Smoke-test end to end**

Run the server in one shell: `go run ./cmd/alkitab-api`
In another:
```bash
curl -s http://localhost:3000/v1/translations
curl -s http://localhost:3000/v1/kjv/books
curl -s http://localhost:3000/v1/kjv/3john/1
curl -s "http://localhost:3000/v1/search?q=truth&version=kjv"
curl -s http://localhost:3000/v1/daily?version=kjv
```
Expected: JSON responses; the chapter returns 14 verses; search returns ≥1 hit.

- [ ] **Step 4: Run the full test suite**

Run: `go test ./... && go vet ./...`
Expected: all PASS, no vet warnings.

- [ ] **Step 5: Commit**

```bash
git add cmd/alkitab-api/main.go
git commit -m "feat: add HTTP entrypoint with env-based source wiring"
```

---

## Task 13: README

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write the README**

`README.md`:
````markdown
# alkitab-api

A Go Bible API server with a data-source-agnostic engine. Ships a JSON-backed
`local` adapter (default) and an opt-in `scrape` adapter for `alkitab.mobi`.

Inspired by [sonnylazuardi/alkitab-api](https://github.com/sonnylazuardi/alkitab-api).

## Run

```bash
go run ./cmd/alkitab-api
# serves on :3000
```

The server ships with a small **public-domain** sample (KJV: 3 John, Philemon)
so it works out of the box. Add full translations via `ALKITAB_DATA_DIR`.

## Endpoints

| Method | Path | Description |
|---|---|---|
| GET | `/v1/translations` | List available translations |
| GET | `/v1/{version}/books` | Books in a translation |
| GET | `/v1/{version}/{book}/{chapter}` | Whole chapter |
| GET | `/v1/{version}/{book}/{chapter}/{verse}` | Single verse |
| GET | `/v1/search?q=&version=` | Full-text search (local corpus only) |
| GET | `/v1/daily?version=` | Deterministic daily verse |
| GET | `/v1/random?version=` | Random verse |

`{book}` accepts an id (`gen`), English name (`Genesis`), or Indonesian name
(`Kejadian`), case-insensitive.

## Configuration (env)

| Var | Default | Purpose |
|---|---|---|
| `ALKITAB_PORT` | `3000` | Listen port |
| `ALKITAB_DATA_DIR` | *(none)* | Extra translations directory |
| `ALKITAB_SCRAPE` | `0` | Enable the `scrape` adapter (`1`) |
| `ALKITAB_BASE_URL` | `https://alkitab.mobi` | Scrape base URL |

## Bring Your Own Data (BYOD)

Drop a JSON file per translation into `ALKITAB_DATA_DIR`. Schema:

```json
{
  "translation": {"id": "kjv", "name": "King James Version", "language": "en"},
  "books": [
    {"id": "3john", "name": "3 John", "abbr": "3John", "testament": "NT", "chapters": 1,
     "chapter_data": [{"number": 1, "verses": [{"verse": 1, "type": "content", "content": "..."}]}]}
  ]
}
```

A runtime file with the same `id` overrides the embedded sample.

## Copyright

This repository distributes **no copyrighted text**. The embedded sample is
public domain (KJV). Translations like Terjemahan Baru (TB) © LAI 1974 are
**not** included; access them at runtime by enabling `ALKITAB_SCRAPE=1` (a
proxy that stores only in-memory cache) or by supplying your own data file for
which you are responsible. Public-domain alternatives (KJV, BBE; 19th-century
Indonesian works like Klinkert 1863/1879 and Melayu Baba 1913) are safe BYOD
candidates.
````

- [ ] **Step 2: Verify the project still builds and tests pass**

Run: `go build ./... && go test ./...`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: add README with usage, BYOD format, and copyright note"
```

---

## Self-Review Notes

- **Spec coverage:** Passage lookup (Tasks 3, 7, 10), search (4, 8, 11), metadata (2, 7, 10), daily/random (5, 11), BYOD/local (7), scrape opt-in (9, 12), Chain fallback (6, 12), REST endpoints (10, 11), error mapping (10), config env (12), copyright/README (13). All spec sections mapped.
- **Refinement vs spec:** `Searcher` interface renamed to `Corpus` and search moved into the Engine (engine scans the adapter's corpus). Behavior identical (local supports search/daily/random; scrape does not → 501); only the interface shape changes, which planning surfaced as cleaner. Documented inline.
- **Type consistency:** `ResolveBookID`, `IndonesianBookName`, `CanonicalBooks`, `NewChain`, `Engine.Source()`, `Server.Handler()` names match across producer/consumer tasks.
