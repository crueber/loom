# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Loom is a self-hosted bookmark manager with a Trello-like interface. It's a Go backend with a SolidJS frontend, designed for minimal footprint deployment in Docker containers.

## Build & Run Commands

**IMPORTANT: Always use Docker for testing and development. OAuth2 configuration is required for authentication.**

### Prerequisites

Before running Loom, you must:
1. Have an OAuth2/OIDC provider configured (Authentik, Keycloak, etc.)
2. Create a `.env` file with OAuth2 credentials and session keys (see `.env.example`)
3. Generate session keys: `openssl rand -hex 32`

### Docker (Recommended for All Development)

```bash
# Ensure .env file exists with OAuth2 configuration
cp .env.example .env
# Edit .env with your OAuth2 credentials and session keys

# Build and run with docker-compose
docker-compose up --build -d

# View logs
docker-compose logs -f

# Rebuild and restart (after code changes)
docker-compose down && docker-compose up --build -d

# Stop the container
docker-compose down
```

### Local Development (JavaScript Changes)

**IMPORTANT: Source files are in `cmd/server/src/`, NOT `cmd/server/static/`**

When making JavaScript changes:
```bash
# Edit source files in cmd/server/src/
# - cmd/server/src/index.jsx
# - cmd/server/src/components/*.jsx
# - cmd/server/src/utils/*.js

# Build the bundle after changes
cd cmd/server
node build.js

# The bundle is generated at cmd/server/static/dist/app.bundle.js
# NEVER edit files in static/dist/ - they are build artifacts
```

**Key Points:**
- Source files: `cmd/server/src/` (edit these)
- Build output: `cmd/server/static/dist/app.bundle.js` (never edit, gitignored)
- Static assets: `cmd/server/static/` (only truly static files like HTML, CSS, images, lib/)
- Run `node build.js` after any JavaScript changes to test locally
- Docker build handles bundling automatically in production

### Local Binary Building (For Build Testing Only - Not for Running)

```bash
# Build the JavaScript bundle first
cd cmd/server
node build.js

# Then build Go server binary
cd ../..
go build -o bin/server ./cmd/server
```

### Building for Production

The Dockerfile uses multi-stage builds:
- **Stage 1 (Node.js)**: Runs `node build.js` to bundle JavaScript from `src/` to `static/dist/` using esbuild with SolidJS support.
- **Stage 2 (Go)**: Builds Go binary with CGO disabled, embeds all of `static/` including the bundle.
- **Stage 3 (Runtime)**: scratch runtime (< 15MB final image).
- Frontend assets are embedded via `//go:embed static` in cmd/server/main.go.

## Architecture

### Core Components

**Single Binary:**
- `cmd/server/main.go` - Web server with embedded static assets

**Backend Layers:**
1. **API Handlers** (`internal/api/`) - HTTP request handling, organized by domain.
2. **Database** (`internal/db/`) - SQLite with modernc.org/sqlite (pure Go, no CGO).
3. **Auth** (`internal/auth/`) - Security layer (Gorilla sessions).
4. **OAuth2** (`internal/oauth/`) - OAuth2/OIDC client.
5. **Models** (`internal/models/`) - Data structures shared across layers.
6. **Favicon** (`internal/favicon/`) - Fetches favicons using Google's service.

### Frontend Architecture (SolidJS)

**Source files** located in `cmd/server/src/` (edit these):
- `index.jsx` - Main entry point and application root.
- `components/` - Modular SolidJS components:
  - `AuthContext.jsx` - Authentication state management.
  - `BoardContext.jsx` - Global state for boards, lists, and items.
  - `Navigation.jsx` - Header and board switching.
  - `ListsManager.jsx` - List container and reordering.
  - `List.jsx` - Individual list component.
  - `Item.jsx` - Individual bookmark/note component.
  - `ColorPicker.jsx` - List color selection.
- `utils/` - Helper functions:
  - `api.js` - API client.
  - `useDragScroll.js` - Custom hook for horizontal scrolling.

**Static assets** in `cmd/server/static/`:
- `index.html` - Single-page app mount point.
- `styles.css` - Custom styles with Pico.css base.
- `lib/` - Third-party libraries (Marked.js, Pico.css).

**Key Frontend Features:**
- **SolidJS fine-grained reactivity** for state management.
- **Context-based architecture** for shared state (Auth, Board).
- **Native HTML5 Drag & Drop** for list and item reordering.
- **Server-side data bootstrapping** via `window.__BOOTSTRAP_DATA__` for instant loads.
- **Markdown rendering** for notes using Marked.js.
- **Stealth UI** with minimal footprint and touch optimizations.

### Frontend Component Guidelines

**CRITICAL: Follow these patterns for all component development**

- **State Management**: Use `createSignal` for local state and `createStore` (via `BoardContext`) for shared state.
- **Reactivity**: Prefer `<Show>` and `<For>` components over ternary operators or manual mapping.
- **DOM Access**: Use the `ref` attribute for direct DOM access instead of `document.getElementById`.
- **Data Flow**: Consume initial data from `BoardContext` which hydrates from `window.__BOOTSTRAP_DATA__`.

### Backend Guidelines

- **Pure Go SQLite**: Do not introduce CGO dependencies.
- **User Isolation**: Every query MUST filter by `user_id` from the session context.
- **Error Handling**: Wrap backend errors with `fmt.Errorf("context: %w", err)`.
- **Migrations**: Managed in `internal/db/migrations.go`. Never drop tables; use safe data migration patterns.

## Troubleshooting

### Common Issues

- **OAuth2 login fails**: Check `OAUTH2_ISSUER_URL` and ensure redirect URL matches exactly.
- **Reordering fails**: Verify the request body format matches the backend's `ReorderItemsRequest` or `ReorderListsRequest`.
- **Static files not updating**: Rebuild required (run `node build.js` in `cmd/server/`).
