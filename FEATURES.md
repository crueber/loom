# Loom features

Every Chris-requested feature, one terse bullet each.
Pending = requested but not yet built on this branch.

## Boards & columns

- Boards hold ordered columns; navbar switcher.
- Columns hold ordered cards; per-column collapse toggle.
- Add board / add column from navbar (appends at end).
- Delete board from navbar (confirm required; removes its columns/cards).
- Delete column from its header × (confirm required; removes its cards; hidden on collapsed rails).
- Pending: deep-link boards via URL.
- Pending: resizable / reorderable columns with drop hints.
- Pending: collapsed columns as narrow rails.
- Collapsed rails: count sits just under the vertical name; name hover shrinks to text.
- Board backgrounds + swatch picker; new boards inherit the current board's background (server persists POST background, unknown ids fall back to paper).
- Pending: column themes.
- Pending: board-overflow dropdown.

## Cards & blocks

- Cards hold ordered blocks: link, note (markdown), image.
- Link blocks show DDG icon + host with letter-tile fallback; card links open in same tab (workspace).
- Image blocks via upload (jpeg/png/gif, 12MB) or URL.
- Inline note editing in place; blur commits.
- Markdown renders; re-edit shows raw text.
- Markdown note headers: `#`/`##`/`###` render as three distinct sizes (h4+ stays plain text).
- Card toolbar appears on hover/focus.
- Compact add: + card per column; + link/note/image per card.
- Pending: Enter-save + modifier-newline while editing.
- Pending: drag cards within/across columns with drop hints (move API exists).
- Blocks drag via handle to reorder within/across cards (card PATCH persists).
- Block drop zones cover the whole block: hovering any part shows the before/after line (handle stays the drag source).
- Open columns hug content (no tall empty slab); lane background below + add card pans the strip; column reorder starts from the header only so pan is never hijacked.
- Board overflow menu sits outside the tab scroller, so navbar scrolling never clips it.
- Todo-list blocks: one block = one checklist (items array); Enter adds items in-list and focuses the new row ready to type, Escape exits, checkbox toggles via card PATCH; background sync never steals todo focus, Todo… placeholder never edits as text.
- Pending: reading mode.

## Navigation & chrome

- Single navbar: brand, boards, + board, + column.
- Settings dialog holds language (EN placeholder) + v1 import; topbar keeps auth + Settings only.
- Board scrolls horizontally; near-full-width columns under 700px.
- Pending: window-level + skinny scrollbars.
- Pending: drag-pan board.

## Data & performance

- SQLite for .db/.sqlite/.sqlite3; JSON file otherwise; empty = in-memory.
- v1 legacy JSON importer.
- v1 import dedupes links on normalized URL (first wins, order kept).
- Cache-first instant load: localStorage paint, bootstrap seed, background revalidate.
- Offline app-shell via service worker; lazy images.
- JS budget ≤50KB gzip (now ~3.5KB); first-paint probe `__LOOM_FIRST_PAINT_MS`.

## Access & auth

- Optional OIDC login (Authorization Code + PKCE, stdlib-only), configured from the UI Settings dialog; off by default = open single-user mode.
- Boards are public or private (new boards private when auth is on); per-board share dialog with viewer/editor invites + roles.
- First OIDC sign-in claims legacy unowned boards once (idempotent backfill flag; owned boards untouched; unwired — no caller yet).
