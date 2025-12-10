package db

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// migrate runs all database migrations
func (db *DB) migrate() error {
	// Create migrations table if it doesn't exist
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			id INTEGER PRIMARY KEY,
			version INTEGER UNIQUE NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Define migrations
	migrations := []struct {
		version int
		sql     string
	}{
		{
			version: 1,
			sql: `
				CREATE TABLE IF NOT EXISTS users (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					username TEXT UNIQUE NOT NULL,
					password_hash TEXT NOT NULL,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				);

				CREATE TABLE IF NOT EXISTS lists (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					user_id INTEGER NOT NULL,
					title TEXT NOT NULL,
					color TEXT NOT NULL,
					position INTEGER NOT NULL,
					collapsed INTEGER DEFAULT 0,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
				);

				CREATE TABLE IF NOT EXISTS bookmarks (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					list_id INTEGER NOT NULL,
					title TEXT NOT NULL,
					url TEXT NOT NULL,
					favicon_url TEXT,
					position INTEGER NOT NULL,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (list_id) REFERENCES lists(id) ON DELETE CASCADE
				);

				CREATE TABLE IF NOT EXISTS sessions (
					id TEXT PRIMARY KEY,
					user_id INTEGER NOT NULL,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					expires_at TIMESTAMP NOT NULL,
					FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_lists_user_id ON lists(user_id);
				CREATE INDEX IF NOT EXISTS idx_lists_position ON lists(user_id, position);
				CREATE INDEX IF NOT EXISTS idx_bookmarks_list_id ON bookmarks(list_id);
				CREATE INDEX IF NOT EXISTS idx_bookmarks_position ON bookmarks(list_id, position);
				CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
				CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
			`,
		},
		{
			version: 2,
			sql: `
				-- Create boards table
				CREATE TABLE IF NOT EXISTS boards (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					user_id INTEGER NOT NULL,
					title TEXT NOT NULL CHECK(length(title) <= 100),
					is_default INTEGER NOT NULL DEFAULT 0,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_boards_user_id ON boards(user_id);
				CREATE UNIQUE INDEX IF NOT EXISTS idx_boards_default ON boards(user_id, is_default) WHERE is_default = 1;

				-- Add board_id column to lists (nullable initially for migration)
				ALTER TABLE lists ADD COLUMN board_id INTEGER;

				CREATE INDEX IF NOT EXISTS idx_lists_board_id ON lists(board_id);
			`,
		},
		{
			version: 3,
			sql: `
				-- Create new items table with support for both bookmarks and notes
				CREATE TABLE IF NOT EXISTS items (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					list_id INTEGER NOT NULL,
					type TEXT NOT NULL CHECK(type IN ('bookmark', 'note')),
					title TEXT,
					url TEXT,
					content TEXT,
					favicon_url TEXT,
					position INTEGER NOT NULL,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (list_id) REFERENCES lists(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_items_list_id ON items(list_id);
				CREATE INDEX IF NOT EXISTS idx_items_position ON items(list_id, position);
				CREATE INDEX IF NOT EXISTS idx_items_type ON items(list_id, type);
			`,
		},
		{
			version: 4,
			sql: `
				-- Migration v4: Convert Google favicon URLs to Base64 data URIs
				-- This is handled by the migrateDataForFaviconsV4 function
				-- No schema changes needed - just data conversion
			`,
		},
	}

	// Run each migration
	for _, migration := range migrations {
		// Check if migration has already been applied
		var exists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM migrations WHERE version = ?)", migration.version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check migration version %d: %w", migration.version, err)
		}

		if exists {
			continue
		}

		// Run migration in a transaction
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %d: %w", migration.version, err)
		}

		// Execute migration SQL
		log.Printf("Running migration version %d...", migration.version)
		if _, err := tx.Exec(migration.sql); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %d: %w", migration.version, err)
		}

		// Run post-migration data migrations
		if migration.version == 2 {
			if err := db.migrateDataForBoardsV2(tx); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to migrate data for version %d: %w", migration.version, err)
			}
		}
		if migration.version == 3 {
			if err := db.migrateDataForItemsV3(tx); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to migrate data for version %d: %w", migration.version, err)
			}
		}
		if migration.version == 4 {
			if err := db.migrateDataForFaviconsV4(tx); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to migrate data for version %d: %w", migration.version, err)
			}
		}

		// Record migration
		if _, err := tx.Exec("INSERT INTO migrations (version) VALUES (?)", migration.version); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %d: %w", migration.version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", migration.version, err)
		}

		log.Printf("Migration version %d completed successfully", migration.version)
	}

	return nil
}

// migrateDataForBoardsV2 creates default boards for all users and migrates existing lists
func (db *DB) migrateDataForBoardsV2(tx *sql.Tx) error {
	log.Println("  Creating default boards for all users...")

	// Get all users
	rows, err := tx.Query("SELECT id, username FROM users")
	if err != nil {
		return fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	userCount := 0
	for rows.Next() {
		var userID int
		var username string
		if err := rows.Scan(&userID, &username); err != nil {
			return fmt.Errorf("failed to scan user: %w", err)
		}

		// Create default board for this user
		result, err := tx.Exec(`
			INSERT INTO boards (user_id, title, is_default, updated_at, created_at)
			VALUES (?, 'Default', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`, userID)
		if err != nil {
			return fmt.Errorf("failed to create default board for user %d: %w", userID, err)
		}

		boardID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get board ID: %w", err)
		}

		// Update all lists for this user to belong to the default board
		updateResult, err := tx.Exec(`
			UPDATE lists SET board_id = ? WHERE user_id = ?
		`, boardID, userID)
		if err != nil {
			return fmt.Errorf("failed to update lists for user %d: %w", userID, err)
		}

		listCount, _ := updateResult.RowsAffected()
		log.Printf("  User '%s' (ID %d): Created default board (ID %d), migrated %d lists",
			username, userID, boardID, listCount)
		userCount++
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating users: %w", err)
	}

	log.Printf("  Migration complete: Created default boards for %d users", userCount)

	// Now make board_id NOT NULL since all lists should have a board_id
	// Note: SQLite doesn't support ALTER COLUMN directly, but since we've populated all rows,
	// future inserts will require board_id via application logic

	return nil
}

// migrateDataForItemsV3 migrates bookmarks to items table
func (db *DB) migrateDataForItemsV3(tx *sql.Tx) error {
	log.Println("  Migrating bookmarks to items table...")

	// Check if bookmarks table exists
	var tableExists bool
	err := tx.QueryRow(`
		SELECT COUNT(*) > 0
		FROM sqlite_master
		WHERE type='table' AND name='bookmarks'
	`).Scan(&tableExists)
	if err != nil {
		return fmt.Errorf("failed to check if bookmarks table exists: %w", err)
	}

	if !tableExists {
		log.Println("  Bookmarks table does not exist, skipping migration")
		return nil
	}

	// Copy only bookmarks that have valid list_id references (no orphaned bookmarks)
	result, err := tx.Exec(`
		INSERT INTO items (id, list_id, type, title, url, favicon_url, position, created_at)
		SELECT b.id, b.list_id, 'bookmark', b.title, b.url, b.favicon_url, b.position, b.created_at
		FROM bookmarks b
		INNER JOIN lists l ON b.list_id = l.id
	`)
	if err != nil {
		return fmt.Errorf("failed to copy bookmarks to items: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("  Migrated %d bookmarks to items table", rowsAffected)

	// Verify counts match (only count valid bookmarks with list references)
	var bookmarkCount, itemCount int
	if err := tx.QueryRow(`
		SELECT COUNT(*)
		FROM bookmarks b
		INNER JOIN lists l ON b.list_id = l.id
	`).Scan(&bookmarkCount); err != nil {
		return fmt.Errorf("failed to count valid bookmarks: %w", err)
	}
	if err := tx.QueryRow("SELECT COUNT(*) FROM items WHERE type = 'bookmark'").Scan(&itemCount); err != nil {
		return fmt.Errorf("failed to count items: %w", err)
	}

	if bookmarkCount != itemCount {
		return fmt.Errorf("data integrity check failed: valid bookmarks count (%d) != items count (%d)", bookmarkCount, itemCount)
	}

	log.Printf("  Data integrity verified: %d bookmarks successfully migrated", bookmarkCount)

	// Drop the old bookmarks table
	if _, err := tx.Exec("DROP TABLE bookmarks"); err != nil {
		return fmt.Errorf("failed to drop bookmarks table: %w", err)
	}

	log.Println("  Dropped old bookmarks table")
	log.Println("  Migration to items table complete")

	return nil
}

// migrateDataForFaviconsV4 converts existing Google favicon URLs to Base64 data URIs
func (db *DB) migrateDataForFaviconsV4(tx *sql.Tx) error {
	log.Println("  Converting Google favicon URLs to Base64 data URIs...")

	// Get all items with Google favicon URLs
	rows, err := tx.Query(`
		SELECT id, favicon_url
		FROM items
		WHERE favicon_url IS NOT NULL
		AND favicon_url LIKE 'https://www.google.com/s2/favicons%'
	`)
	if err != nil {
		return fmt.Errorf("failed to query items with favicon URLs: %w", err)
	}
	defer rows.Close()

	type itemWithFavicon struct {
		id         int
		faviconURL string
	}

	var itemsToUpdate []itemWithFavicon
	for rows.Next() {
		var item itemWithFavicon
		if err := rows.Scan(&item.id, &item.faviconURL); err != nil {
			return fmt.Errorf("failed to scan item: %w", err)
		}
		itemsToUpdate = append(itemsToUpdate, item)
	}

	if len(itemsToUpdate) == 0 {
		log.Println("  No Google favicon URLs to convert")
		return nil
	}

	log.Printf("  Found %d items with Google favicon URLs to convert", len(itemsToUpdate))

	// Create HTTP client for fetching favicons
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	converted := 0
	failed := 0

	for _, item := range itemsToUpdate {
		// Fetch the favicon
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		req, err := http.NewRequestWithContext(ctx, "GET", item.faviconURL, nil)
		if err != nil {
			cancel()
			failed++
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			cancel()
			failed++
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			cancel()
			failed++
			continue
		}

		// Get content type before reading body
		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "image/png"
		}

		// Read favicon bytes
		faviconBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()

		if err != nil || len(faviconBytes) < 100 {
			failed++
			continue
		}

		// Encode to Base64
		encoded := base64.StdEncoding.EncodeToString(faviconBytes)
		dataURI := fmt.Sprintf("data:%s;base64,%s", contentType, encoded)

		// Update the item
		if _, err := tx.Exec("UPDATE items SET favicon_url = ? WHERE id = ?", dataURI, item.id); err != nil {
			return fmt.Errorf("failed to update item %d: %w", item.id, err)
		}

		converted++
	}

	log.Printf("  Converted %d favicons successfully, %d failed", converted, failed)
	return nil
}

// CleanExpiredSessions removes expired sessions from the database
func (db *DB) CleanExpiredSessions() error {
	_, err := db.Exec("DELETE FROM sessions WHERE expires_at < CURRENT_TIMESTAMP")
	return err
}
