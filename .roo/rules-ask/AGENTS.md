# AGENTS.md

This file provides guidance to agents when working with code in this repository.

## Non-Obvious Documentation Rules
- **Frontend Source**: `cmd/server/src/` is the source of truth. `cmd/server/static/dist/` is generated.
- **Bootstrap Data**: `window.__BOOTSTRAP_DATA__` is the primary data source for initial render. See `cmd/server/app_handler.go` for the injection logic.
- **Unified Items**: The `items` table and `items.js` component handle both bookmarks and notes. The `bookmarks` table is deprecated.
- **Favicon Service**: Favicons are fetched via Google's service in `internal/favicon/favicon.go` and stored as Base64 data URIs in the DB.
