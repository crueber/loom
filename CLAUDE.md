# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Loom is a self-hosted bookmark manager with a Trello-like interface. It's a Go backend with vanilla JavaScript frontend, designed for minimal footprint deployment in Docker containers.

## Build & Run Commands

**IMPORTANT: Always use Docker for testing and development. Do not use local binaries for testing as they use auto-generated session keys that change on every restart, causing session issues.**

### Docker (Recommended for All Development)

```bash
# Build and run with docker-compose
docker-compose up --build -d

# View logs
docker-compose logs -f

# Create a user in the container
docker exec -it loom /user create <username>

# Rebuild and restart (after code changes)
docker-compose down && docker-compose up --build -d

# Stop the container
docker-compose down
```

### Local Binary Building (For Build Testing Only - Not for Running)

```bash
# Build both binaries
go build -o bin/server ./cmd/server
go build -o bin/user ./cmd/user

# User management (local)
./bin/user create <username>
./bin/user list
./bin/user delete <username>
./bin/user reset-password <username>
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
   - `data.go` - Combined data endpoint (boards, lists, items) for instant loads
   - `boards.go` - Board CRUD and board-specific data loading
   - `lists.go` - List CRUD, reordering, and copy/move between boards
   - `items.go` - Items (bookmarks & notes) CRUD and reordering
   - `bookmarks.go` - Deprecated, kept for backward compatibility
   - `export.go` - Import/export JSON functionality

2. **Database** (`internal/db/`) - SQLite with modernc.org/sqlite (pure Go, no CGO):
   - `db.go` - Connection management, enables WAL mode and foreign keys
   - `migrations.go` - Schema migrations run on startup
   - `queries.go` - All SQL queries for users, boards, lists, items

3. **Auth** (`internal/auth/`) - Security layer:
   - `password.go` - Argon2id hashing/verification
   - `session.go` - Gorilla sessions with cookie-based storage

4. **Models** (`internal/models/`) - Data structures shared across layers

5. **Favicon** (`internal/favicon/`) - Fetches favicons using Google's service

### Frontend Architecture

Located in `cmd/server/static/`:
- `index.html` - Single-page app structure with Alpine.js directives
- `app.js` - Main initialization (~83 lines)
- `components/` - Modular Alpine.js components:
  - `auth.js` - Authentication logic (login/logout, user state)
  - `lists.js` - List CRUD and rendering
  - `items.js` - Bookmark CRUD and rendering (prepared for Phase 3 Items refactor)
  - `flipCard.js` - Shared card flip behavior and state management
  - `dragScroll.js` - Horizontal drag-to-scroll functionality
  - `cache.js` - LocalStorage management for instant loads
- `utils/api.js` - API helper functions (all fetch calls)
- `styles.css` - Custom styles with Pico.css base (~10KB)
- `lib/` - Third-party libraries:
  - Pico.css (~10KB)
  - SortableJS (drag-and-drop)
  - Alpine.js v3.x (~15KB, reactive framework)

**Key Frontend Features:**
- **Alpine.js reactive components** for state management and modularity
- **Component-based architecture** for easier development and maintenance
- **Event-driven communication** between components via CustomEvents
- Drag-to-scroll: Click and drag whitespace to scroll horizontally
- SortableJS handles list/bookmark reordering with auto-scroll near edges
- Card flip UI: Gear icon (⚙️) flips lists/bookmarks to show configuration panels
- Mobile touch optimizations: Long-press (200ms) to drag, always-visible action buttons on touch devices
- Single flip enforcement: Only one card can be flipped at a time
- Keyboard shortcuts: ESC to close, Enter to save
- Color picker: Inline grid in list configuration (8 predefined colors, validated server-side)
- **LocalStorage caching** for instant page loads with background refresh

### Data Flow

1. **Initialization**: Server loads session keys (or generates them) → initializes DB → runs migrations → starts HTTP server
2. **Authentication**: Cookie-based sessions via Gorilla sessions, middleware on protected routes
3. **User isolation**: All queries filter by `user_id` from session context
4. **Ordering**: Lists and items have `position` fields, reorder endpoints update all positions atomically
5. **Instant Loads**: `/api/data` endpoint returns boards, lists, and items in a single request for optimal performance. Data is cached locally for instant page loads with background refresh.

## Important Patterns

### Card Flip UI Implementation

**Touch Device Detection** ([app.js:4-8](cmd/server/static/app.js#L4-L8)):
- Detects touch capability via `'ontouchstart' in window || navigator.maxTouchPoints > 0`
- Adds `.touch-device` class to body for CSS targeting
- Managed globally in app.js initialization

**Flip State Management** ([components/flipCard.js](cmd/server/static/components/flipCard.js)):
- Global `currentlyFlippedCard` state tracks which card (if any) is flipped
- `flipToList()` and `flipToBookmark()` functions handle flipping logic
- `closeFlippedCard()` handles cleanup and temp card removal
- Disables all SortableJS instances when any card is flipped
- Re-enables sortables when card closes
- ESC key listener closes any open configuration panel

**List Configuration** (3D transform approach):
- CSS: `perspective: 1000px`, `transform-style: preserve-3d`, `rotateY(180deg)`
- Gear icon (⚙️) replaces edit/color/delete buttons
- Configuration panel includes:
  - Title input
  - Inline color grid
  - Board selector with Copy/Move buttons (when multiple boards exist)
  - Delete/cancel/save buttons
- Copy/Move functionality:
  - Board selector dropdown shows all boards except current
  - Copy button duplicates list (with all items) to target board, appends "(copy)" to title
  - Move button transfers list (with all items) to target board
  - Both buttons disabled until board selected
  - After operation, prompts user to navigate to target board
- Managed by [components/lists.js](cmd/server/static/components/lists.js)
- `pointer-events: none` on hidden card sides prevents click-through
- List header collapse disabled when flipped

**Bookmark Configuration** (simple show/hide approach):
- Uses `display: none/block` instead of 3D transforms to avoid layout issues
- Gear icon (⚙️) replaces edit/delete buttons
- Configuration panel includes: title input, URL input, delete/cancel/save buttons
- Managed by [components/items.js](cmd/server/static/components/items.js)
- Naturally expands to fit content without taking out of document flow

**Adding Items** (temporary card pattern):
- **No prompt dialogs**: "+ Add Bookmark" and "+ Add List" create temporary cards immediately
- Temporary cards use string IDs: `temp-${Date.now()}`, marked with `data-is-temp="true"`
- Cards appear flipped to configuration panel with empty inputs
- **Adding bookmarks**: Empty title/URL inputs, save with both filled creates bookmark, empty save cancels
- **Adding lists**: Empty title input, default color (Blue) pre-selected, empty save cancels
- Delete buttons hidden on temp items via CSS: `[data-is-temp="true"] .config-delete-btn { display: none }`
- Cancel/ESC removes temp card from DOM
- `closeFlippedCard()` detects temp items and removes them instead of unflipping

**SortableJS Integration** ([components/lists.js](cmd/server/static/components/lists.js), [components/items.js](cmd/server/static/components/items.js)):
- `delay: 200, delayOnTouchOnly: true` for long-press drag on touch devices
- `filter: '[data-flipped="true"]'` prevents dragging flipped cards
- All instances disabled when any card is flipped via `.option("disabled", true)`
- Re-enabled when card closes via `.option("disabled", false)`
- Auto-scroll near edges, animation on drop
- Managed separately: listsSortable for lists, bookmarkSortables for bookmarks

**Keyboard Support** ([components/flipCard.js:107-111](cmd/server/static/components/flipCard.js#L107-L111)):
- ESC key closes any open configuration panel (including temp items)
- Enter key in inputs saves changes (handled in lists.js and items.js)

**Drag-Scroll** ([components/dragScroll.js](cmd/server/static/components/dragScroll.js)):
- Excludes INPUT, BUTTON, A, TEXTAREA, SELECT from initiating drag-scroll
- Disabled when any card is flipped to prevent interference
- Preserves normal interaction with form elements while maintaining whitespace drag
- Initialized via `initializeHorizontalDragScroll()` in app.js

### Color Validation

Colors must match between frontend and backend. When changing colors:
1. Update `COLORS` array in [components/lists.js](cmd/server/static/components/lists.js)
2. Update CSS classes in [styles.css](cmd/server/static/styles.css)
3. Update `validColors` slice in [internal/api/lists.go](internal/api/lists.go)

### Session Management

- Session keys should be 32 or 64 bytes (hex-encoded)
- Encryption keys should be 16, 24, or 32 bytes (hex-encoded)
- Auto-generated keys are logged as warnings in development
- Set `SECURE_COOKIE=true` when behind HTTPS

### Database Migrations

Migrations run automatically on startup in `internal/db/migrations.go`. Add new migrations to the `migrations` slice with incrementing versions. The system tracks applied migrations in a `migrations` table.

**CRITICAL: Backward Compatibility Requirement**
- **NEVER drop the database** to fix migration errors - migrations must handle existing data safely
- Always maintain backward compatibility when modifying schema or migrating data
- Handle orphaned records: Use INNER JOIN to copy only records with valid foreign key references
- Include data integrity checks: Verify row counts match before and after migration
- Check for table existence before attempting to migrate from it
- Example: Migration v3 (bookmarks → items) uses `INNER JOIN lists` to skip orphaned bookmarks
- Default boards: Created lazily in `GetBoards()` when user has no boards (not during user creation)

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
- **Card flip interface**: Configuration panels appear as "back" of cards using 3D transforms (lists) or simple show/hide (bookmarks)
- **Drag-to-scroll**: Users can click and drag whitespace to scroll (grab cursors indicate draggable areas)
- **Touch-friendly**: On touch devices, action buttons always visible, 44px minimum touch targets, long-press to initiate drag
- **Single focus**: Only one configuration panel open at a time, previous auto-closes
- **Darker colors**: All list header colors are darker (~35% darker than typical) for better readability with white text
- **Minimal footprint**: Compact sizing throughout (nav bar ~30-35px, small fonts, tight spacing)
- **Adaptive compression**: Bookmark URLs hidden when list has 7+ bookmarks (using `:has(.bookmark-item:nth-child(7))` selector)
- **Bottom padding**: 3rem padding at bottom of lists wrapper to clearly show where content ends

## Component Architecture (Phase 1: Completed)

### Alpine.js Integration

The frontend uses Alpine.js v3.x for reactive state management and component modularity. Each component is registered using `Alpine.data()` and attached to DOM elements via `x-data` directives.

**Component Structure:**
- Each component is a self-contained module in `components/` directory
- Components communicate via CustomEvents for loose coupling
- State is managed within each component using Alpine's reactivity
- Global state (like `currentlyFlippedCard`) is managed in dedicated components

### Component Communication Patterns

**Event-Driven Architecture:**
Components communicate through CustomEvents rather than direct function calls:

```javascript
// Dispatching events
const event = new CustomEvent('eventName', { detail: { data: value } });
document.dispatchEvent(event);

// Listening for events
document.addEventListener('eventName', (event) => {
    // Handle event.detail.data
});
```

**Key Events:**
- `userLoggedIn` - Dispatched when user logs in successfully
- `userLoggedOut` - Dispatched when user logs out
- `boardsDataLoaded` - Boards data loaded from `/api/data` endpoint
- `boardDataLoaded` - Specific board data loaded (for non-default boards)
- `bookmarksDataLoaded` - Lists manager notifies items manager of items data (maintains legacy name for compatibility)
- `renderListBookmarks` - Request to render items for a specific list
- `addBookmarkRequested` - Request to add a new bookmark to a list
- `addNoteRequested` - Request to add a new note to a list
- `listDeleted` - Notify items manager that a list was deleted
- `listsUpdated` - Notify cache update needed after list changes
- `listFlipped` / `bookmarkFlipped` / `noteFlipped` - Notify that a card was flipped
- `removeTempList` - Request to remove temporary list from state
- `reloadDataRequested` - Trigger full data reload

### Cache Management

[components/cache.js](cmd/server/static/components/cache.js) provides instant page loads:
1. Load cached data from localStorage immediately (instant render)
2. Fetch fresh data from server in background (single `/api/data` call includes boards, lists, and items)
3. Compare cached vs fresh data using JSON.stringify
4. Only re-render if data has changed
5. Save fresh data to cache for next page load

**Cache Data Structure:**
```json
{
  "boards": [...],
  "lists": [...],
  "items": {
    "1": [...],  // items keyed by list_id
    "2": [...]
  }
}
```

### Component Responsibilities

**authManager** ([components/auth.js](cmd/server/static/components/auth.js)):
- Login/logout functionality
- Current user state management
- Screen visibility (login vs app)
- Dispatches `userLoggedIn` and `userLoggedOut` events

**listsManager** ([components/lists.js](cmd/server/static/components/lists.js)):
- Lists CRUD operations (create, read, update, delete)
- List rendering with Alpine.js templates
- SortableJS integration for drag-and-drop
- Temporary list creation pattern
- Color selection and validation
- Collapse/expand functionality
- Dispatches events for bookmark rendering and cache updates

**itemsManager** ([components/items.js](cmd/server/static/components/items.js)):
- Bookmarks CRUD operations
- Bookmark rendering within lists
- SortableJS integration for bookmark reordering
- Cross-list bookmark dragging
- Temporary bookmark creation pattern
- Prepared for Phase 3 refactor to unified Items (links & notes)

**Standalone Utilities:**
- [components/flipCard.js](cmd/server/static/components/flipCard.js) - Global flip state management
- [components/dragScroll.js](cmd/server/static/components/dragScroll.js) - Horizontal scroll functionality
- [components/cache.js](cmd/server/static/components/cache.js) - LocalStorage helpers
- [utils/api.js](cmd/server/static/utils/api.js) - All API fetch calls

### Development Patterns

**When Adding New Features:**
1. Determine which component owns the feature
2. Add state and methods to the appropriate Alpine component
3. Use CustomEvents for cross-component communication
4. Update cache via `listsUpdated` or direct `saveToCache()` calls
5. Test that events propagate correctly between components

**When Modifying Components:**
1. Maintain event-driven communication (avoid direct coupling)
2. Keep components focused on their core responsibilities
3. Use Alpine's reactivity (`this.$nextTick()`, etc.) for DOM updates
4. Remember that SortableJS instances need manual enable/disable management

**Migration Notes (Phase 1 → Phase 2/3):**
- `items.js` is currently managing bookmarks, will be refactored in Phase 3 to handle both links and notes
- Component structure is prepared for boards feature (Phase 2)
- All components use event-driven patterns to facilitate future multi-board support

## Troubleshooting

### Debug Output

The codebase has DEBUG println statements in `internal/auth/session.go`. These can be removed for production but are helpful for diagnosing session issues.

### Common Issues

- **Color picker 400 errors**: Color not in `validColors` array in `internal/api/lists.go`
- **Session expiry**: Check `SESSION_MAX_AGE` and that session keys persist across restarts
- **Favicons not loading**: Requires outbound HTTPS access to Google's favicon service
- **Static files not updating**: Assets are embedded at build time, rebuild required
