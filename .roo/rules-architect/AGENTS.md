# AGENTS.md

This file provides guidance to agents when working with code in this repository.

## Non-Obvious Architecture Rules
- **CGO-Free**: The project strictly avoids CGO. Use `modernc.org/sqlite` for database interactions.
- **Stateless Frontend**: Components should not rely on `localStorage`. All state must be derived from `window.__BOOTSTRAP_DATA__` or API responses.
- **SolidJS Reactivity**: Use fine-grained reactivity. Avoid re-rendering entire lists when a single item changes.
- **Atomic Reordering**: Reordering operations for lists and items must be performed in a single database transaction using `CASE` statements for efficiency.
- **Migration Safety**: Migrations in `internal/db/migrations.go` must use `INNER JOIN` or existence checks to prevent data loss or orphaned records during schema changes.
