# Loom features

Every Chris-requested feature, one terse bullet each.
Pending = requested but not yet built on this branch.

## Boards & columns

- Boards hold ordered columns; navbar switcher.
- Columns hold ordered cards; per-column collapse toggle.
- Add board / add column from navbar (appends at end).
- Pending: deep-link boards via URL.
- Pending: resizable / reorderable columns with drop hints.
- Pending: collapsed columns as narrow rails.
- Collapsed rails: count sits just under the vertical name; name hover shrinks to text.
- Pending: board/column themes + 12-swatch picker.
- Pending: board-overflow dropdown.

## Cards & blocks

- Cards hold ordered blocks: link, note (markdown), image.
- Link blocks show favicon + host.
- Image blocks via upload (jpeg/png/gif, 12MB) or URL.
- Inline note editing in place; blur commits.
- Markdown renders; re-edit shows raw text.
- Card toolbar appears on hover/focus.
- Compact add: + card per column; + link/note/image per card.
- Pending: Enter-save + modifier-newline while editing.
- Pending: drag cards within/across columns with drop hints (move API exists).
- Blocks drag via handle to reorder within/across cards (card PATCH persists).
- Block drop zones cover the whole block: hovering any part shows the before/after line (handle stays the drag source).
- Open columns hug content (no tall empty slab); lane background below + add card pans the strip.
- Board overflow menu sits outside the tab scroller, so navbar scrolling never clips it.
- Pending: reading mode.

## Navigation & chrome

- Single navbar: brand, boards, + board, + column.
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
