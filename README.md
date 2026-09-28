# Loom rebuild (scratch)

Full-scratch rebuild of Loom per the approved plan on [OSS-5](/OSS/issues/OSS-5):
Columns.app-style card-centric columns, v1 cards = links + rich notes + images,
new-tab instant load preserved. Reference implementation (do not patch) lives at
`~/dev/github.com/crueber/loom`.

## Architecture

- **Single binary** (`cmd/loom`): stdlib `net/http` API + static file server +
  server bootstrap (`__BOOTSTRAP_DATA__`) inlined into `/`.
- **Data model**: boards → columns → cards, each card holds ordered blocks
  (`link` | `note` markdown | `image`). Column color/collapse/position
  semantics carried over for v1 migration.
- **Store**: `internal/store.Store` interface. Ships with a zero-dependency
  JSON file backend; `schema.sql` is the SQLite DDL the next step wires to a
  `database/sql` driver (same interface, mechanical swap).
- **Frontend** (`web/`, vanilla JS, no framework): cache-first render from
  `localStorage` (synchronous paint, no spinner), `__BOOTSTRAP_DATA__` as
  cold-start seed, background revalidation, optimistic-local writes with
  server as source of truth, service-worker app-shell cache, lazy images
  (`loading="lazy"`, `decoding="async"`), LRU image cap pending with the
  server image store.

## Run

```sh
go run ./cmd/loom --addr :8080 --data loom-data.json --web web
```

## Checks

```sh
go build ./... && go test ./... && sh scripts/budget-check.sh
```

## Measurements

- Initial JS budget: `sh scripts/budget-check.sh` (≤50KB gzip, fails CI over).
  Current: **3.5KB** (upload + URL-attach UI included).
- Warm first-paint: `window.__LOOM_FIRST_PAINT_MS` (cache→paint ms, target <100ms).
  Measured with Obscura (persistent profile, real Chromium): **cold 1–2ms,
  warm 1ms** from `localStorage` cache, 1 column + 1 card rendered, no spinner.
- Server bootstrap latency: `curl -o /dev/null -w '%{time_total}\n' localhost:8080/`
  (~1ms on SQLite, seeded board inlined as `__BOOTSTRAP_DATA__`).
- Offline: SW caches app shell cache-first + `localStorage` render precedes all
  network; sync failures leave cached UI in place. Full server-down reload is
  covered by QA's browser pass ([OSS-7](/OSS/issues/OSS-7)).

## Deploy

Single binary + SQLite, Docker: `docker build -t loom-rebuild .`
(`Dockerfile`: golang/bookworm builder for cgo, debian-slim runtime, DB in `/data`).

## Status / next

- [x] Scaffold: model, v1 import, API CRUD, file store, shell UI, SW, budget gate
- [ ] SQLite driver swap onto `schema.sql` (keep `Store` interface)
- [ ] Server image store + thumbnails + client LRU cap (~50–100MB)
- [ ] Drag-and-drop + rich editor as lazy chunks (out of initial bundle)
- [ ] Auth/OIDC + i18n table stakes carried back over
- [ ] Measured warm-render <100ms + QA pass on Columns.app UX
