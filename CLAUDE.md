# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Loom is a self-hosted bookmark manager with a Trello-like interface. It's a Go backend with vanilla JavaScript frontend, designed for minimal footprint deployment in Docker containers.

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
# - cmd/server/src/app.js
# - cmd/server/src/components/*.js
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

# Note: /user CLI tool has been removed - users are auto-provisioned via OAuth2
```

### Building for Production

The Dockerfile uses multi-stage builds:
- **Stage 1 (Node.js)**: Runs `node build.js` to bundle JavaScript from `src/` to `static/dist/`
- **Stage 2 (Go)**: Builds Go binary with CGO disabled, embeds all of `static/` including the bundle
- **Stage 3 (Runtime)**: scratch runtime (< 15MB final image)
- Frontend assets are embedded via `//go:embed static` in cmd/server/main.go

## Architecture

### Core Components

**Single Binary:**
- `cmd/server/main.go` - Web server with embedded static assets

**Backend Layers:**
1. **API Handlers** (`internal/api/`) - HTTP request handling, organized by domain:
   - `auth.go` - OAuth2 authentication (login/logout/callback)
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
   - `session.go` - Gorilla sessions with cookie-based storage (SameSite=Lax for OAuth2)

4. **OAuth2** (`internal/oauth/`) - OAuth2/OIDC client:
   - `client.go` - OIDC auto-discovery, token exchange, ID token verification

5. **Models** (`internal/models/`) - Data structures shared across layers

6. **Favicon** (`internal/favicon/`) - Fetches favicons using Google's service

### Frontend Architecture

**Source files** located in `cmd/server/src/` (edit these):
- `app.js` - Main initialization (~83 lines)
- `components/` - Modular Alpine.js components:
  - `auth.js` - Authentication logic (login/logout, user state)
  - `lists.js` - List CRUD and rendering
  - `items.js` - Bookmark CRUD and rendering (prepared for Phase 3 Items refactor)
  - `flipCard.js` - Shared card flip behavior and state management
  - `dragScroll.js` - Horizontal drag-to-scroll functionality
  - `cache.js` - LocalStorage management for instant loads
  - `events.js` - Event name constants registry
  - `dataBootstrap.js` - Data loading and initialization
  - `boards.js` - Board management and navigation
- `utils/api.js` - API helper functions (all fetch calls, event dispatcher)

**Static assets** in `cmd/server/static/` (never edit components here):
- `index.html` - Single-page app structure with Alpine.js directives
- `styles.css` - Custom styles with Pico.css base (~10KB)
- `lib/` - Third-party libraries:
  - Pico.css (~10KB)
  - SortableJS (drag-and-drop)
  - Alpine.js v3.x (~15KB, reactive framework)
  - Marked.js (markdown rendering)
- `dist/` - Build output (gitignored):
  - `app.bundle.js` - Bundled JavaScript from src/
  - `version.txt` - Build version hash

**Key Frontend Features:**
- **Alpine.js reactive components** for state management and modularity
- **Component-based architecture** for easier development and maintenance
- **Event-driven communication** between components via CustomEvents
- **Alpine.store()** for shared state (flip card state, sortable instances)
- **Centralized event registry** (events.js) for all CustomEvent names
- Drag-to-scroll: Click and drag whitespace to scroll horizontally
- SortableJS handles list/bookmark reordering with auto-scroll near edges
- Card flip UI: Gear icon (⚙️) flips lists/bookmarks to show configuration panels
- Mobile touch optimizations: Long-press (200ms) to drag, always-visible action buttons on touch devices
- Single flip enforcement: Only one card can be flipped at a time
- Keyboard shortcuts: ESC to close, Enter to save
- Color picker: Inline grid in list configuration (8 predefined colors, validated server-side)
- **LocalStorage caching** for instant page loads with background refresh

### Frontend Component Guidelines

**CRITICAL: Follow these patterns for all component development**

#### Component File Structure

Each component must follow this structure:

1. **Imports** - All dependencies at top
2. **Constants** - Component-specific constants (colors, etc.)
3. **Component Definition** - Single `Alpine.data()` or `Alpine.store()` export
4. **State** - Reactive properties
5. **Lifecycle** - `init()` method
6. **Public Methods** - Core functionality (alphabetical order)
7. **Private Methods** - Internal helpers prefixed with `_` (alphabetical order)
8. **Event Handlers** - Methods prefixed with `handle` (alphabetical order)

#### State Management Rules

- **Component state**: Use `Alpine.data()` properties for component-local state
- **Shared state**: Use `Alpine.store()` for state shared across components (NOT global variables)
- **External events**: Use `dispatchEvent()` facade from utils/api.js with Events constants
- **Internal state**: Keep private with underscore prefix (`_internalState`)

**Examples:**
```javascript
// Component-local state (auth.js)
Alpine.data('authManager', () => ({
    currentUser: null,  // ✅ Component state
    loading: true,
    // ...
}));

// Shared state (flipCard.js)
Alpine.store('flipCard', {
    currentlyFlipped: null,  // ✅ Shared across components
    listsSortable: null,
    bookmarkSortables: {}
});

// ❌ NEVER use global variables
let currentlyFlippedCard = null;  // NO!
```

#### Event Communication Patterns

**Use centralized event registry:**
```javascript
import { Events } from './events.js';
import { dispatchEvent } from '../utils/api.js';

// Dispatching events
dispatchEvent(Events.BOARD_DATA_LOADED, {
    board: boardData,
    lists: listsData
});

// Listening for events
document.addEventListener(Events.BOARD_DATA_LOADED, (event) => {
    this.board = event.detail.board;
    this.lists = event.detail.lists;
});
```

**Event naming conventions:**
- Use SCREAMING_SNAKE_CASE for event constant names
- Use past tense for events (DATA_LOADED, USER_LOGGED_IN)
- Document all events in events.js with JSDoc comments

#### DOM Manipulation Rules

- ✅ **ALWAYS prefer Alpine directives**: `x-show`, `x-if`, `x-for`, `x-bind`, `x-model`, `x-text`
- ✅ **SOMETIMES use $refs**: For direct DOM access when Alpine can't handle it
- ⚠️ **RARELY use querySelector**: Only in flipCard.js for legacy card flip logic
- ❌ **NEVER manually create DOM elements**: Use Alpine templates with `x-for` or `x-if`
- ❌ **NEVER use innerHTML**: Use Alpine's reactivity instead

**Examples:**
```javascript
// ✅ Good - Alpine handles rendering
get visibleLists() {
    return this.lists.filter(l => !l.collapsed);
}

// ❌ Bad - Manual DOM manipulation
renderLists() {
    const html = this.lists.map(l => `<div>...</div>`).join('');
    container.innerHTML = html;
}
```

#### Template Location Rules

- **Simple toggles**: Use `x-show` or `x-if` inline in index.html
- **Repeated elements**: Use `x-for` with Alpine data objects
- **Complex templates**: Keep in index.html, NOT in separate files
- **Dynamic attributes**: Use `x-bind:` or `:` shorthand

**IMPORTANT: Avoid `<template>` tag issues:**
- Alpine's `x-if` uses `<template>` internally - prefer this over raw `<template>` tags
- For `x-for`, directly use on the element to repeat, not a wrapper `<template>`
- Only use `<template>` when you need a non-rendering container for `x-if`

**Examples:**
```html
<!-- ✅ Good - x-for on the element itself -->
<div x-for="list in lists" :key="list.id" class="list-card">
    <!-- list content -->
</div>

<!-- ✅ Good - x-if with template when needed -->
<template x-if="showDeleteUI">
    <div class="delete-confirmation">
        <!-- confirmation UI -->
    </div>
</template>

<!-- ❌ Bad - unnecessary template wrapper -->
<template x-for="list in lists">
    <div class="list-card">...</div>
</template>
```

#### Component Method Organization

**Public methods** (no underscore prefix):
- CRUD operations: `createList()`, `updateList()`, `deleteList()`
- UI actions: `flipToList()`, `saveList()`, `cancelEdit()`
- Event handlers: `handleListFlipped()`, `handleKeyDown()`

**Private methods** (underscore prefix):
- Internal helpers: `_buildListData()`, `_validateInput()`
- DOM queries: `_getListElement()`, `_focusInput()`
- Data transformations: `_prepareApiPayload()`

**Lifecycle:**
- Use `init()` for component initialization
- Set up event listeners in `init()`
- Clean up resources in component destruction (if needed)

#### Testing Checklist

Before committing component changes:
- [ ] No global variables (use Alpine.store() instead)
- [ ] All events use Events constants from events.js
- [ ] All event dispatches use dispatchEvent() facade
- [ ] No manual DOM creation (use Alpine templates)
- [ ] Methods organized: public, private, handlers
- [ ] Event listeners registered in init()
- [ ] JSDoc comments for public methods
- [ ] Component follows file structure guidelines

### Data Flow

1. **Initialization**: Server validates OAuth2 config → loads session keys → initializes DB → runs migrations → starts HTTP server
2. **Authentication**:
   - User clicks "Log In" → redirected to OAuth2 provider (e.g., Authentik)
   - OAuth2 provider redirects back with authorization code
   - Server exchanges code for ID token, verifies token, extracts user email
   - Auto-provision: Create user if first login, create default board
   - Create Gorilla session with user_id
3. **User isolation**: All queries filter by `user_id` from session context
4. **Ordering**: Lists and items have `position` fields, reorder endpoints update all positions atomically
5. **Instant Loads**: `/api/data` endpoint returns boards, lists, and items in a single request for optimal performance. Data is cached locally for instant page loads with background refresh.

## Important Patterns

### Navigation and Board Management

**Desktop Navigation** (> 768px):
- Logo, board switcher, username, Export/Import buttons, Logout button
- Board switcher dropdown contains:
  - List of all boards (clickable to navigate)
  - "Rename Board" → inline input with Cancel/Save buttons
  - "Delete Board" → inline confirmation with Cancel/Delete buttons
  - "+ New Board" at bottom
- No click-to-rename on board title (removed)
- All actions use inline UI patterns (no browser confirm dialogs)

**Mobile Navigation** (< 768px):
- Logo on left, hamburger menu (☰) on right
- Hamburger opens overlay from right side containing:
  - **Boards section**: Same board switcher dropdown as desktop
  - **Account section**: Username, Export/Import buttons, Logout button
- Clicking outside overlay or any navigation item closes menu
- Close button (×) in top-right of overlay

**Board Management Logic**:
- Cannot delete the only remaining board
- Deleting a board redirects to another board or default board
- Renaming updates board title in place without page reload
- All board operations update cache immediately

### Card Flip UI Implementation

**Touch Device Detection** ([app.js:4-8](cmd/server/static/app.js#L4-L8)):
- Detects touch capability via `'ontouchstart' in window || navigator.maxTouchPoints > 0`
- Adds `.touch-device` class to body for CSS targeting
- Managed globally in app.js initialization

**Flip State Management** ([src/components/flipCard.js](cmd/server/src/components/flipCard.js)):
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
- Managed by [src/components/lists.js](cmd/server/src/components/lists.js)
- `pointer-events: none` on hidden card sides prevents click-through
- List header collapse disabled when flipped

**Bookmark Configuration** (simple show/hide approach):
- Uses `display: none/block` instead of 3D transforms to avoid layout issues
- Gear icon (⚙️) replaces edit/delete buttons
- Configuration panel includes: title input, URL input, delete/cancel/save buttons
- Managed by [src/components/items.js](cmd/server/src/components/items.js)
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

**SortableJS Integration** ([src/components/lists.js](cmd/server/src/components/lists.js), [src/components/items.js](cmd/server/src/components/items.js)):
- `delay: 200, delayOnTouchOnly: true` for long-press drag on touch devices
- `filter: '[data-flipped="true"]'` prevents dragging flipped cards
- All instances disabled when any card is flipped via `.option("disabled", true)`
- Re-enabled when card closes via `.option("disabled", false)`
- Auto-scroll near edges, animation on drop
- Managed separately: listsSortable for lists, bookmarkSortables for bookmarks

**Keyboard Support** ([src/components/flipCard.js](cmd/server/src/components/flipCard.js)):
- ESC key closes any open configuration panel (including temp items)
- Enter key in inputs saves changes (handled in lists.js and items.js)

**Drag-Scroll** ([src/components/dragScroll.js](cmd/server/src/components/dragScroll.js)):
- Excludes INPUT, BUTTON, A, TEXTAREA, SELECT from initiating drag-scroll
- Disabled when any card is flipped to prevent interference
- Preserves normal interaction with form elements while maintaining whitespace drag
- Initialized via `initializeHorizontalDragScroll()` in app.js

### Color Validation

Colors must match between frontend and backend. When changing colors:
1. Update `COLORS` array in [src/components/lists.js](cmd/server/src/components/lists.js)
2. Update CSS classes in [styles.css](cmd/server/static/styles.css)
3. Update `validColors` slice in [internal/api/lists.go](internal/api/lists.go)

### OAuth2 Authentication

**Requirements:**
- OAuth2/OIDC provider must be configured (Authentik, Keycloak, etc.)
- Required environment variables: `OAUTH2_ISSUER_URL`, `OAUTH2_CLIENT_ID`, `OAUTH2_CLIENT_SECRET`, `OAUTH2_REDIRECT_URL`
- Provider must include `openid`, `profile`, and `email` scopes
- **ID token encryption must be disabled** (use signed JWT, not encrypted JWE)
- Issuer URL should NOT include `/.well-known/openid-configuration` (auto-appended)

**Auto-Provisioning:**
- Users are identified by email from OAuth2 claims
- On first login, user account is created automatically
- Default board "My Bookmarks" is created for new users
- Existing users (by email) log in to existing accounts

### Session Management

- Session keys must be 32 bytes (64 hex characters)
- Encryption keys must be 32 bytes (64 hex characters)
- **Session keys are mandatory** - no auto-generation in production
- Generate keys with: `openssl rand -hex 32`
- Set `SECURE_COOKIE=true` when behind HTTPS
- SameSite=Lax for OAuth2 redirect compatibility

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

### Required Variables

| Variable | Purpose | Example |
|----------|---------|---------|
| `OAUTH2_ISSUER_URL` | OAuth2 provider issuer URL | `https://auth.example.com/application/o/loom/` |
| `OAUTH2_CLIENT_ID` | OAuth2 client ID | `abc123` |
| `OAUTH2_CLIENT_SECRET` | OAuth2 client secret | `secret123` |
| `OAUTH2_REDIRECT_URL` | OAuth2 callback URL | `http://localhost:8080/auth/callback` |
| `SESSION_KEY` | Cookie signing key (64 hex chars) | Generate with `openssl rand -hex 32` |
| `ENCRYPTION_KEY` | Cookie encryption key (64 hex chars) | Generate with `openssl rand -hex 32` |

### Optional Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `DATABASE_PATH` | SQLite database location | `./data/bookmarks.db` |
| `PORT` | HTTP server port | `8080` |
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
- **Mobile responsive**: Hamburger menu (< 768px) with overlay for navigation, all features accessible on mobile
- **No confirm dialogs**: Inline confirmation patterns for destructive actions (delete board, delete list)

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

[src/components/cache.js](cmd/server/src/components/cache.js) provides instant page loads:
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

**authManager** ([src/components/auth.js](cmd/server/src/components/auth.js)):
- OAuth2 login flow (redirects to `/auth/login`)
- Logout functionality
- Current user state management
- Screen visibility (login vs app)
- Manual login button (no auto-redirect after logout)
- Dispatches `userLoggedIn` and `userLoggedOut` events

**boardsManager** ([src/components/boards.js](cmd/server/src/components/boards.js)):
- Board CRUD operations (create, rename, delete)
- Board switching and navigation
- Mobile menu state management
- Board switcher dropdown with inline rename/delete UI
- Prevents deleting the only board
- Manages board data caching

**listsManager** ([src/components/lists.js](cmd/server/src/components/lists.js)):
- Lists CRUD operations (create, read, update, delete)
- List rendering with Alpine.js templates
- SortableJS integration for drag-and-drop
- Temporary list creation pattern
- Color selection and validation
- Collapse/expand functionality
- Copy/move lists between boards
- Dispatches events for bookmark rendering and cache updates

**itemsManager** ([src/components/items.js](cmd/server/src/components/items.js)):
- Items (bookmarks & notes) CRUD operations
- Item rendering within lists
- SortableJS integration for item reordering
- Cross-list item dragging
- Temporary item creation pattern
- Markdown rendering for notes with color tag support
- Handles both bookmarks and notes (unified items approach)

**Standalone Utilities:**
- [src/components/flipCard.js](cmd/server/src/components/flipCard.js) - Global flip state management
- [src/components/dragScroll.js](cmd/server/src/components/dragScroll.js) - Horizontal scroll functionality
- [src/components/cache.js](cmd/server/src/components/cache.js) - LocalStorage helpers
- [src/utils/api.js](cmd/server/src/utils/api.js) - All API fetch calls

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

The codebase has DEBUG println statements in `internal/auth/session.go` and `internal/api/auth.go`. These can be removed for production but are helpful for diagnosing authentication issues.

### Common Issues

- **OAuth2 login fails**:
  - **"Failed to initialize OAuth2 client"**: Check `OAUTH2_ISSUER_URL` is correct and doesn't include `/.well-known/openid-configuration`
  - **"Invalid session state"**: Check cookies are enabled, SameSite=Lax should work with OAuth2
  - **"Failed to verify ID token"**: Disable ID token encryption in OAuth2 provider (use signed JWT, not encrypted JWE)
  - Ensure redirect URL matches exactly between `.env` and OAuth2 provider config
- **Auto-provisioning fails**: Check OAuth2 provider sends `email` claim in ID token
- **Color picker 400 errors**: Color not in `validColors` array in `internal/api/lists.go`
- **Session expiry**: Check `SESSION_MAX_AGE` and that session keys persist across restarts
- **Favicons not loading**: Requires outbound HTTPS access to Google's favicon service
- **Static files not updating**: Assets are embedded at build time, rebuild required (run `node build.js` in `cmd/server/`)
