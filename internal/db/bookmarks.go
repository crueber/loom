package db

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/crueber/loom/internal/models"
)

// CreateBookmark creates a new bookmark
func (db *DB) CreateBookmark(listID int, title, url string, faviconURL *string, position int) (*models.Bookmark, error) {
	result, err := db.Exec(
		"INSERT INTO bookmarks (list_id, title, url, favicon_url, position) VALUES (?, ?, ?, ?, ?)",
		listID, title, url, faviconURL, position,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create bookmark: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get bookmark ID: %w", err)
	}

	return db.GetBookmark(int(id))
}

// GetBookmark retrieves a bookmark by ID
func (db *DB) GetBookmark(id int) (*models.Bookmark, error) {
	var bookmark models.Bookmark
	err := db.QueryRow(
		"SELECT id, list_id, title, url, favicon_url, position, created_at FROM bookmarks WHERE id = ?",
		id,
	).Scan(&bookmark.ID, &bookmark.ListID, &bookmark.Title, &bookmark.URL, &bookmark.FaviconURL, &bookmark.Position, &bookmark.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get bookmark: %w", err)
	}

	return &bookmark, nil
}

// GetBookmarks retrieves all bookmarks for a list
func (db *DB) GetBookmarks(listID int) ([]*models.Bookmark, error) {
	rows, err := db.Query(
		"SELECT id, list_id, title, url, favicon_url, position, created_at FROM bookmarks WHERE list_id = ? ORDER BY position",
		listID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get bookmarks: %w", err)
	}
	defer rows.Close()

	var bookmarks []*models.Bookmark
	for rows.Next() {
		var bookmark models.Bookmark
		if err := rows.Scan(&bookmark.ID, &bookmark.ListID, &bookmark.Title, &bookmark.URL, &bookmark.FaviconURL, &bookmark.Position, &bookmark.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan bookmark: %w", err)
		}
		bookmarks = append(bookmarks, &bookmark)
	}

	return bookmarks, nil
}

// UpdateBookmark updates a bookmark
func (db *DB) UpdateBookmark(id int, title, url *string, faviconURL **string) error {
	query := "UPDATE bookmarks SET "
	args := []any{}
	updates := []string{}

	if title != nil {
		updates = append(updates, "title = ?")
		args = append(args, *title)
	}
	if url != nil {
		updates = append(updates, "url = ?")
		args = append(args, *url)
	}
	if faviconURL != nil {
		updates = append(updates, "favicon_url = ?")
		args = append(args, *faviconURL)
	}

	if len(updates) == 0 {
		return nil
	}

	query += updates[0]
	for i := 1; i < len(updates); i++ {
		query += ", " + updates[i]
	}

	query += " WHERE id = ?"
	args = append(args, id)

	result, err := db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update bookmark: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("bookmark not found")
	}

	return nil
}

// DeleteBookmark deletes a bookmark
func (db *DB) DeleteBookmark(id int) error {
	result, err := db.Exec("DELETE FROM bookmarks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete bookmark: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("bookmark not found")
	}

	return nil
}

// UpdateBookmarkPositions updates positions for multiple bookmarks
func (db *DB) UpdateBookmarkPositions(positions map[int]struct {
	Position int
	ListID   int
}) error {
	if len(positions) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Build bulk UPDATE with CASE statements for efficiency
	var ids []interface{}
	var positionCases, listIDCases strings.Builder
	positionCases.WriteString("CASE id ")
	listIDCases.WriteString("CASE id ")

	for bookmarkID, pos := range positions {
		ids = append(ids, bookmarkID)
		positionCases.WriteString(fmt.Sprintf("WHEN %d THEN %d ", bookmarkID, pos.Position))
		listIDCases.WriteString(fmt.Sprintf("WHEN %d THEN %d ", bookmarkID, pos.ListID))
	}

	positionCases.WriteString("END")
	listIDCases.WriteString("END")

	// Build placeholders for IN clause
	placeholders := make([]string, len(ids))
	for i := range ids {
		placeholders[i] = "?"
	}

	query := fmt.Sprintf(`
		UPDATE bookmarks
		SET position = %s,
		    list_id = %s
		WHERE id IN (%s)
	`, positionCases.String(), listIDCases.String(), strings.Join(placeholders, ","))

	_, err = tx.Exec(query, ids...)
	if err != nil {
		return fmt.Errorf("failed to update bookmark positions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// VerifyBookmarkOwnership checks if a bookmark belongs to a user (through the list)
func (db *DB) VerifyBookmarkOwnership(bookmarkID, userID int) (bool, error) {
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM bookmarks b
			JOIN lists l ON b.list_id = l.id
			WHERE b.id = ? AND l.user_id = ?
		)
	`, bookmarkID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to verify bookmark ownership: %w", err)
	}
	return exists, nil
}
