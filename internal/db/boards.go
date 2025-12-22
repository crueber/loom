package db

import (
	"database/sql"
	"fmt"

	"github.com/crueber/loom/internal/models"
)

// GetBoards retrieves all boards for a user, sorted by most recently updated with default board first
func (db *DB) GetBoards(userID int) ([]*models.Board, error) {
	rows, err := db.Query(`
		SELECT id, user_id, title, is_default, updated_at, created_at
		FROM boards
		WHERE user_id = ?
		ORDER BY is_default DESC, updated_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get boards: %w", err)
	}
	defer rows.Close()

	var boards []*models.Board
	for rows.Next() {
		var board models.Board
		var isDefault int
		if err := rows.Scan(&board.ID, &board.UserID, &board.Title, &isDefault, &board.UpdatedAt, &board.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan board: %w", err)
		}
		board.IsDefault = isDefault == 1
		boards = append(boards, &board)
	}

	// If no boards exist, create a default board
	if len(boards) == 0 {
		defaultBoard, err := db.GetDefaultBoard(userID)
		if err != nil {
			return nil, err
		}
		boards = append(boards, defaultBoard)
	}

	return boards, nil
}

// GetBoardByID retrieves a board by ID
func (db *DB) GetBoardByID(boardID, userID int) (*models.Board, error) {
	var board models.Board
	var isDefault int
	err := db.QueryRow(`
		SELECT id, user_id, title, is_default, updated_at, created_at
		FROM boards
		WHERE id = ? AND user_id = ?
	`, boardID, userID).Scan(&board.ID, &board.UserID, &board.Title, &isDefault, &board.UpdatedAt, &board.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get board: %w", err)
	}

	board.IsDefault = isDefault == 1
	return &board, nil
}

// GetDefaultBoard retrieves the default board for a user, creating one if it doesn't exist
func (db *DB) GetDefaultBoard(userID int) (*models.Board, error) {
	var board models.Board
	var isDefault int
	err := db.QueryRow(`
		SELECT id, user_id, title, is_default, updated_at, created_at
		FROM boards
		WHERE user_id = ? AND is_default = 1
	`, userID).Scan(&board.ID, &board.UserID, &board.Title, &isDefault, &board.UpdatedAt, &board.CreatedAt)

	if err == sql.ErrNoRows {
		// Create default board
		result, err := db.Exec(`
			INSERT INTO boards (user_id, title, is_default, updated_at, created_at)
			VALUES (?, 'Default', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to create default board: %w", err)
		}

		id, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("failed to get board ID: %w", err)
		}

		return db.GetBoardByID(int(id), userID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get default board: %w", err)
	}

	board.IsDefault = isDefault == 1
	return &board, nil
}

// CreateBoard creates a new board
func (db *DB) CreateBoard(userID int, title string, isDefault bool) (*models.Board, error) {
	isDefaultInt := 0
	if isDefault {
		isDefaultInt = 1
	}
	result, err := db.Exec(`
		INSERT INTO boards (user_id, title, is_default, updated_at, created_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, userID, title, isDefaultInt)
	if err != nil {
		return nil, fmt.Errorf("failed to create board: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get board ID: %w", err)
	}

	return db.GetBoardByID(int(id), userID)
}

// UpdateBoard updates a board's title
func (db *DB) UpdateBoard(boardID, userID int, title string) error {
	result, err := db.Exec(`
		UPDATE boards
		SET title = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ?
	`, title, boardID, userID)
	if err != nil {
		return fmt.Errorf("failed to update board: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("board not found")
	}

	return nil
}

// DeleteBoard deletes a board (cannot delete default board)
func (db *DB) DeleteBoard(boardID, userID int) error {
	// Check if it's the default board
	var isDefault int
	err := db.QueryRow("SELECT is_default FROM boards WHERE id = ? AND user_id = ?", boardID, userID).Scan(&isDefault)
	if err == sql.ErrNoRows {
		return fmt.Errorf("board not found")
	}
	if err != nil {
		return fmt.Errorf("failed to check board: %w", err)
	}

	if isDefault == 1 {
		return fmt.Errorf("cannot delete default board")
	}

	result, err := db.Exec("DELETE FROM boards WHERE id = ? AND user_id = ?", boardID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete board: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("board not found")
	}

	return nil
}

// VerifyBoardOwnership checks if a board belongs to a user
func (db *DB) VerifyBoardOwnership(boardID, userID int) (bool, error) {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM boards WHERE id = ? AND user_id = ?)", boardID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to verify board ownership: %w", err)
	}
	return exists, nil
}

// TouchBoard updates the updated_at timestamp for a board
func (db *DB) TouchBoard(boardID int) error {
	_, err := db.Exec("UPDATE boards SET updated_at = CURRENT_TIMESTAMP WHERE id = ?", boardID)
	if err != nil {
		return fmt.Errorf("failed to touch board: %w", err)
	}
	return nil
}
