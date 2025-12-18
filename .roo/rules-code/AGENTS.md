# AGENTS.md

This file provides guidance to agents when working with code in this repository.

## Non-Obvious Coding Rules
- **Component Structure**: Alpine components in `cmd/server/src/components/` must follow: Imports -> Constants -> `Alpine.data()`/`store()` -> State -> `init()` -> Public Methods -> Private Methods (`_` prefix) -> Event Handlers (`handle` prefix).
- **Event Dispatching**: Always use `dispatchEvent(Events.NAME, detail)` from `src/utils/api.js`. Never use raw `CustomEvent` constructors.
- **DOM Manipulation**: Strictly prefer Alpine directives (`x-show`, `x-for`, etc.). Avoid `innerHTML` or manual element creation.
- **Database Queries**: Every query in `internal/db/queries.go` must include a `user_id` filter to ensure data isolation.
- **SQLite Transactions**: Use `db.Begin()` for multi-step operations (like reordering) to ensure atomicity.
- **Error Handling**: Backend errors should be wrapped with `fmt.Errorf("context: %w", err)` to preserve the error chain.
