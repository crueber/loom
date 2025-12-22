package db

import (
	"database/sql"
	"fmt"

	"github.com/crueber/loom/internal/models"
)

// CreateList creates a new list
func (db *DB) CreateList(userID int, boardID int, title, color string, position int) (*models.List, error) {
	result, err := db.Exec(
		"INSERT INTO lists (user_id, board_id, title, color, position) VALUES (?, ?, ?, ?, ?)",
		userID, boardID, title, color, position,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create list: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get list ID: %w", err)
	}

	// Touch the board to update its updated_at timestamp
	db.TouchBoard(boardID)

	return db.GetList(int(id), userID)
}

// GetList retrieves a list by ID and user ID
func (db *DB) GetList(id, userID int) (*models.List, error) {
	var list models.List
	err := db.QueryRow(
		"SELECT id, user_id, board_id, title, color, position, collapsed, created_at FROM lists WHERE id = ? AND user_id = ?",
		id, userID,
	).Scan(&list.ID, &list.UserID, &list.BoardID, &list.Title, &list.Color, &list.Position, &list.Collapsed, &list.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get list: %w", err)
	}

	return &list, nil
}

// GetLists retrieves all lists for a user
func (db *DB) GetLists(userID int) ([]*models.List, error) {
	rows, err := db.Query(
		"SELECT id, user_id, board_id, title, color, position, collapsed, created_at FROM lists WHERE user_id = ? ORDER BY position",
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get lists: %w", err)
	}
	defer rows.Close()

	var lists []*models.List
	for rows.Next() {
		var list models.List
		if err := rows.Scan(&list.ID, &list.UserID, &list.BoardID, &list.Title, &list.Color, &list.Position, &list.Collapsed, &list.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan list: %w", err)
		}
		lists = append(lists, &list)
	}

	return lists, nil
}

// GetListsByBoard retrieves all lists for a specific board
func (db *DB) GetListsByBoard(userID int, boardID int) ([]*models.List, error) {
	rows, err := db.Query(
		"SELECT id, user_id, board_id, title, color, position, collapsed, created_at FROM lists WHERE user_id = ? AND board_id = ? ORDER BY position",
		userID, boardID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get lists: %w", err)
	}
	defer rows.Close()

	var lists []*models.List
	for rows.Next() {
		var list models.List
		if err := rows.Scan(&list.ID, &list.UserID, &list.BoardID, &list.Title, &list.Color, &list.Position, &list.Collapsed, &list.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan list: %w", err)
		}
		lists = append(lists, &list)
	}

	return lists, nil
}

// UpdateList updates a list
func (db *DB) UpdateList(id, userID int, title, color *string, collapsed *bool) error {
	query := "UPDATE lists SET "
	args := []any{}
	updates := []string{}

	if title != nil {
		updates = append(updates, "title = ?")
		args = append(args, *title)
	}
	if color != nil {
		updates = append(updates, "color = ?")
		args = append(args, *color)
	}
	if collapsed != nil {
		updates = append(updates, "collapsed = ?")
		args = append(args, *collapsed)
	}

	if len(updates) == 0 {
		return nil
	}

	query += updates[0]
	for i := 1; i < len(updates); i++ {
		query += ", " + updates[i]
	}

	query += " WHERE id = ? AND user_id = ?"
	args = append(args, id, userID)

	result, err := db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update list: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("list not found")
	}

	return nil
}

// DeleteList deletes a list
func (db *DB) DeleteList(id, userID int) error {
	result, err := db.Exec("DELETE FROM lists WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete list: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("list not found")
	}

	return nil
}

// UpdateListPositions updates positions for multiple lists
func (db *DB) UpdateListPositions(userID int, positions map[int]int) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for listID, position := range positions {
		_, err := tx.Exec("UPDATE lists SET position = ? WHERE id = ? AND user_id = ?", position, listID, userID)
		if err != nil {
			return fmt.Errorf("failed to update list position: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// MoveOrCopyListToBoard moves or copies a list (with all its items) to another board
func (db *DB) MoveOrCopyListToBoard(listID, userID, targetBoardID int, copy bool) (*models.List, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Verify list ownership and get list details
	var list models.List
	err = tx.QueryRow(
		"SELECT id, user_id, board_id, title, color, position, collapsed, created_at FROM lists WHERE id = ? AND user_id = ?",
		listID, userID,
	).Scan(&list.ID, &list.UserID, &list.BoardID, &list.Title, &list.Color, &list.Position, &list.Collapsed, &list.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("list not found")
	}

	// Verify target board ownership
	var boardExists bool
	err = tx.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM boards WHERE id = ? AND user_id = ?)",
		targetBoardID, userID,
	).Scan(&boardExists)
	if err != nil || !boardExists {
		return nil, fmt.Errorf("target board not found")
	}

	// Get the max position in target board
	var maxPosition int
	err = tx.QueryRow(
		"SELECT COALESCE(MAX(position), -1) FROM lists WHERE board_id = ?",
		targetBoardID,
	).Scan(&maxPosition)
	if err != nil {
		return nil, fmt.Errorf("failed to get max position: %w", err)
	}

	newPosition := maxPosition + 1

	if copy {
		// Create a copy of the list
		result, err := tx.Exec(
			"INSERT INTO lists (user_id, board_id, title, color, position, collapsed) VALUES (?, ?, ?, ?, ?, ?)",
			userID, targetBoardID, list.Title+" (copy)", list.Color, newPosition, list.Collapsed,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to copy list: %w", err)
		}

		newListID, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("failed to get new list ID: %w", err)
		}

		// Copy all items from the original list to the new list
		_, err = tx.Exec(
			"INSERT INTO items (list_id, type, title, url, content, favicon_url, position) SELECT ?, type, title, url, content, favicon_url, position FROM items WHERE list_id = ?",
			newListID, listID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to copy items: %w", err)
		}

		list.ID = int(newListID)
		list.BoardID = targetBoardID
		list.Title = list.Title + " (copy)"
		list.Position = newPosition
	} else {
		// Move the list
		_, err = tx.Exec(
			"UPDATE lists SET board_id = ?, position = ? WHERE id = ? AND user_id = ?",
			targetBoardID, newPosition, listID, userID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to move list: %w", err)
		}

		list.BoardID = targetBoardID
		list.Position = newPosition
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &list, nil
}

// VerifyListOwnership checks if a list belongs to a user
func (db *DB) VerifyListOwnership(listID, userID int) (bool, error) {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM lists WHERE id = ? AND user_id = ?)", listID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to verify list ownership: %w", err)
	}
	return exists, nil
}
