package db

import (
	"database/sql"
	"fmt"

	"github.com/crueber/loom/internal/models"
)

// User queries

// CreateUser inserts a new user into the database
func (db *DB) CreateUser(username, passwordHash string) (*models.User, error) {
	result, err := db.Exec(
		"INSERT INTO users (username, password_hash) VALUES (?, ?)",
		username, passwordHash,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID: %w", err)
	}

	return db.GetUserByID(int(id))
}

// GetUserByID retrieves a user by ID
func (db *DB) GetUserByID(id int) (*models.User, error) {
	var user models.User
	err := db.QueryRow(
		"SELECT id, username, password_hash, created_at FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// GetUserByUsername retrieves a user by username
func (db *DB) GetUserByUsername(username string) (*models.User, error) {
	var user models.User
	err := db.QueryRow(
		"SELECT id, username, password_hash, created_at FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// DeleteUser deletes a user by username
func (db *DB) DeleteUser(username string) error {
	result, err := db.Exec("DELETE FROM users WHERE username = ?", username)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// ListUsers returns all users
func (db *DB) ListUsers() ([]*models.User, error) {
	rows, err := db.Query("SELECT id, username, password_hash, created_at FROM users ORDER BY username")
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, &user)
	}

	return users, nil
}

// UpdateUserPassword updates a user's password
func (db *DB) UpdateUserPassword(username, passwordHash string) error {
	result, err := db.Exec("UPDATE users SET password_hash = ? WHERE username = ?", passwordHash, username)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// List queries

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
	args := []interface{}{}
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

// Bookmark queries

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

// GetAllBookmarks retrieves all bookmarks for a user (across all lists)
func (db *DB) GetAllBookmarks(userID int) ([]*models.Bookmark, error) {
	rows, err := db.Query(
		`SELECT b.id, b.list_id, b.title, b.url, b.favicon_url, b.position, b.created_at
		 FROM bookmarks b
		 INNER JOIN lists l ON b.list_id = l.id
		 WHERE l.user_id = ?
		 ORDER BY b.list_id, b.position`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get all bookmarks: %w", err)
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
	args := []interface{}{}
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
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for bookmarkID, pos := range positions {
		_, err := tx.Exec("UPDATE bookmarks SET position = ?, list_id = ? WHERE id = ?", pos.Position, pos.ListID, bookmarkID)
		if err != nil {
			return fmt.Errorf("failed to update bookmark position: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
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

// Item queries (unified bookmarks and notes)

// CreateItem creates a new item (bookmark or note)
func (db *DB) CreateItem(listID int, itemType string, title, url, content *string, faviconURL *string, position int) (*models.Item, error) {
	result, err := db.Exec(
		"INSERT INTO items (list_id, type, title, url, content, favicon_url, position) VALUES (?, ?, ?, ?, ?, ?, ?)",
		listID, itemType, title, url, content, faviconURL, position,
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
		"SELECT id, list_id, type, title, url, content, favicon_url, position, created_at FROM items WHERE id = ?",
		id,
	).Scan(&item.ID, &item.ListID, &item.Type, &item.Title, &item.URL, &item.Content, &item.FaviconURL, &item.Position, &item.CreatedAt)

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
		"SELECT id, list_id, type, title, url, content, favicon_url, position, created_at FROM items WHERE list_id = ? ORDER BY position",
		listID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get items: %w", err)
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

// UpdateItem updates an item (supports partial updates)
func (db *DB) UpdateItem(id int, title, url, content *string, faviconURL **string) error {
	query := "UPDATE items SET "
	args := []interface{}{}
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
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for itemID, pos := range positions {
		_, err := tx.Exec("UPDATE items SET position = ?, list_id = ? WHERE id = ?", pos.Position, pos.ListID, itemID)
		if err != nil {
			return fmt.Errorf("failed to update item position: %w", err)
		}
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

// Board queries

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
func (db *DB) CreateBoard(userID int, title string) (*models.Board, error) {
	result, err := db.Exec(`
		INSERT INTO boards (user_id, title, is_default, updated_at, created_at)
		VALUES (?, ?, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, userID, title)
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
