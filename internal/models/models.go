package models

import "time"

// User represents a user account
type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// List represents a collection of bookmarks
type List struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Title     string    `json:"title"`
	Color     string    `json:"color"`
	Position  int       `json:"position"`
	Collapsed bool      `json:"collapsed"`
	CreatedAt time.Time `json:"created_at"`
}

// Bookmark represents a single bookmark
type Bookmark struct {
	ID         int       `json:"id"`
	ListID     int       `json:"list_id"`
	Title      string    `json:"title"`
	URL        string    `json:"url"`
	FaviconURL *string   `json:"favicon_url,omitempty"`
	Position   int       `json:"position"`
	CreatedAt  time.Time `json:"created_at"`
}

// Session represents a user session
type Session struct {
	ID        string    `json:"id"`
	UserID    int       `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ExportData represents the structure for exporting user data
type ExportData struct {
	Version    int              `json:"version"`
	ExportedAt time.Time        `json:"exported_at"`
	Lists      []ExportList     `json:"lists"`
}

// ExportList represents a list with its bookmarks in export format
type ExportList struct {
	ID        int              `json:"id"`
	Title     string           `json:"title"`
	Color     string           `json:"color"`
	Position  int              `json:"position"`
	Collapsed bool             `json:"collapsed"`
	Bookmarks []ExportBookmark `json:"bookmarks"`
}

// ExportBookmark represents a bookmark in export format
type ExportBookmark struct {
	ID         int     `json:"id"`
	Title      string  `json:"title"`
	URL        string  `json:"url"`
	FaviconURL *string `json:"favicon_url,omitempty"`
	Position   int     `json:"position"`
}
