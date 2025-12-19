# AGENTS.md

This file provides guidance to agents when working with code in this repository.

## Non-Obvious Coding Rules
- **SolidJS Components**: Use Functional Components with JSX. Prefer `Show` and `For` components over ternary operators or manual mapping for better performance.
- **State Management**: Use `createSignal` for local state and `createStore` for complex or shared state.
- **Refs**: Use the `ref` attribute for direct DOM access (e.g., for SortableJS) instead of `document.getElementById`.
- **Database Queries**: Every query in `internal/db/queries.go` must include a `user_id` filter to ensure data isolation.
- **SQLite Transactions**: Use `db.Begin()` for multi-step operations (like reordering) to ensure atomicity.
- **Error Handling**: Backend errors should be wrapped with `fmt.Errorf("context: %w", err)` to preserve the error chain.
