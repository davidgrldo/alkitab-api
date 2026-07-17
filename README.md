# alkitab-api

A Go Bible API server with a data-source-agnostic engine. Ships a JSON-backed
`local` adapter (default) and an opt-in `scrape` adapter for `alkitab.mobi`.

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
