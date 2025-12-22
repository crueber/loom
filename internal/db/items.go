package db

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/crueber/loom/internal/models"
)

// GetNextItemPosition returns the next position for a new item in a list
func (db *DB) GetNextItemPosition(listID int) (int, error) {
	var position int
	err := db.QueryRow(
		"SELECT COALESCE(MAX(position), -1) + 1 FROM items WHERE list_id = ?",
		listID,
	).Scan(&position)
	if err != nil {
		return 0, fmt.Errorf("failed to get next item position: %w", err)
	}
	return position, nil
}

// CreateItem creates a new item (bookmark or note)
func (db *DB) CreateItem(listID int, itemType string, title, url, content *string, faviconURL *string, iconSource string, customIconURL *string, position int) (*models.Item, error) {
	result, err := db.Exec(
		"INSERT INTO items (list_id, type, title, url, content, favicon_url, icon_source, custom_icon_url, position) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		listID, itemType, title, url, content, faviconURL, iconSource, customIconURL, position,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create item: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get item ID: %w", err)
	}

	return db.GetItem(int(id))
}

// GetItem retrieves an item by ID
func (db *DB) GetItem(id int) (*models.Item, error) {
	var item models.Item
	err := db.QueryRow(
		"SELECT id, list_id, type, title, url, content, favicon_url, icon_source, custom_icon_url, position, created_at FROM items WHERE id = ?",
		id,
	).Scan(&item.ID, &item.ListID, &item.Type, &item.Title, &item.URL, &item.Content, &item.FaviconURL, &item.IconSource, &item.CustomIconURL, &item.Position, &item.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	return &item, nil
}

// GetItems retrieves all items for a list
func (db *DB) GetItems(listID int) ([]*models.Item, error) {
	rows, err := db.Query(
		"SELECT id, list_id, type, title, url, content, favicon_url, icon_source, custom_icon_url, position, created_at FROM items WHERE list_id = ? ORDER BY position",
		listID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get items: %w", err)
	}
	defer rows.Close()

	var items []*models.Item
	for rows.Next() {
		var item models.Item
		if err := rows.Scan(&item.ID, &item.ListID, &item.Type, &item.Title, &item.URL, &item.Content, &item.FaviconURL, &item.IconSource, &item.CustomIconURL, &item.Position, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}
		items = append(items, &item)
	}

	return items, nil
}

// GetAllItems retrieves all items for a user (across all lists)
func (db *DB) GetAllItems(userID int) ([]*models.Item, error) {
	rows, err := db.Query(
		`SELECT i.id, i.list_id, i.type, i.title, i.url, i.content, i.favicon_url, i.position, i.created_at
		 FROM items i
		 INNER JOIN lists l ON i.list_id = l.id
		 WHERE l.user_id = ?
		 ORDER BY i.list_id, i.position`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get all items: %w", err)
	}
	defer rows.Close()

	var items []*models.Item
	for rows.Next() {
		var item models.Item
		if err := rows.Scan(&item.ID, &item.ListID, &item.Type, &item.Title, &item.URL, &item.Content, &item.FaviconURL, &item.Position, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}
		items = append(items, &item)
	}

	return items, nil
}

// GetItemsByBoard retrieves all items for a specific board
func (db *DB) GetItemsByBoard(userID, boardID int) ([]*models.Item, error) {
	rows, err := db.Query(
		`SELECT i.id, i.list_id, i.type, i.title, i.url, i.content, i.favicon_url, i.icon_source, i.custom_icon_url, i.position, i.created_at
		 FROM items i
		 INNER JOIN lists l ON i.list_id = l.id
		 WHERE l.user_id = ? AND l.board_id = ?
		 ORDER BY i.list_id, i.position`,
		userID, boardID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get items by board: %w", err)
	}
	defer rows.Close()

	var items []*models.Item
	for rows.Next() {
		var item models.Item
		if err := rows.Scan(&item.ID, &item.ListID, &item.Type, &item.Title, &item.URL, &item.Content, &item.FaviconURL, &item.IconSource, &item.CustomIconURL, &item.Position, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}
		items = append(items, &item)
	}

	return items, nil
}

// UpdateItem updates an item (supports partial updates)
func (db *DB) UpdateItem(id int, title, url, content *string, faviconURL **string) error {
	query := "UPDATE items SET "
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
	if content != nil {
		updates = append(updates, "content = ?")
		args = append(args, *content)
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
		return fmt.Errorf("failed to update item: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("item not found")
	}

	return nil
}

// UpdateItemFields updates an item using a map of field names to values
func (db *DB) UpdateItemFields(id int, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}

	query := "UPDATE items SET "
	args := []any{}
	updates := []string{}

	// Allowed fields for update
	allowedFields := map[string]bool{
		"title":           true,
		"url":             true,
		"content":         true,
		"favicon_url":     true,
		"icon_source":     true,
		"custom_icon_url": true,
	}

	for field, value := range fields {
		if !allowedFields[field] {
			continue
		}
		updates = append(updates, field+" = ?")
		args = append(args, value)
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
		return fmt.Errorf("failed to update item: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("item not found")
	}

	return nil
}

// DeleteItem deletes an item
func (db *DB) DeleteItem(id int) error {
	result, err := db.Exec("DELETE FROM items WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("item not found")
	}

	return nil
}

// UpdateItemPositions updates positions for multiple items
func (db *DB) UpdateItemPositions(positions map[int]struct {
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

	for itemID, pos := range positions {
		ids = append(ids, itemID)
		positionCases.WriteString(fmt.Sprintf("WHEN %d THEN %d ", itemID, pos.Position))
		listIDCases.WriteString(fmt.Sprintf("WHEN %d THEN %d ", itemID, pos.ListID))
	}

	positionCases.WriteString("END")
	listIDCases.WriteString("END")

	// Build placeholders for IN clause
	placeholders := make([]string, len(ids))
	for i := range ids {
		placeholders[i] = "?"
	}

	query := fmt.Sprintf(`
		UPDATE items
		SET position = %s,
		    list_id = %s
		WHERE id IN (%s)
	`, positionCases.String(), listIDCases.String(), strings.Join(placeholders, ","))

	_, err = tx.Exec(query, ids...)
	if err != nil {
		return fmt.Errorf("failed to update item positions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// VerifyItemOwnership checks if an item belongs to a user (through the list)
func (db *DB) VerifyItemOwnership(itemID, userID int) (bool, error) {
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM items i
			JOIN lists l ON i.list_id = l.id
			WHERE i.id = ? AND l.user_id = ?
		)
	`, itemID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to verify item ownership: %w", err)
	}
	return exists, nil
}

// VerifyItemsAndListsOwnership verifies that all items and lists belong to a user in a single query
func (db *DB) VerifyItemsAndListsOwnership(itemIDs []int, listIDs []int, userID int) (bool, error) {
	if len(itemIDs) == 0 {
		return true, nil
	}

	// Build placeholders for IN clauses
	itemPlaceholders := make([]string, len(itemIDs))
	listPlaceholders := make([]string, len(listIDs))
	for i := range itemIDs {
		itemPlaceholders[i] = "?"
	}
	for i := range listIDs {
		listPlaceholders[i] = "?"
	}

	// Build query to verify all items belong to user and all target lists belong to user
	query := fmt.Sprintf(`
		SELECT
			(SELECT COUNT(DISTINCT i.id) FROM items i
			 JOIN lists l ON i.list_id = l.id
			 WHERE i.id IN (%s) AND l.user_id = ?) as item_count,
			(SELECT COUNT(DISTINCT l.id) FROM lists l
			 WHERE l.id IN (%s) AND l.user_id = ?) as list_count
	`, strings.Join(itemPlaceholders, ","), strings.Join(listPlaceholders, ","))

	// Build args slice
	args := make([]interface{}, 0, len(itemIDs)+len(listIDs)+2)
	for _, id := range itemIDs {
		args = append(args, id)
	}
	args = append(args, userID)
	for _, id := range listIDs {
		args = append(args, id)
	}
	args = append(args, userID)

	var itemCount, listCount int
	err := db.QueryRow(query, args...).Scan(&itemCount, &listCount)
	if err != nil {
		return false, fmt.Errorf("failed to verify ownership: %w", err)
	}

	return itemCount == len(itemIDs) && listCount == len(listIDs), nil
}
