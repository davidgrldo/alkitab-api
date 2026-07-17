# alkitab-api

> **One engine, many sources.** A Bible API in Go that treats copyright as an
> architecture problem — not an afterthought.

Most Bible translations, including the Indonesian *Terjemahan Baru*, are
copyrighted. This project responds by design instead of by ignoring it: the
engine ships only **public-domain text**, reads **your own data** at runtime,
and keeps access to copyrighted translations strictly **opt-in** — a live
proxy, never a redistribution.

One binary. No database. Zero copyrighted text in the repo.

```
HTTP ──▶ server ──▶ engine ──▶ Source
         net/http   cache ·    ├── local   default · embedded JSON + BYOD corpus
         REST       Chain      └── scrape  opt-in · runtime proxy, no redistribution

         transport ──▶ core ──▶ adapters — dependencies point inward
```

- **`local`** — JSON via `go:embed` plus anything you drop in a data
  directory. Carries the in-memory corpus that powers search/daily/random.
- **`scrape`** — a runtime proxy for `alkitab.mobi` (goquery, 10 s timeout,
  4 MiB body cap). No corpus: search/daily/random honestly return `501`.
- **`Chain`** — tries `local` first, falls through to `scrape` when a version
  isn't found. Capability is checked by type assertion, not by hoping.

## Quick start

```bash
go run ./cmd/alkitab-api
# alkitab-api listening on :3000 (scrape=false)

curl localhost:3000/v1/kjv/3john/1/4
```

```json
{
  "version": "kjv",
  "book": "3john",
  "chapter": 1,
  "verses": [
    {
      "verse": 4,
      "content": "I have no greater joy than to hear that my children walk in truth.",
      "type": "content",
      "order": 0
    }
  ]
}
```

Works out of the box: a small **public-domain KJV sample** (3 John, Philemon)
is embedded. Full translations are yours to add — see [BYOD](#bring-your-own-data-byod).

## Endpoints

| Method | Path | Description |
|---|---|---|
| GET | `/v1/translations` | List available translations |
| GET | `/v1/{version}/books` | Books, chapter counts, OT/NT category |
| GET | `/v1/{version}/{book}/{chapter}` | Whole chapter, section headings included |
| GET | `/v1/{version}/{book}/{chapter}/{verse}` | Single verse |
| GET | `/v1/search?q=&version=` | Case-insensitive substring search (local corpus) |
| GET | `/v1/daily?version=` | Deterministic per date & version (UTC) |
| GET | `/v1/random?version=` | Random verse, seeded by `math/rand/v2` |

`{book}` accepts an id (`gen`), an English name (`Genesis`), or an Indonesian
name (`Kejadian`) — case-insensitive. Typed errors map to honest status codes:
`404` unknown, `400` malformed, `501` unsupported capability. Internal
messages never leak.

## Bring Your Own Data (BYOD)

Drop one JSON file per translation into `ALKITAB_DATA_DIR`:

```json
{
  "translation": { "id": "kjv", "name": "King James Version", "language": "en" },
  "books": [
    {
      "id": "3john", "name": "3 John", "abbr": "3John",
      "testament": "NT", "chapters": 1,
      "chapter_data": [
        { "number": 1, "verses": [
          { "verse": 1, "type": "content", "content": "..." }
        ] }
      ]
    }
  ]
}
```

A runtime file with the same `id` overrides the embedded sample. Safe
public-domain candidates: KJV, BBE, and 19th-century Indonesian works such as
Klinkert (1863/1879) and Melayu Baba (1913).

## Configuration

| Var | Default | Purpose |
|---|---|---|
| `ALKITAB_PORT` | `3000` | Listen port |
| `ALKITAB_DATA_DIR` | *(none)* | Extra translations directory |
| `ALKITAB_SCRAPE` | `0` | `1` enables the `scrape` adapter |
| `ALKITAB_BASE_URL` | `https://alkitab.mobi` | Scrape base URL |

## Copyright

This repository distributes **no copyrighted text**. The embedded sample is
public domain (KJV: 3 John, Philemon). Translations like *Terjemahan Baru*
(TB) © LAI 1974 are **not** included; access them at runtime by enabling
`ALKITAB_SCRAPE=1` (a proxy that keeps only an in-memory cache) or by
supplying your own data file, for which you are responsible.

## Microsite

A single-file, framework-free microsite that tells the whole story visually
lives at [`site/index.html`](site/index.html) — open it in a browser, run the
server, and the live demo button fetches a verse for real.
