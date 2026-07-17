# Contributing

Thanks for considering a contribution. Ground rules — there are only three:

1. **Never commit copyrighted Bible text.** The repo ships public-domain text
   only (embedded sample and release assets). Copyrighted translations reach
   users via BYOD or the opt-in scrape proxy — that boundary is the project's
   whole architecture. PRs that embed copyrighted text will be closed.
2. **Keep dependencies near zero.** The core has none; the scrape adapter has
   goquery. A new dependency needs a reason a few lines of stdlib can't answer.
3. **Tests ride along.** `go vet ./... && go test ./...` must pass; new
   behavior brings the smallest test that would fail without it.

## Dev loop

```bash
go run ./cmd/alkitab-api          # serve on :3000
go test ./...                     # 5 packages, no external services needed
```

The scrape adapter's tests run against recorded HTML in `scrape/testdata/` —
no network. If you change the parser, refresh the fixture and say so in the PR.
