-- Loom rebuild v2 schema (SQLite).
-- Target DDL for the SQLite-backed store. The file-backed store in
-- store.go implements the same Store interface so the API and UI can
-- land first; swapping in this schema + a database/sql driver is the
-- explicit next step (single binary preserved either way).

CREATE TABLE IF NOT EXISTS boards (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL DEFAULT '',
  background TEXT NOT NULL DEFAULT '',
  position INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS columns (
  id TEXT PRIMARY KEY,
  board_id TEXT NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
  title TEXT NOT NULL DEFAULT '',
  color TEXT NOT NULL DEFAULT '',
  position INTEGER NOT NULL DEFAULT 0,
  collapsed INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_columns_board ON columns(board_id, position);

CREATE TABLE IF NOT EXISTS cards (
  id TEXT PRIMARY KEY,
  column_id TEXT NOT NULL REFERENCES columns(id) ON DELETE CASCADE,
  position INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_cards_column ON cards(column_id, position);

CREATE TABLE IF NOT EXISTS blocks (
  id TEXT PRIMARY KEY,
  card_id TEXT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
  type TEXT NOT NULL, -- link | note | image | todo
  position INTEGER NOT NULL DEFAULT 0,
  url TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL DEFAULT '',
  content TEXT NOT NULL DEFAULT '',
  image_url TEXT NOT NULL DEFAULT '',
  thumb_url TEXT NOT NULL DEFAULT '',
  alt TEXT NOT NULL DEFAULT '',
  checked INTEGER NOT NULL DEFAULT 0,
  items TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_blocks_card ON blocks(card_id, position);

-- Server-stored image uploads + thumbnails.
CREATE TABLE IF NOT EXISTS images (
  id TEXT PRIMARY KEY,
  content_type TEXT NOT NULL,
  size_bytes INTEGER NOT NULL DEFAULT 0,
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  thumb_width INTEGER NOT NULL DEFAULT 0,
  thumb_height INTEGER NOT NULL DEFAULT 0,
  blob BLOB NOT NULL,
  thumb_blob BLOB NOT NULL,
  created_at TEXT NOT NULL
);

-- Auth + board permissions (OSS-50, UI-configured OIDC). Additive:
-- legacy databases gain these via CREATE TABLE IF NOT EXISTS plus
-- ALTER TABLE boards in OpenSQLite; existing boards stay public/unowned.
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  issuer TEXT NOT NULL DEFAULT '',
  subject TEXT NOT NULL DEFAULT '',
  email TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_issuer_subject ON users(issuer, subject);

-- Single-row OIDC settings (id always 1).
CREATE TABLE IF NOT EXISTS auth_settings (
  id INTEGER PRIMARY KEY CHECK(id = 1),
  issuer TEXT NOT NULL DEFAULT '',
  client_id TEXT NOT NULL DEFAULT '',
  client_secret TEXT NOT NULL DEFAULT '',
  enabled INTEGER NOT NULL DEFAULT 0,
  require_auth INTEGER NOT NULL DEFAULT 0,
  oidc_backfill_done INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS board_members (
  board_id TEXT NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role TEXT NOT NULL DEFAULT 'viewer',
  PRIMARY KEY (board_id, user_id)
);

CREATE TABLE IF NOT EXISTS sessions (
  token_hash TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL DEFAULT ''
);

-- Per-user preferences (OSS-83). Absent row -> defaults.
CREATE TABLE IF NOT EXISTS user_prefs (
  user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  theme TEXT NOT NULL DEFAULT 'paper',
  language TEXT NOT NULL DEFAULT 'en'
);

-- Server-cached link icons (OSS-82 child scope, 7-day TTL).
CREATE TABLE IF NOT EXISTS icons (
  host TEXT PRIMARY KEY,
  content_type TEXT NOT NULL DEFAULT '',
  blob BLOB NOT NULL,
  fetched_at TEXT NOT NULL
);
