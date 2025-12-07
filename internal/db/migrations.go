package db

import (
	"database/sql"
	"fmt"
	"log"
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

// CleanExpiredSessions removes expired sessions from the database
func (db *DB) CleanExpiredSessions() error {
	_, err := db.Exec("DELETE FROM sessions WHERE expires_at < CURRENT_TIMESTAMP")
	return err
}
