// SQLite-backed Store implementation (cgo via mattn/go-sqlite3).
// Same Store interface as the file backend; schema matches schema.sql.
// Selected automatically when --data points at a .db/.sqlite/.sqlite3 file.
package store

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/crueber/loom-rebuild/internal/model"
)

//go:embed schema.sql
var sqliteSchema string

// SQLiteStore persists boards/columns/cards/blocks in SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// OpenSQLite opens (creating if needed) the SQLite database at path
// and applies the schema.
func OpenSQLite(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", path+"?cache=shared&mode=rwc&_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(sqliteSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	// Migrate pre-background databases: ignore duplicate-column errors.
	_, _ = db.Exec(`ALTER TABLE boards ADD COLUMN background TEXT NOT NULL DEFAULT ''`)
	// Migrate pre-todo databases: ignore duplicate-column errors.
	_, _ = db.Exec(`ALTER TABLE blocks ADD COLUMN checked INTEGER NOT NULL DEFAULT 0`)
	// Migrate pre-auth databases (OSS-50): same ignore-if-exists pattern.
	_, _ = db.Exec(`ALTER TABLE boards ADD COLUMN owner_id TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE boards ADD COLUMN visibility TEXT NOT NULL DEFAULT 'public'`)
	_, _ = db.Exec(`INSERT OR IGNORE INTO auth_settings(id) VALUES(1)`)
	// Migrate pre-backfill databases (OSS-71): first-sign-in claim flag.
	_, _ = db.Exec(`ALTER TABLE auth_settings ADD COLUMN oidc_backfill_done INTEGER NOT NULL DEFAULT 0`)
	// Migrate pre-checklist databases: todo items live as JSON here.
	_, _ = db.Exec(`ALTER TABLE blocks ADD COLUMN items TEXT NOT NULL DEFAULT ''`)
	// Migrate pre-prefs databases (OSS-83): per-user preferences.
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS user_prefs (user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE, theme TEXT NOT NULL DEFAULT 'paper', language TEXT NOT NULL DEFAULT 'en')`)
	return &SQLiteStore{db: db}, nil
}

// Close releases the database handle.
func (s *SQLiteStore) Close() error { return s.db.Close() }

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTS(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

func (s *SQLiteStore) ListBoards() ([]model.Board, error) {
	rows, err := s.db.Query(`SELECT id,title,COALESCE(background,''),position,COALESCE(owner_id,''),COALESCE(visibility,'public'),created_at,updated_at FROM boards ORDER BY position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Board
	for rows.Next() {
		var b model.Board
		var ca, ua string
		if err := rows.Scan(&b.ID, &b.Title, &b.Background, &b.Position, &b.OwnerID, &b.Visibility, &ca, &ua); err != nil {
			return nil, err
		}
		b.CreatedAt, b.UpdatedAt = parseTS(ca), parseTS(ua)
		out = append(out, b)
	}
	if out == nil {
		out = []model.Board{}
	}
	return out, rows.Err()
}

func (s *SQLiteStore) CreateBoard(title string) (model.Board, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM boards`).Scan(&n); err != nil {
		return model.Board{}, err
	}
	b := model.NewBoard(title, n)
	b.Visibility = model.VisibilityPublic
	_, err := s.db.Exec(`INSERT INTO boards(id,title,background,position,owner_id,visibility,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`,
		b.ID, b.Title, b.Background, b.Position, b.OwnerID, b.Visibility, ts(b.CreatedAt), ts(b.UpdatedAt))
	return b, err
}

func (s *SQLiteStore) getBoard(id string) (model.Board, error) {
	var b model.Board
	var ca, ua string
	err := s.db.QueryRow(`SELECT id,title,COALESCE(background,''),position,COALESCE(owner_id,''),COALESCE(visibility,'public'),created_at,updated_at FROM boards WHERE id=?`, id).
		Scan(&b.ID, &b.Title, &b.Background, &b.Position, &b.OwnerID, &b.Visibility, &ca, &ua)
	if err == sql.ErrNoRows {
		return b, fmt.Errorf("board not found")
	}
	b.CreatedAt, b.UpdatedAt = parseTS(ca), parseTS(ua)
	return b, err
}

func (s *SQLiteStore) listColumns(boardID string) ([]model.Column, error) {
	rows, err := s.db.Query(`SELECT id,board_id,title,color,position,collapsed,created_at FROM columns WHERE board_id=? ORDER BY position`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Column
	for rows.Next() {
		var c model.Column
		var collapsed int
		var ca string
		if err := rows.Scan(&c.ID, &c.BoardID, &c.Title, &c.Color, &c.Position, &collapsed, &ca); err != nil {
			return nil, err
		}
		c.Collapsed = collapsed != 0
		c.CreatedAt = parseTS(ca)
		out = append(out, c)
	}
	if out == nil {
		out = []model.Column{}
	}
	return out, rows.Err()
}

func (s *SQLiteStore) listCards(columnID string) ([]model.Card, error) {
	rows, err := s.db.Query(`SELECT id,column_id,position,created_at,updated_at FROM cards WHERE column_id=? ORDER BY position`, columnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Card
	for rows.Next() {
		var c model.Card
		var ca, ua string
		if err := rows.Scan(&c.ID, &c.ColumnID, &c.Position, &ca, &ua); err != nil {
			return nil, err
		}
		c.CreatedAt, c.UpdatedAt = parseTS(ca), parseTS(ua)
		blocks, err := s.listBlocks(c.ID)
		if err != nil {
			return nil, err
		}
		c.Blocks = blocks
		out = append(out, c)
	}
	if out == nil {
		out = []model.Card{}
	}
	return out, rows.Err()
}

func (s *SQLiteStore) listBlocks(cardID string) ([]model.Block, error) {
	rows, err := s.db.Query(`SELECT id,type,position,url,title,content,image_url,thumb_url,alt,COALESCE(checked,0),COALESCE(items,'') FROM blocks WHERE card_id=? ORDER BY position`, cardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Block
	for rows.Next() {
		var b model.Block
		var checked int
		var itemsRaw string
		if err := rows.Scan(&b.ID, &b.Type, &b.Position, &b.URL, &b.Title, &b.Content, &b.ImageURL, &b.ThumbURL, &b.Alt, &checked, &itemsRaw); err != nil {
			return nil, err
		}
		b.Checked = checked != 0
		if itemsRaw != "" {
			_ = json.Unmarshal([]byte(itemsRaw), &b.Items)
		}
		out = append(out, b)
	}
	if out == nil {
		out = []model.Block{}
	}
	// Migrate consecutive legacy todo rows into single checklist blocks.
	out = model.NormalizeTodoBlocks(out)
	return out, rows.Err()
}

func (s *SQLiteStore) GetTree(boardID string) (model.BoardTree, error) {
	b, err := s.getBoard(boardID)
	if err != nil {
		return model.BoardTree{}, err
	}
	cols, err := s.listColumns(boardID)
	if err != nil {
		return model.BoardTree{}, err
	}
	tree := model.BoardTree{Board: b, Columns: cols, Cards: map[string][]model.Card{}}
	if tree.Columns == nil {
		tree.Columns = []model.Column{}
	}
	for _, c := range cols {
		cards, err := s.listCards(c.ID)
		if err != nil {
			return model.BoardTree{}, err
		}
		tree.Cards[c.ID] = cards
	}
	return tree, nil
}

func (s *SQLiteStore) UpdateBoard(id, title, background string) (model.Board, error) {
	res, err := s.db.Exec(`UPDATE boards SET title=?, background=?, updated_at=? WHERE id=?`, title, background, ts(time.Now().UTC()), id)
	if err != nil {
		return model.Board{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.Board{}, fmt.Errorf("board not found")
	}
	return s.getBoard(id)
}

func (s *SQLiteStore) DeleteBoard(id string) error {
	res, err := s.db.Exec(`DELETE FROM boards WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("board not found")
	}
	return nil
}

func (s *SQLiteStore) CreateColumn(boardID, title, color string) (model.Column, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM columns WHERE board_id=?`, boardID).Scan(&n); err != nil {
		return model.Column{}, err
	}
	col := model.NewColumn(boardID, title, color, n)
	_, err := s.db.Exec(`INSERT INTO columns(id,board_id,title,color,position,collapsed,created_at) VALUES(?,?,?,?,?,?,?)`,
		col.ID, col.BoardID, col.Title, col.Color, col.Position, 0, ts(col.CreatedAt))
	return col, err
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *SQLiteStore) UpdateColumn(col model.Column) (model.Column, error) {
	res, err := s.db.Exec(`UPDATE columns SET title=?, color=?, position=?, collapsed=? WHERE id=?`,
		col.Title, col.Color, col.Position, boolInt(col.Collapsed), col.ID)
	if err != nil {
		return model.Column{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.Column{}, fmt.Errorf("column not found")
	}
	var updated model.Column
	var collapsed int
	var ca string
	err = s.db.QueryRow(`SELECT id,board_id,title,color,position,collapsed,created_at FROM columns WHERE id=?`, col.ID).
		Scan(&updated.ID, &updated.BoardID, &updated.Title, &updated.Color, &updated.Position, &collapsed, &ca)
	updated.Collapsed = collapsed != 0
	updated.CreatedAt = parseTS(ca)
	return updated, err
}

func (s *SQLiteStore) DeleteColumn(id string) error {
	res, err := s.db.Exec(`DELETE FROM columns WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("column not found")
	}
	return nil
}

func insertBlock(tx *sql.Tx, cardID string, b model.Block, pos int) error {
	if b.ID == "" {
		b.ID = model.NewID()
	}
	var itemsRaw string
	if len(b.Items) > 0 {
		raw, err := json.Marshal(b.Items)
		if err != nil {
			return err
		}
		itemsRaw = string(raw)
	}
	_, err := tx.Exec(`INSERT INTO blocks(id,card_id,type,position,url,title,content,image_url,thumb_url,alt,checked,items) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		b.ID, cardID, b.Type, pos, b.URL, b.Title, b.Content, b.ImageURL, b.ThumbURL, b.Alt, boolInt(b.Checked), itemsRaw)
	return err
}

func (s *SQLiteStore) CreateCard(columnID string, blocks []model.Block) (model.Card, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM cards WHERE column_id=?`, columnID).Scan(&n); err != nil {
		return model.Card{}, err
	}
	blocks = model.NormalizeTodoBlocks(blocks)
	card := model.NewCard(columnID, n, blocks)
	tx, err := s.db.Begin()
	if err != nil {
		return model.Card{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO cards(id,column_id,position,created_at,updated_at) VALUES(?,?,?,?,?)`,
		card.ID, card.ColumnID, card.Position, ts(card.CreatedAt), ts(card.UpdatedAt)); err != nil {
		return model.Card{}, err
	}
	for i, b := range card.Blocks {
		if err := insertBlock(tx, card.ID, b, i); err != nil {
			return model.Card{}, err
		}
	}
	return card, tx.Commit()
}

func (s *SQLiteStore) UpdateCard(card model.Card) (model.Card, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return model.Card{}, err
	}
	defer tx.Rollback()
	var existingCol string
	err = tx.QueryRow(`SELECT column_id FROM cards WHERE id=?`, card.ID).Scan(&existingCol)
	if err == sql.ErrNoRows {
		return model.Card{}, fmt.Errorf("card not found")
	}
	if err != nil {
		return model.Card{}, err
	}
	card.ColumnID = existingCol
	if _, err := tx.Exec(`DELETE FROM blocks WHERE card_id=?`, card.ID); err != nil {
		return model.Card{}, err
	}
	card.Blocks = model.NormalizeTodoBlocks(card.Blocks)
	for i := range card.Blocks {
		if card.Blocks[i].ID == "" {
			card.Blocks[i].ID = model.NewID()
		}
		card.Blocks[i].Position = i
		if err := insertBlock(tx, card.ID, card.Blocks[i], i); err != nil {
			return model.Card{}, err
		}
	}
	if _, err := tx.Exec(`UPDATE cards SET updated_at=? WHERE id=?`, ts(time.Now().UTC()), card.ID); err != nil {
		return model.Card{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Card{}, err
	}
	cards, err := s.listCards(card.ColumnID)
	if err != nil {
		return model.Card{}, err
	}
	for _, c := range cards {
		if c.ID == card.ID {
			return c, nil
		}
	}
	return model.Card{}, fmt.Errorf("card not found")
}

func (s *SQLiteStore) DeleteCard(id string) error {
	res, err := s.db.Exec(`DELETE FROM cards WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("card not found")
	}
	return nil
}

func (s *SQLiteStore) MoveCard(id, toColumnID string, position int) (model.Card, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return model.Card{}, err
	}
	defer tx.Rollback()
	var fromCol string
	var fromPos int
	err = tx.QueryRow(`SELECT column_id, position FROM cards WHERE id=?`, id).Scan(&fromCol, &fromPos)
	if err == sql.ErrNoRows {
		return model.Card{}, fmt.Errorf("card not found")
	}
	if err != nil {
		return model.Card{}, err
	}
	if _, err := tx.Exec(`UPDATE cards SET position = position - 1 WHERE column_id=? AND position > ?`, fromCol, fromPos); err != nil {
		return model.Card{}, err
	}
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM cards WHERE column_id=?`, toColumnID).Scan(&n); err != nil {
		return model.Card{}, err
	}
	if position < 0 || position > n {
		position = n
	}
	if _, err := tx.Exec(`UPDATE cards SET position = position + 1 WHERE column_id=? AND position >= ?`, toColumnID, position); err != nil {
		return model.Card{}, err
	}
	if _, err := tx.Exec(`UPDATE cards SET column_id=?, position=?, updated_at=? WHERE id=?`, toColumnID, position, ts(time.Now().UTC()), id); err != nil {
		return model.Card{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Card{}, err
	}
	cards, err := s.listCards(toColumnID)
	if err != nil {
		return model.Card{}, err
	}
	for _, c := range cards {
		if c.ID == id {
			return c, nil
		}
	}
	return model.Card{}, fmt.Errorf("card not found")
}

func (s *SQLiteStore) ImportTree(tree model.BoardTree) (model.BoardTree, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return model.BoardTree{}, err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM boards`).Scan(&n); err != nil {
		return model.BoardTree{}, err
	}
	tree.Board.Position = n
	if tree.Board.Visibility == "" {
		tree.Board.Visibility = model.VisibilityPublic
	}
	if _, err := tx.Exec(`INSERT INTO boards(id,title,background,position,owner_id,visibility,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`,
		tree.Board.ID, tree.Board.Title, tree.Board.Background, tree.Board.Position, tree.Board.OwnerID, tree.Board.Visibility, ts(tree.Board.CreatedAt), ts(tree.Board.UpdatedAt)); err != nil {
		return model.BoardTree{}, err
	}
	for _, c := range tree.Columns {
		if _, err := tx.Exec(`INSERT INTO columns(id,board_id,title,color,position,collapsed,created_at) VALUES(?,?,?,?,?,?,?)`,
			c.ID, tree.Board.ID, c.Title, c.Color, c.Position, boolInt(c.Collapsed), ts(c.CreatedAt)); err != nil {
			return model.BoardTree{}, err
		}
	}
	for colID, cards := range tree.Cards {
		for _, card := range cards {
			if _, err := tx.Exec(`INSERT INTO cards(id,column_id,position,created_at,updated_at) VALUES(?,?,?,?,?)`,
				card.ID, colID, card.Position, ts(card.CreatedAt), ts(card.UpdatedAt)); err != nil {
				return model.BoardTree{}, err
			}
			for i, b := range card.Blocks {
				if err := insertBlock(tx, card.ID, b, i); err != nil {
					return model.BoardTree{}, err
				}
			}
		}
	}
	return tree, tx.Commit()
}

// ImageRecord is one stored upload plus its thumbnail.
type ImageRecord struct {
	ID          string
	ContentType string
	SizeBytes   int
	Width       int
	Height      int
	ThumbWidth  int
	ThumbHeight int
	Blob        []byte
	ThumbBlob   []byte
	CreatedAt   time.Time
}

// SaveImage stores an upload and its thumbnail, returning the id.
func (s *SQLiteStore) SaveImage(contentType string, w, h, tw, th int, blob, thumb []byte) (ImageRecord, error) {
	rec := ImageRecord{
		ID: model.NewID(), ContentType: contentType,
		SizeBytes: len(blob), Width: w, Height: h,
		ThumbWidth: tw, ThumbHeight: th,
		Blob: blob, ThumbBlob: thumb, CreatedAt: time.Now().UTC(),
	}
	_, err := s.db.Exec(`INSERT INTO images(id,content_type,size_bytes,width,height,thumb_width,thumb_height,blob,thumb_blob,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		rec.ID, rec.ContentType, rec.SizeBytes, rec.Width, rec.Height, rec.ThumbWidth, rec.ThumbHeight, rec.Blob, rec.ThumbBlob, ts(rec.CreatedAt))
	return rec, err
}

// GetImage loads an upload by id.
func (s *SQLiteStore) GetImage(id string) (ImageRecord, error) {
	var rec ImageRecord
	var ca string
	err := s.db.QueryRow(`SELECT id,content_type,size_bytes,width,height,thumb_width,thumb_height,blob,thumb_blob,created_at FROM images WHERE id=?`, id).
		Scan(&rec.ID, &rec.ContentType, &rec.SizeBytes, &rec.Width, &rec.Height, &rec.ThumbWidth, &rec.ThumbHeight, &rec.Blob, &rec.ThumbBlob, &ca)
	if err == sql.ErrNoRows {
		return rec, fmt.Errorf("image not found")
	}
	rec.CreatedAt = parseTS(ca)
	return rec, err
}

// SQLiteStore satisfies the Store interface.
var _ Store = (*SQLiteStore)(nil)
