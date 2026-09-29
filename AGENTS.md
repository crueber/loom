# AGENTS.md

Rebuild branch (`rebuild/columns-ux`): single Go binary (`cmd/loom`) + vanilla JS (`web/`).

- Edit `web/app.js`, not build output (no bundler; budget gate `sh scripts/budget-check.sh`).
- Backend: `internal/model`, `internal/store` (`Store` interface; SQLite/file), `internal/api`.
- Checks: `gofmt -l . && go vet ./... && go test ./...`.

## Standing rule

Every feature change updates `FEATURES.md` in the same commit (append/modify one bullet, keep it terse).
