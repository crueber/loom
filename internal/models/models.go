package models

import "time"

// User represents a user account
type User struct {
	ID            int       `json:"id"`
	Username      string    `json:"username"`
	Email         string    `json:"email"`
	PasswordHash  string    `json:"-"`
	OAuthProvider *string   `json:"oauth_provider,omitempty"`
	OAuthSub      *string   `json:"oauth_sub,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// Board represents a collection of lists
type Board struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Title     string    `json:"title"`
	IsDefault bool      `json:"is_default"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedAt time.Time `json:"created_at"`
}

// List represents a collection of bookmarks
type List struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	BoardID   int       `json:"board_id"`
	Title     string    `json:"title"`
	Color     string    `json:"color"`
	Position  int       `json:"position"`
	Collapsed bool      `json:"collapsed"`
	CreatedAt time.Time `json:"created_at"`
}

// Item represents a single item (bookmark or note)
type Item struct {
	ID         int       `json:"id"`
	ListID     int       `json:"list_id"`
	Type       string    `json:"type"` // "bookmark" or "note"
	Title      *string   `json:"title,omitempty"`
	URL        *string   `json:"url,omitempty"`
	Content    *string   `json:"content,omitempty"`
	FaviconURL *string   `json:"favicon_url,omitempty"`
	Position   int       `json:"position"`
	CreatedAt  time.Time `json:"created_at"`
}

// Bookmark represents a single bookmark (for backward compatibility)
type Bookmark struct {
	ID         int       `json:"id"`
	ListID     int       `json:"list_id"`
	Title      string    `json:"title"`
	URL        string    `json:"url"`
	FaviconURL *string   `json:"favicon_url,omitempty"`
	Position   int       `json:"position"`
	CreatedAt  time.Time `json:"created_at"`
}

// Note represents a single note
type Note struct {
	ID        int       `json:"id"`
	ListID    int       `json:"list_id"`
	Content   string    `json:"content"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
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
	Version    int          `json:"version"`
	ExportedAt time.Time    `json:"exported_at"`
	Lists      []ExportList `json:"lists"`
}

// ExportList represents a list with its items in export format
type ExportList struct {
	ID        int              `json:"id"`
	Title     string           `json:"title"`
	Color     string           `json:"color"`
	Position  int              `json:"position"`
	Collapsed bool             `json:"collapsed"`
	Bookmarks []ExportBookmark `json:"bookmarks"` // For backward compatibility
	Notes     []ExportNote     `json:"notes,omitempty"`
	Items     []ExportItem     `json:"items,omitempty"` // New unified format
}

// ExportItem represents an item in export format
type ExportItem struct {
	ID         int     `json:"id"`
	Type       string  `json:"type"`
	Title      *string `json:"title,omitempty"`
	URL        *string `json:"url,omitempty"`
	Content    *string `json:"content,omitempty"`
	FaviconURL *string `json:"favicon_url,omitempty"`
	Position   int     `json:"position"`
}

// ExportBookmark represents a bookmark in export format (for backward compatibility)
type ExportBookmark struct {
	ID         int     `json:"id"`
	Title      string  `json:"title"`
	URL        string  `json:"url"`
	FaviconURL *string `json:"favicon_url,omitempty"`
	Position   int     `json:"position"`
}

// ExportNote represents a note in export format
type ExportNote struct {
	ID       int    `json:"id"`
	Content  string `json:"content"`
	Position int    `json:"position"`
}
