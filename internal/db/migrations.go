package db

import (
	"fmt"
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
		if _, err := tx.Exec(migration.sql); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %d: %w", migration.version, err)
		}

		// Record migration
		if _, err := tx.Exec("INSERT INTO migrations (version) VALUES (?)", migration.version); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %d: %w", migration.version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", migration.version, err)
		}
	}

	return nil
}

// CleanExpiredSessions removes expired sessions from the database
func (db *DB) CleanExpiredSessions() error {
	_, err := db.Exec("DELETE FROM sessions WHERE expires_at < CURRENT_TIMESTAMP")
	return err
}
