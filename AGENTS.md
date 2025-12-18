# AGENTS.md

This file provides guidance to agents when working with code in this repository.

## Non-Obvious Project Patterns

### Frontend (Alpine.js + esbuild)
- **Source vs Static**: Edit files in `cmd/server/src/`, NOT `cmd/server/static/dist/`. The latter are build artifacts.
- **Data Bootstrapping**: Initial data is injected via `window.__BOOTSTRAP_DATA__` in `cmd/server/app_handler.go`. Components must consume this to avoid API calls on load.
- **Event Registry**: All cross-component communication MUST use constants from `cmd/server/src/components/events.js` via the `dispatchEvent` facade in `cmd/server/src/utils/api.js`.
- **Card Flip UI**: 
  - Lists use 3D transforms (`rotateY(180deg)`).
  - Bookmarks/Notes use simple `display: none/block` toggles.
  - Only one card can be flipped at a time, managed by `Alpine.store('flipCard')`.
- **Temporary Items**: New items use string IDs (e.g., `temp-123`) and `data-is-temp="true"`. `closeFlippedCard()` removes these from DOM if cancelled.

### Backend (Go + SQLite)
- **Pure Go SQLite**: Uses `modernc.org/sqlite`, which requires NO CGO. Do not introduce CGO dependencies.
- **Migrations**: Managed in `internal/db/migrations.go`. Never drop tables; use `INNER JOIN` to handle data migration safely and avoid orphaned records.
- **User Isolation**: Every query MUST filter by `user_id` from the session context.
- **Embedded Assets**: Static files are embedded via `//go:embed static`. Rebuild the Go binary after running `node build.js` to see frontend changes.

### Critical Commands
- **JS Build**: `cd cmd/server && node build.js` (Required after any `src/` change).
- **Full Stack**: `docker-compose up --build` is the recommended dev environment.
- **Go Build**: `go build -o bin/server ./cmd/server` (Requires JS bundle to exist first).
