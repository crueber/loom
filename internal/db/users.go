package db

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/crueber/loom/internal/models"
)

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
	var email sql.NullString
	var locale sql.NullString
	var theme sql.NullString
	var oauthProvider, oauthSub sql.NullString
	err := db.QueryRow(
		"SELECT id, username, email, locale, theme, password_hash, oauth_provider, oauth_sub, created_at FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Username, &email, &locale, &theme, &user.PasswordHash, &oauthProvider, &oauthSub, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if email.Valid {
		user.Email = email.String
	}
	if locale.Valid {
		user.Locale = locale.String
	}
	if theme.Valid {
		user.Theme = theme.String
	}
	if oauthProvider.Valid {
		user.OAuthProvider = &oauthProvider.String
	}
	if oauthSub.Valid {
		user.OAuthSub = &oauthSub.String
	}

	return &user, nil
}

// EnsureStandaloneUser ensures that the default standalone user exists
func (db *DB) EnsureStandaloneUser() error {
	email := "user@standalone"
	username := "standalone"

	// Check if user already exists
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = ?)", email).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check for standalone user: %w", err)
	}

	if exists {
		return nil
	}

	// Create the user
	_, err = db.Exec(
		"INSERT INTO users (username, email, oauth_provider, oauth_sub, password_hash) VALUES (?, ?, ?, ?, '')",
		username, email, "standalone", "standalone",
	)
	if err != nil {
		return fmt.Errorf("failed to create standalone user: %w", err)
	}

	// Get the new user's ID
	var userID int
	err = db.QueryRow("SELECT id FROM users WHERE email = ?", email).Scan(&userID)
	if err != nil {
		return fmt.Errorf("failed to get standalone user ID: %w", err)
	}

	// Create default board for the user
	_, err = db.CreateBoard(userID, "My Bookmarks", true)
	if err != nil {
		return fmt.Errorf("failed to create default board for standalone user: %w", err)
	}

	log.Printf("Created default standalone user: %s", email)
	return nil
}

// GetUserByUsername retrieves a user by username
func (db *DB) GetUserByUsername(username string) (*models.User, error) {
	var user models.User
	var email sql.NullString
	var locale sql.NullString
	var theme sql.NullString
	var oauthProvider, oauthSub sql.NullString
	err := db.QueryRow(
		"SELECT id, username, email, locale, theme, password_hash, oauth_provider, oauth_sub, created_at FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &email, &locale, &theme, &user.PasswordHash, &oauthProvider, &oauthSub, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if email.Valid {
		user.Email = email.String
	}
	if locale.Valid {
		user.Locale = locale.String
	}
	if theme.Valid {
		user.Theme = theme.String
	}
	if oauthProvider.Valid {
		user.OAuthProvider = &oauthProvider.String
	}
	if oauthSub.Valid {
		user.OAuthSub = &oauthSub.String
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
	rows, err := db.Query("SELECT id, username, email, locale, theme, password_hash, oauth_provider, oauth_sub, created_at FROM users ORDER BY username")
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		var email sql.NullString
		var locale sql.NullString
		var theme sql.NullString
		var oauthProvider, oauthSub sql.NullString
		if err := rows.Scan(&user.ID, &user.Username, &email, &locale, &theme, &user.PasswordHash, &oauthProvider, &oauthSub, &user.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		if email.Valid {
			user.Email = email.String
		}
		if locale.Valid {
			user.Locale = locale.String
		}
		if theme.Valid {
			user.Theme = theme.String
		}
		if oauthProvider.Valid {
			user.OAuthProvider = &oauthProvider.String
		}
		if oauthSub.Valid {
			user.OAuthSub = &oauthSub.String
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

// GetUserByEmail retrieves a user by email address
func (db *DB) GetUserByEmail(email string) (*models.User, error) {
	var user models.User
	var locale sql.NullString
	var theme sql.NullString
	var oauthProvider, oauthSub sql.NullString
	err := db.QueryRow(
		"SELECT id, username, email, locale, theme, password_hash, oauth_provider, oauth_sub, created_at FROM users WHERE email = ?",
		email,
	).Scan(&user.ID, &user.Username, &user.Email, &locale, &theme, &user.PasswordHash, &oauthProvider, &oauthSub, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if locale.Valid {
		user.Locale = locale.String
	}
	if theme.Valid {
		user.Theme = theme.String
	}

	if oauthProvider.Valid {
		user.OAuthProvider = &oauthProvider.String
	}
	if oauthSub.Valid {
		user.OAuthSub = &oauthSub.String
	}

	return &user, nil
}

// CreateOAuthUser creates a new user from OAuth2 authentication
func (db *DB) CreateOAuthUser(email, provider, sub string) (*models.User, error) {
	// Use email as username for new OAuth users (can be changed later if needed)
	result, err := db.Exec(
		"INSERT INTO users (username, email, oauth_provider, oauth_sub, password_hash) VALUES (?, ?, ?, ?, '')",
		email, email, provider, sub,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OAuth user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID: %w", err)
	}

	return db.GetUserByID(int(id))
}
