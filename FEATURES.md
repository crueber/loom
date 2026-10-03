# Loom features

Every Chris-requested feature, one terse bullet each.
Pending = requested but not yet built on this branch.

## Boards & columns

- Boards hold ordered columns; navbar switcher.
- Columns hold ordered cards; per-column collapse toggle with a quick (~190ms) width-glide animation and a per-user Animations switch in Settings General (default on; honors reduced-motion).
- Add board / add column from navbar (appends at end).
- Delete board from the user menu (UI confirm modal; removes its columns/cards).
- Column header ▾ menu (right side): color swatches first, Rename second, separator, Delete last (UI confirm modal; removes its cards); header click toggles collapse; menu hidden on collapsed rails; header ink balanced left/right (12px/6px pads, zero title bleed, flush resize, slim ▾ button).
- Pending: deep-link boards via URL.
- Pending: resizable / reorderable columns with drop hints.
- Columns resize by dragging the header handle (widths persist locally); double-click the handle resets to the standard width.
- Pending: collapsed columns as narrow rails.
- Collapsed rails: count sits just under the vertical name; name hover shrinks to text; clicking empty rail space expands the column; rail background reuses the header color wash; adjacent rails share one theme-aware hairline divider (no bright seam on dark themes).
- Board backgrounds + swatch picker; new boards inherit the current board's background (server persists POST background, unknown ids fall back to paper).
- Pending: column themes.
- Pending: board-overflow dropdown.

## Cards & blocks

- Cards hold ordered blocks: link, note (markdown), image.
- Link blocks show DDG icon + host with letter-tile fallback; card links open in same tab (workspace).
- Link icons served locally via `GET /icons/{host}.ico` (server-cached DDG ip3, 7-day TTL).
- Image blocks via upload (jpeg/png/gif, 12MB) or URL.
- Inline note editing in place; blur commits.
- Empty notes render nothing (no "+ add text" placeholder); toolbar Add-note focuses a fresh editable note.
- Markdown renders; re-edit shows raw text.
- Markdown note headers: `#`/`##`/`###` render as three distinct sizes (h4+ stays plain text).
- Card toolbar appears on hover/focus; card delete asks via the shared confirm modal (Esc/Cancel deletes nothing).
- Shared confirm modal (confirm + text-input modes) replaces native confirm/prompt/alert for deletes, board rename, and errors.
- Compact add: + card per column; + link/note/image per card; Add-note always appends a new note at the card bottom and focuses it (never an existing note); add clicks update only that card, never a full-board rebuild, so icon-less link favicons never blink.
- Pending: Enter-save + modifier-newline while editing.
- Pending: drag cards within/across columns with drop hints (move API exists).
- Blocks drag via slim handle parked in the card's left padding to reorder within/across cards (card PATCH persists); blocks keep symmetric full-width layout, never covering icons/checkboxes.
- Block drop zones cover the whole block: hovering any part shows the before/after line (handle stays the drag source).
- Open columns hug content (no tall empty slab); lane background below + add card pans the strip; column reorder starts from the header only so pan is never hijacked.
- Board overflow menu sits outside the tab scroller, so navbar scrolling never clips it.
- Todo-list blocks: one block = one checklist (items array); Enter adds items in-list and focuses the new row ready to type, Up/Down moves between todo rows in the card (caret at end), Escape exits, checkbox toggles via card PATCH; background sync never steals todo focus, Todo… placeholder never edits as text; render blurs-before-rebuild so Enter never strands focus (re-entrancy guard); moving between todos or Escape-cancelling a non-empty row touches no favicon <img> (in-place restore, no board rebuild).
- Single-card text edits (note commit changed/unchanged, todo commit/Enter-chain/empty-delete, checkbox toggle, background card sync) refresh only that card via `refreshCardDOM` — other cards' link favicon nodes survive, so icon-less links never blink; full `render()` stays for column/board structural changes only.
- Pending: reading mode.

## Navigation & chrome

- Single navbar: brand, boards, + board, + column.
- Mobile navbar: hamburger toggles at both topbar edges — left opens the board list, right opens board controls + Settings.
- Settings dialog holds language (EN placeholder) + v1 import; topbar right zone holds one user menu (Dicebear Landscape avatar when signed in, stored seed assigned once per user, Menu button when anonymous) with Share/Themes/Settings/Delete/Login-Logout — Share + swatches no longer sit in #boardctl.
- Settings General renames the current board (empty/unchanged is a no-op; disabled with a hint when no board is open).
- Settings opens as a sidebar modal (General/Data/Account/Admin); theme + language persist locally and sync via GET/PUT /api/me/prefs when signed in.
- Board scrolls horizontally; near-full-width columns under 700px.
- Pending: window-level + skinny scrollbars.
- Pending: drag-pan board.
- Drag card/block-handle/column-header onto the navbar board strip opens a copy/move-to-board dialog (existing APIs only, create-then-delete).

## Data & performance

- SQLite for .db/.sqlite/.sqlite3; JSON file otherwise; empty = in-memory.
- v1 legacy JSON importer.
- v1 import dedupes links on normalized URL (first wins, order kept).
- Cache-first instant load: localStorage paint, bootstrap seed, background revalidate.
- Offline app-shell via service worker; lazy images.
- API reads are network-first (SW cache is offline fallback only) so first reload paints server truth.
- JS budget ≤50KB gzip (now ~3.5KB); first-paint probe `__LOOM_FIRST_PAINT_MS`.

## Access & auth

- Optional OIDC login (Authorization Code + PKCE, stdlib-only), configured from the UI Settings dialog; off by default = open single-user mode.
- Boards are public or private (new boards private when auth is on); per-board share dialog with viewer/editor invites + roles.
- First OIDC sign-in claims legacy unowned boards once (idempotent backfill flag; owned boards untouched; unwired — no caller yet).
- Admin Test connection button POSTs unsaved issuer/client ID to /api/auth/test (discovery-only, persists nothing).
- OIDC login syncs the admin bit from the verified `groups` claim (`admin` exact match; first user bootstraps admin; sole-admin guard blocks last demote).
- Admin OIDC pane shows the effective callback URL plus an optional public base URL override (trailing slash trimmed, non-http(s) rejected).
- Login-required Looms show anonymous visitors a dedicated page (no navbar, Login button) instead of an empty board.
