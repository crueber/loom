# AGENTS
1. Primary workflow runs inside Docker: `docker-compose up --build -d`.
2. Rebuild/restart after code changes with `docker-compose down && docker-compose up --build -d`.
3. Tail logs via `docker-compose logs -f`; stop with `docker-compose down`.
4. Build binaries locally when needed: `go build -o bin/server ./cmd/server` and `go build -o bin/user ./cmd/user`.
5. CLI user management (inside container or locally): `./bin/user <cmd>`.
6. Run full tests from repo root: `docker-compose run --rm loom go test ./...` (preferred) or `go test ./...`.
7. Run a single test: `go test ./internal/api -run TestName`.
8. Formatting: run `gofmt -w` on touched Go files; no other formatters configured.
9. Imports: stdlib, blank line, third-party, project (`github.com/crueber/loom/...`).
10. Use explicit types and avoid `interface{}` unless required; keep shared structs in `internal/models`.
11. Prefer short, descriptive names; exported identifiers need doc-comments.
12. Return early on errors; wrap with context (`fmt.Errorf("...: %w", err)`) before bubbling.
13. Never log sensitive data (session keys, passwords); rely on structured API error helpers.
14. Keep HTTP handlers thin: validate input, call db layer, respond via helper functions.
15. Ensure color/config constants stay in sync across frontend (`cmd/server/static`) and backend.
16. For JS, stay vanilla + Alpine; no new frameworks; keep features modular under `components/`.
17. Use CustomEvents for component communication and honor existing flip/drag patterns.
18. Cache changes must update `components/cache.js` plus relevant events (e.g., `listsUpdated`).
19. No Cursor or Copilot rules exist; follow this file plus repo docs.
20. When unsure, mirror existing style before expanding scope and confirm with maintainers.
