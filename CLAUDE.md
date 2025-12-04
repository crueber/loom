# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Home Links is a self-hosted bookmark manager with a Trello-like interface. It's a Go backend with vanilla JavaScript frontend, designed for minimal footprint deployment in Docker containers.

## Build & Run Commands

### Local Development

```bash
# Build both binaries
go build -o bin/server ./cmd/server
go build -o bin/user ./cmd/user

# Run the server (development)
./bin/server

# Create a user
./bin/user create <username>

# Other user commands
./bin/user list
./bin/user delete <username>
./bin/user reset-password <username>
```

### Docker

```bash
# Build and run with docker-compose
docker-compose up --build -d

# View logs
docker-compose logs -f

# Create a user in the container
docker exec -it home-links /user create <username>

# Rebuild and restart (after code changes)
docker-compose down && docker-compose up --build -d
```

### Building for Production

The Dockerfile uses multi-stage builds with CGO disabled for static binaries:
- Stage 1: golang:1.24-alpine builder
- Stage 2: scratch runtime (< 15MB final image)
- Frontend assets are embedded via `//go:embed static` in cmd/server/main.go

## Architecture

### Core Components

**Two Binaries:**
- `cmd/server/main.go` - Web server with embedded static assets
- `cmd/user/main.go` - CLI tool for user management

**Backend Layers:**
1. **API Handlers** (`internal/api/`) - HTTP request handling, organized by domain:
   - `auth.go` - Login/logout/registration
   - `lists.go` - List CRUD and reordering
   - `bookmarks.go` - Bookmark CRUD and reordering
   - `export.go` - Import/export JSON functionality

2. **Database** (`internal/db/`) - SQLite with modernc.org/sqlite (pure Go, no CGO):
   - `db.go` - Connection management, enables WAL mode and foreign keys
   - `migrations.go` - Schema migrations run on startup
   - `queries.go` - All SQL queries for users, lists, bookmarks

3. **Auth** (`internal/auth/`) - Security layer:
   - `password.go` - Argon2id hashing/verification
   - `session.go` - Gorilla sessions with cookie-based storage

4. **Models** (`internal/models/`) - Data structures shared across layers

5. **Favicon** (`internal/favicon/`) - Fetches favicons using Google's service

### Frontend Architecture

Located in `cmd/server/static/`:
- `index.html` - Single-page app structure
- `app.js` - Vanilla JavaScript (no frameworks), uses SortableJS for drag-and-drop
- `styles.css` - Custom styles with Pico.css base (~10KB)
- `lib/` - Third-party libraries (Pico.css, SortableJS)

**Key Frontend Features:**
- Drag-to-scroll: Click and drag whitespace to scroll horizontally
- SortableJS handles list/bookmark reordering with auto-scroll near edges
- Color picker modal with 8 predefined colors (validated server-side)

### Data Flow

1. **Initialization**: Server loads session keys (or generates them) → initializes DB → runs migrations → starts HTTP server
2. **Authentication**: Cookie-based sessions via Gorilla sessions, middleware on protected routes
3. **User isolation**: All queries filter by `user_id` from session context
4. **Ordering**: Lists and bookmarks have `position` fields, reorder endpoints update all positions atomically

## Important Patterns

### Color Validation

Colors must match between frontend and backend. When changing colors:
1. Update `COLORS` array in `cmd/server/static/app.js`
2. Update CSS classes in `cmd/server/static/styles.css`
3. Update `validColors` slice in `internal/api/lists.go`

### Session Management

- Session keys should be 32 or 64 bytes (hex-encoded)
- Encryption keys should be 16, 24, or 32 bytes (hex-encoded)
- Auto-generated keys are logged as warnings in development
- Set `SECURE_COOKIE=true` when behind HTTPS

### Database Migrations

Migrations run automatically on startup in `internal/db/migrations.go`. Add new migrations to the `migrations` slice with incrementing versions. The system tracks applied migrations in a `migrations` table.

### Embedded Assets

Static files are embedded at compile time using `//go:embed static` in `cmd/server/main.go`. Changes to static files require a rebuild to take effect.

## Configuration

All configuration via environment variables (no config files):

| Variable | Purpose | Default |
|----------|---------|---------|
| `DATABASE_PATH` | SQLite database location | `./data/bookmarks.db` |
| `PORT` | HTTP server port | `8080` |
| `SESSION_KEY` | Cookie signing key (hex) | Auto-generated |
| `ENCRYPTION_KEY` | Cookie encryption key (hex) | Auto-generated |
| `SESSION_MAX_AGE` | Session duration in seconds | `31536000` (1 year) |
| `SECURE_COOKIE` | Enable secure flag on cookies | `false` |

## UI Design Philosophy

- **Stealth UI**: Navigation bar at 60% opacity, buttons at 50%, fade to full on hover
- **Drag-to-scroll**: Users can click and drag whitespace to scroll (grab cursors indicate draggable areas)
- **Darker colors**: All list header colors are darker (~35% darker than typical) for better readability with white text
- **Minimal footprint**: Compact sizing throughout (nav bar ~30-35px, small fonts, tight spacing)
- **Bottom padding**: 3rem padding at bottom of lists wrapper to clearly show where content ends

## Troubleshooting

### Debug Output

The codebase has DEBUG println statements in `internal/auth/session.go`. These can be removed for production but are helpful for diagnosing session issues.

### Common Issues

- **Color picker 400 errors**: Color not in `validColors` array in `internal/api/lists.go`
- **Session expiry**: Check `SESSION_MAX_AGE` and that session keys persist across restarts
- **Favicons not loading**: Requires outbound HTTPS access to Google's favicon service
- **Static files not updating**: Assets are embedded at build time, rebuild required
