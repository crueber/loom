# Loom (rebuild branch)

Single-binary personal boards app: boards → columns → cards, each card
holding ordered blocks. Vanilla-JS frontend, stdlib `net/http` API,
SQLite persistence. This branch is `rebuild/columns-ux`
([PR #3](https://github.com/crueber/loom/pull/3), **do not merge** —
review lives in [OSS-12](/OSS/issues/OSS-12)).

## Run

```sh
# SQLite (default for production / Docker):
go run ./cmd/loom --addr :8080 --data /data/loom.db --web web

# JSON-file backend (local dev):
go run ./cmd/loom --addr :8080 --data loom-data.json --web web

# In-memory (no persistence, e.g. throwaway tests):
go run ./cmd/loom --addr :8080 --data "" --web web
```

Flags are defined in `cmd/loom/main.go`: `--addr` (default `:8080`),
`--data` (default `loom-data.json`; a `.db`/`.sqlite`/`.sqlite3` suffix
selects SQLite, any other path selects the JSON-file backend, empty
selects in-memory), `--web` (default `web`, the assets directory).

On first run with an empty store, the server seeds a `Home` board with
one `Start here` column and a welcome card, so a new tab is never empty.

## Architecture

- **Single binary** (`cmd/loom`): stdlib `net/http` JSON API + static
  file server + server bootstrap (`__BOOTSTRAP_DATA__`) inlined into
  `/` (see `cmd/loom/main.go`, `web/index.html`).
- **Data model** (`internal/model`): boards → columns → cards; each
  card holds ordered blocks of type `link` | `note` | `image`.
  `note` blocks are markdown, rendered client-side by a minimal
  renderer (`# H1`, `## H2`, `**bold**`, `*italic*`, `` `code` `` —
  see `web/app.js`). Columns carry title/color/position/collapsed.
- **Store** (`internal/store`): the `Store` interface has two
  backends. `SQLiteStore` (`sqlite.go`, cgo via `mattn/go-sqlite3`,
  WAL mode, DDL in `schema.sql`) is selected automatically when
  `--data` points at a `.db`/`.sqlite` file. `FileStore`
  (`store.go`) is a zero-dependency JSON-file backend; empty path
  runs it in-memory with no persistence.
- **v1 legacy importer**: `POST /api/import/v1` accepts a v1 export
  JSON payload (`internal/model/import_v1.go`) and stores it as a new
  board tree. It is **additive-only** — `ImportTree` appends the
  board/columns/cards and never modifies or deletes existing data.
  `GET /api/export` returns a v2 export (all boards + trees).
- **Images**: `POST /api/images` accepts one multipart `file` field
  (jpeg/png/gif, ≤12MB — `internal/images`, `MaxUploadBytes`),
  stores the original plus a JPEG thumbnail, and returns an image
  block payload to attach to a card. Served at `GET /images/{id}`
  and `GET /images/{id}/thumb` with immutable long-cache headers.
  Image upload/serving requires the SQLite backend; the JSON-file
  backend answers `501`.
- **Auth status**: optional OIDC login (Authorization Code + PKCE,
  stdlib-only: provider discovery + JWKS RS256 verify with
  `crypto/rsa`, no new Go deps). Configured entirely from the
  **Settings dialog in the UI** (issuer, client ID, secret, enable +
  require-login toggles) — no flags, env vars, or config files; the
  secret stays server-side and settings persist in the database
  (SQLite `auth_settings` row / JSON-file `auth` block). OFF by
  default = open single-user mode, no login. ON = new boards private
  by default (existing boards stay public), per-board public/private
  switch + viewer/editor invites via the **Share dialog**, server-side
  sessions in an HttpOnly SameSite cookie. Code: `internal/auth`
  (OIDC), `internal/api/auth.go` (routes + permission scoping),
  `internal/store` (`users`, `auth_settings`, `board_members`,
  `sessions` tables / JSON blocks, `boards.owner_id` + `visibility`).
- **Frontend** (`web/`, vanilla JS, no framework): cache-first render
  from `localStorage` (key `loom.cache.v2`, migrated from
  `loom.cache.v1`; column widths in `loom.colwidths.v1`),
  `__BOOTSTRAP_DATA__` as the cold-start seed (preferred only when
  no cache exists), background revalidation with the **server as
  source of truth**, optimistic-local writes, service-worker
  app-shell cache (`loom-shell-v1`: shell + images cache-first, API
  reads stale-while-revalidate), lazy images, HTML5 drag-and-drop
  for cards (within/across columns) and column reorder, inline
  editing.
- **Full route list** lives in `internal/api/api.go` (`Handler.Register`)
  plus `internal/api/auth.go`: `GET/POST /api/boards`, `GET/PATCH/DELETE /api/boards/{id}`,
  `POST /api/boards/{id}/columns`, `PATCH/DELETE /api/columns/{id}`,
  `POST /api/columns/{id}/cards`, `PATCH/DELETE /api/cards/{id}`,
  `POST /api/cards/{id}/move`, `POST /api/import/v1`,
  `GET /api/export`, `POST /api/images`, `GET /images/…`,
  `GET /api/auth/status`, `GET/PUT /api/auth/settings`,
  `GET /api/me`, `GET /api/auth/login`, `GET /api/auth/callback`,
  `POST /api/auth/logout`, `GET/POST /api/boards/{id}/members`,
  `PATCH/DELETE /api/boards/{id}/members/{user_id}`.

## Checks

```sh
gofmt -l . && go vet ./... && go build ./... && go test ./... && sh scripts/budget-check.sh
```

## Measurements

Re-measured on this branch (2026-09-29, `rebuild/columns-ux` @ `9f257f5`):

- Initial JS budget: `sh scripts/budget-check.sh` (≤50KB gzip, fails
  CI over). Current: **14.4KB** (`web/app.js`, 14740 bytes gzipped).
- Server bootstrap latency (local run, seeded SQLite, board inlined
  as `__BOOTSTRAP_DATA__`):
  `curl -o /dev/null -w '%{time_total}\n' localhost:PORT/` →
  **~2.7ms cold, ~1.2ms warm** on loopback.
- Client first-paint is instrumented as `window.__LOOM_FIRST_PAINT_MS`
  (script-start → cache/bootstrap paint, `web/app.js` boot section),
  target <100ms. Read it from a live tab (e.g. DevTools console or
  Obscura with a persistent profile) rather than trusting a quoted
  figure — no fresh browser-measured number was taken in this round.

## Deploy

Single binary + SQLite via Docker (`Dockerfile`: `golang:1.26-bookworm`
builder with `CGO_ENABLED=1` for `mattn/go-sqlite3`, `debian:bookworm-slim`
runtime — not scratch/alpine, since cgo needs gcc at build time and
glibc at runtime; SQLite DB lives in `/data`):

```sh
docker build -t loom .
docker run -p 8080:8080 -v loom-data:/data loom
```

The image entrypoint is
`["/loom", "--addr", ":8080", "--data", "/data/loom.db", "--web", "/web"]`.
