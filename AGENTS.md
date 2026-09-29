# AGENTS.md — working conventions for Loom (`rebuild/columns-ux`)

## Commands

```sh
go run ./cmd/loom --addr :8080 --data loom-data.json --web web  # local dev (JSON file)
go run ./cmd/loom --addr :8080 --data /data/loom.db --web web   # local dev (SQLite)
gofmt -l .                                                       # must print nothing
go vet ./...                                                     # must be clean
go build ./... && go test ./...                                  # must be green
sh scripts/budget-check.sh                                       # initial JS ≤50KB gzip, fails CI over
```

Run the full line (`gofmt` + `go vet` + `go build` + `go test` +
`budget-check`) before pushing. Prefer the smallest relevant test
(`go test ./internal/store/…`) during iteration, full suite at the end.

Rebuild branch: single Go binary (`cmd/loom`) + vanilla JS (`web/`).

- Edit `web/app.js`, not build output (no bundler; budget gate `sh scripts/budget-check.sh`).
- Backend: `internal/model`, `internal/store` (`Store` interface; SQLite/file), `internal/api`.

## Standing rule

Every feature change updates `FEATURES.md` in the same commit (append/modify one bullet, keep it terse).

## Branch / PR / preview conventions

- Active branch: `rebuild/columns-ux`. Base new work on latest
  `origin` head of that branch.
- PR: [crueber/loom#3](https://github.com/crueber/loom/pull/3).
  **Do not merge it** — review belongs to [OSS-12](/OSS/issues/OSS-12).
- Docs-only rounds (like this one) must not change behavior:
  `go test` stays green and `scripts/budget-check.sh` stays passing.
- Preview: run the branch container/image (`docker build -t loom .`
  per `Dockerfile`) against a scratch `/data` volume — never the
  production database path.

## Hard constraints

- **JS budget**: `web/app.js` ≤50KB gzipped (`scripts/budget-check.sh`
  enforces). Editor/dnd/upload extras stay out of the initial bundle.
- **First paint**: cache/seed → paint target <100ms, instrumented as
  `window.__LOOM_FIRST_PAINT_MS`. Never quote a stale ms figure —
  re-measure and say how.
- **Single binary**: stdlib HTTP + embedded/static web assets only;
  cgo SQLite (`mattn/go-sqlite3`) is the only native dependency.
- **Clean tree**: `gofmt`/`go vet`/`go test` clean on every push.
- **No-merge rule**: never merge `rebuild/columns-ux` (PR #3) —
  that decision belongs to the OSS-12 review.

## Claim verification

Every factual claim in `README.md` (flags, routes, store modes,
cache keys, image limits, auth status) must be verified against the
tree before pushing: flags in `cmd/loom/main.go`, routes in
`internal/api/api.go`, store in `internal/store/`, keys and
bootstrap/paint logic in `web/app.js`. Post per-file accuracy notes
on the task when done.
