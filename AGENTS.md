# AGENTS.md

This file provides guidance to agents when working with code in this repository.

## Non-Obvious Project Patterns

### Frontend (SolidJS + esbuild)
- **Source vs Static**: Edit files in `cmd/server/src/`, NOT `cmd/server/static/dist/`. The latter are build artifacts.
- **JSX Components**: Templates are now in `.jsx` files within `src/components/`. `index.html` is just a mount point.
- **Data Bootstrapping**: Initial data is injected via `window.__BOOTSTRAP_DATA__` and consumed by SolidJS stores/contexts on initialization.
- **Reactivity**: Use SolidJS signals (`createSignal`) and stores (`createStore`) for state. Avoid direct DOM manipulation.
- **Card Flip UI**: 
  - Lists use 3D transforms (`rotateY(180deg)`).
  - Bookmarks/Notes use simple `display: none/block` toggles.
  - Managed via SolidJS component state.
- **Temporary Items**: New items use string IDs (e.g., `temp-123`) and are filtered out if cancelled.
- **Internationalization (i18n)**: 
  - **No Hardcoded Strings**: All user-facing text must use the `t()` function from `useI18n()`.
  - **Translation Files**: Add new keys to all translations in `cmd/server/static/locales/`.
  - **Backend Injection**: Translations are injected via `window.__I18N_DATA__` in `cmd/server/app_handler.go`.

### Backend (Go + SQLite)
- **Pure Go SQLite**: Uses `modernc.org/sqlite`, which requires NO CGO. Do not introduce CGO dependencies.
- **Migrations**: Managed in `internal/db/migrations.go`. Never drop tables; use `INNER JOIN` to handle data migration safely and avoid orphaned records.
- **User Isolation**: Every query MUST filter by `user_id` from the session context.
- **Embedded Assets**: Static files are embedded via `//go:embed static`. Rebuild the Go binary after running `node build.js` to see frontend changes.
- **In-Memory Caching**: 
  - The `ServeApp` handler uses an LRU cache (keyed by `userID:boardID`) to store fully hydrated HTML.
  - **Invalidation**: Managed via `cacheInvalidationMiddleware` in `cmd/server/middleware.go`. It intercepts `POST`, `PUT`, and `DELETE` requests to `/api/boards`, `/api/lists`, and `/api/items`.
  - **Manual Invalidation**: If adding new mutation endpoints, ensure they are covered by the middleware or call `appHandler.InvalidateCache(userID, boardID)` manually.

### Critical Commands
- **JS Build**: `cd cmd/server && node build.js` (Required after any `src/` change).
- **Full Stack**: `docker-compose up --build` is the recommended dev environment.
- **Go Build**: `go build -o bin/server ./cmd/server` (Requires JS bundle to exist first).
