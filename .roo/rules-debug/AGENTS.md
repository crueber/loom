# AGENTS.md

This file provides guidance to agents when working with code in this repository.

## Non-Obvious Debugging Rules
- **Session Debugging**: Check `internal/auth/session.go` and `internal/api/auth.go` for `println` debug statements.
- **OAuth2 Failures**: Verify `OAUTH2_ISSUER_URL` does NOT end with `/.well-known/openid-configuration`.
- **Frontend Logs**: Check the browser console for "✓ JavaScript bundled successfully" to verify the build hash matches `version.txt`.
- **Database State**: Use `sqlite3 ./data/loom.db` to inspect the `migrations` table if schema issues occur.
- **Cookie Issues**: If login fails, check if `SECURE_COOKIE` is `true` while running on `http`.
