# Design: alkitab-api (Go)

- **Date:** 2026-07-17
- **Status:** Approved (brainstorm)
- **Module:** `github.com/davidgrldo/alkitab-api`
- **Inspiration:** `sonnylazuardi/alkitab-api` (Node.js, GraphQL, scrape `alkitab.mobi`)

## 1. Motivation

`sonnylazuardi/alkitab-api` is an Express + Apollo GraphQL server that scrapes
`http://alkitab.mobi/{version}/{book}/{chapter}` with `cheerio` and caches each
chapter in an in-memory `chapterMap`. Its core weakness is the runtime-only
dependency on a live third-party site: fragile (HTML changes break it), slow on
first hit, a single point of failure, and rate-limitable. The cache is also
volatile — lost on every restart.

This project rebuilds the idea in Go with two structural changes:

1. **No hard dependency on `alkitab.mobi`.** The engine is data-source-agnostic.
   A `local` adapter (JSON + `go:embed`, in-memory scan) is the default; a
   `scrape` adapter (port of the original logic) is opt-in.
2. **Bring Your Own Data (BYOD).** The repository ships **no copyrighted text**.
   A clear JSON format lets users supply their own translations. Default sample
   data is public domain only (KJV/BBE), so `go build && ./alkitab-api` works
   out of the box without legal exposure.

### Why this is feasible and an improvement

- A single Bible translation is ~31,105 verses, a few MB total. Brute-force
  substring scan over 31k short strings in Go is microsecond-to-low-millisecond
  territory, so **no database is required** for full-text search at this scale.
- Go produces a single static binary, uses far less memory than Node, and the
  stdlib `net/http` router (Go 1.22+ path patterns) covers all routing needs
  with zero dependencies.
- Copyright is handled structurally (BYOD + PD defaults) rather than hoped away.

## 2. Non-goals (YAGNI for v1)

GraphQL, audio, concordance / cross-references, user accounts / auth, rate
limiting, metrics, Dockerfile. Each can be layered on later without rewriting
the engine.

## 3. Key decisions

| Decision | Choice | Rationale |
|---|---|---|
| Copyright strategy | BYOD + pluggable adapters; ship PD only | Repo stays legally clean; users own their data |
| v1 features | Passage lookup, full-text search, book metadata, daily/random | Covers original + useful extras |
| Storage | JSON files + `go:embed` + in-memory scan | No DB; sufficient at Bible scale; clearest BYOD format |
| Search | In-memory substring scan | 31k verses is trivial to scan; no index needed |
| API style | REST on stdlib `net/http` (Go 1.22+ routing) | Zero deps; idiomatic; GraphQL addable later as a thin `gqlgen` layer over the same engine |
| Scrape adapter | Opt-in, default off, behind env var | Preserves TB access path without forcing the dependency |

## 4. Architecture

Three layers, dependencies pointing inward only:

```
transport (internal/server)  ──▶  engine (internal/bible)  ◀──  adapters (internal/local, internal/scrape)
   net/http                       pure domain types             Source / Searcher impls
```

The domain layer has no knowledge of JSON, HTTP, or scraping. Adapters
implement the `Source` contract. The REST handlers are a thin shell over the
engine. This separation is what makes BYOD and opt-in scrape possible without
rewrites.

### Project layout

```
cmd/alkitab-api/main.go          # entrypoint: read env, wire engine, start HTTP
internal/
  bible/                          # DOMAIN (pure Go, no I/O deps)
    types.go                      # Verse, Chapter, Book, Translation, VerseHit, errors
    source.go                     # Source + Searcher interfaces (adapter contract)
    engine.go                     # Engine: Chapter()/Search()/Daily()/Random() + cache
    metadata.go                   # canonical 66-book metadata (ID/EN names, chapter counts, OT/NT)
    engine_test.go
  local/                          # 'local' adapter: JSON + go:embed + in-memory
    local.go                      # implements Source + Searcher
    local_test.go
    data/                         # PD sample translations + registry
      registry.json
      kjv.json
      bbe.json
  scrape/                         # 'scrape' adapter: alkitab.mobi (OPT-IN, default off)
    scrape.go                     # implements Source only
    scrape_test.go
  server/                         # REST transport
    server.go                     # Go 1.22+ router + handlers
    server_test.go
go.mod
README.md
```

## 5. Component contracts

### 5.1 Domain types (`internal/bible/types.go`)

```go
package bible

type Verse struct {
    Number  int    `json:"verse"`
    Content string `json:"content"`
    Type    string `json:"type"`  // "content" | "title"
    Order   int    `json:"order"` // position within chapter (titles included)
}

type Chapter struct {
    Translation string  `json:"version"`
    Book        string  `json:"book"`
    Number      int     `json:"chapter"`
    Verses      []Verse `json:"verses"`
}

type Book struct {
    ID           string `json:"id"`        // e.g. "gen"
    Name         string `json:"name"`      // e.g. "Kejadian"
    Abbreviation string `json:"abbr"`      // e.g. "Kej"
    Testament    string `json:"testament"` // "OT" | "NT"
    Chapters     int    `json:"chapters"`
}

type Translation struct {
    ID       string `json:"id"`       // e.g. "kjv", "tb"
    Name     string `json:"name"`     // e.g. "King James Version"
    Language string `json:"language"` // ISO 639-1, e.g. "en", "id"
}

type VerseHit struct {
    Translation string `json:"version"`
    Book        string `json:"book"`
    Chapter     int    `json:"chapter"`
    Verse       Verse  `json:"verse"`
}

// Typed errors.
var (
    ErrNotFound            = errors.New("bible: not found")
    ErrUnsupportedVersion  = errors.New("bible: unsupported version")
    ErrUnsupportedFeature  = errors.New("bible: feature not supported by active source")
)
```

### 5.2 Adapter contract (`internal/bible/source.go`)

```go
package bible

// Source is the mandatory contract every adapter implements.
type Source interface {
    Translations() []Translation
    Books(version string) ([]Book, error)
    Chapter(version, book string, chapter int) (*Chapter, error)
}

// Searcher is an optional capability. Adapters with an in-memory corpus
// (local) implement it; network-only adapters (scrape) do not.
type Searcher interface {
    Search(version, query string) ([]VerseHit, error)
}
```

The Engine probes for `Searcher` via type assertion. If absent, the
`/search`, `/daily`, and `/random` endpoints return HTTP 501 with a clear
message. This is the idiomatic Go optional-capability pattern (cf.
`io.ReadWriter`) and keeps the three corpus-dependent features honest.

### 5.3 Engine (`internal/bible/engine.go`)

```go
package bible

type Engine struct {
    src    Source
    mu     sync.RWMutex
    cache  map[string]*Chapter // key: version:book:chapter
}

func New(src Source) *Engine { ... }

// Chapter delegates to the source and caches the result.
func (e *Engine) Chapter(version, book string, chapter int) (*Chapter, error)

// Search returns hits whose verse content contains query (case-insensitive).
// Requires the source to implement Searcher; otherwise ErrUnsupportedFeature.
func (e *Engine) Search(version, query string) ([]VerseHit, error)

// DailyVerse returns a deterministic reference for the given date, so all
// requests on the same day agree. Requires Searcher-backed corpus.
func (e *Engine) DailyVerse(version string, t time.Time) (*VerseHit, error)

// RandomVerse returns an unpredictable reference. Requires Searcher-backed corpus.
func (e *Engine) RandomVerse(version string) (*VerseHit, error)
```

The engine is the only thing transport code touches; adapters are never used
directly by handlers.

`DailyVerse` determinism: the seed is `hash(YYYYMMDD, version) % len(corpus)`,
so every request on the same calendar day (UTC) for the same version returns
the same reference, and different days generally differ.

### 5.4 Chain (composite source) (`internal/bible/engine.go`)

The Engine wraps a single `Source`. When the scrape adapter is enabled, the
wiring composes a `Chain` that satisfies `Source` and delegates in order
(local first, scrape on miss):

```go
type Chain struct{ sources []Source }
```

- `Chapter` returns the first non-`ErrNotFound` result; `Books`/`Translations`
  merge across members.
- `Chain` implements `Searcher` iff at least one member does, by delegating to
  the first member that does (i.e. `local`). Scrape results are therefore never
  searched, which matches its lack of a feasible search path.

This resolves the single-`Source` Engine against the local→scrape fallback
described in §8 without complicating the Engine itself.

### 5.5 `local` adapter (`internal/local/`)

- Loads every `data/*.json` via `//go:embed` at compile time (PD defaults: KJV and BBE — both unambiguously public domain and readily available as clean JSON).
- Also reads an optional runtime directory (`ALKITAB_DATA_DIR`) so a user can
  drop in their own translations (e.g. TB) without rebuilding. Both sets are
  merged; a runtime entry with the same `id` overrides the embedded one.
- Holds the full corpus in memory once at startup.
- Implements `Source` (lookup, books, translations) and `Searcher`
  (case-insensitive substring scan across the loaded corpus).

> Note on 19th-century Indonesian translations (Klinkert 1863/1879, Melayu Baba
> 1913): these are public-domain *options* a user may supply via
> `ALKITAB_DATA_DIR`. They are **not** bundled in v1 because sourcing clean
> JSON for them is out of scope; v1 bundles only KJV and BBE. The README will
> list them as legal BYOD candidates.

### 5.6 `scrape` adapter (`internal/scrape/`)

- `net/http` GET `{base}/{version}/{book}/{chapter}` (default base
  `https://alkitab.mobi`, overridable).
- Parses HTML with `github.com/PuerkitoBio/goquery` (one justified dependency;
  the closest Go analog to `cheerio`). Extracts verses from `p[data-begin]`,
  `.paragraphtitle`, `.reftext`, mirroring the original selectors and field
  semantics (titles get `type:"title"` and `verse == lastVerse+1`).
- Implements `Source` only — not `Searcher` (per-query scrape search is not
  feasible). Results are cached in the Engine's chapter cache like any source.

## 6. Data format (BYOD JSON)

One file per translation. Self-describing so a user can author one without
reading the source:

```json
{
  "translation": {
    "id": "kjv",
    "name": "King James Version",
    "language": "en"
  },
  "books": [
    {
      "id": "gen",
      "name": "Genesis",
      "abbr": "Gen",
      "testament": "OT",
      "chapters": 50,
      "chapter_data": [
        {
          "number": 1,
          "verses": [
            { "verse": 1, "type": "content", "content": "In the beginning God created the heaven and the earth." }
          ]
        }
      ]
    }
  ]
}
```

`data/registry.json` enumerates the embedded translations for quick listing;
runtime-directory translations are discovered by scanning `*.json` in the dir.

## 7. REST API

All routes use Go 1.22+ method+path patterns. JSON in, JSON out. Errors use
`{"error":"<message>"}` with an appropriate status code.

```
GET /v1/translations                            -> { "translations": [...] }
GET /v1/{version}/books                         -> { "books": [...] }
GET /v1/{version}/{book}/{chapter}              -> Chapter (all verses)
GET /v1/{version}/{book}/{chapter}/{verse}      -> Chapter filtered to one verse
GET /v1/search?q={query}&version={version}      -> { "hits": [VerseHit, ...] }
GET /v1/daily?version={version}                 -> VerseHit (deterministic by date)
GET /v1/random?version={version}                -> VerseHit
```

`{book}` accepts the book's `id` (`gen`), `name` (`Kejadian`/`Genesis`), or
`abbr` (`Kej`), matched case-insensitively against the canonical metadata
table in `internal/bible/metadata.go`. The `scrape` adapter translates to
whatever form its target URL expects (alkitab.mobi uses names). `{verse}` must
parse as a positive integer.

Status code mapping:

| Condition | Status |
|---|---|
| Unknown version | 404 (`ErrUnsupportedVersion`) |
| Book/chapter/verse not found | 404 (`ErrNotFound`) |
| Feature unsupported by active source | 501 (`ErrUnsupportedFeature`) |
| Malformed input | 400 |
| Empty search query | 400 |

## 8. Configuration

Environment variables only (no config file, no flag framework — stdlib
`os.Getenv`). All optional:

| Var | Default | Purpose |
|---|---|---|
| `ALKITAB_PORT` | `3000` | HTTP listen port |
| `ALKITAB_DATA_DIR` | *(none)* | Extra runtime translations directory |
| `ALKITAB_SCRAPE` | `0` | Enable the scrape adapter when set to `1` |
| `ALKITAB_BASE_URL` | `https://alkitab.mobi` | Base URL for the scrape adapter |

When `ALKITAB_SCRAPE=1` the scrape adapter is composed under the local adapter
(fallback chain: local first, scrape on miss); when disabled, only local runs.

## 9. Error handling

- All adapters return the typed errors defined in `types.go`; they never leak
  raw `net/http` or scrape errors to the client.
- The HTTP layer is the single place errors are mapped to status codes, so the
  mapping is consistent and testable.
- Search/daily/random on a non-`Searcher` source returns 501 with a message
  naming the missing capability (no silent empty results).

## 10. Testing

`testing` stdlib only — no external framework.

- **`bible`**: table-driven `Search` (substring, case-insensitivity, no-match);
  `DailyVerse` determinism (same date → same ref, different date → likely
  different); `RandomVerse` stays in-bounds across many draws.
- **`local`**: load embedded sample; assert non-zero verse count; spot-check a
  known verse; assert `Books()` matches the 66-book canon.
- **`scrape`**: `httptest.Server` serves a saved HTML fixture (no network);
  assert parsed verses match expected content, types, and order.
- **`server`**: `httptest.NewRecorder` per endpoint; assert JSON shape and
  status codes, including the 404/501/400 paths.

## 11. Legal note (intentional)

The repository must never commit copyrighted translation text (notably
Terjemahan Baru © LAI 1974 and other LAI / WBTC translations). The default
embedded data is restricted to public-domain works — **KJV and BBE** are
bundled in v1; 19th-century Indonesian translations (Klinkert 1863/1879, Melayu
Baba 1913) are documented as legal BYOD candidates a user may add but are not
bundled. The `scrape` adapter is a runtime proxy the operator opts into; it
stores only an in-memory cache, never redistributed source text. This mirrors
how the original project avoided redistribution and is documented in the README.

## 12. Open follow-ups (post-v1)

- GraphQL layer via `gqlgen` over the same engine.
- Concordance / cross-references as a new optional adapter.
- Audio (clearly a separate source type).
- Optional `Dockerfile` and rate limiting once there is a deployment that needs them.
