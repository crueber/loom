package model

import "time"

// Auth + board-permission model (OSS-50, UI-configured OIDC).
//
// Visibility: "" (legacy boards) behaves as "public". When global auth
// is enabled, new boards default to "private"; existing boards keep
// their stored value so enabling auth never locks anyone out.

const (
	VisibilityPublic  = "public"
	VisibilityPrivate = "private"
)

// Member roles on a board.
const (
	RoleViewer = "viewer"
	RoleEditor = "editor"
)

// EffectiveVisibility normalizes legacy/empty values to public.
func (b Board) EffectiveVisibility() string {
	if b.Visibility == VisibilityPrivate {
		return VisibilityPrivate
	}
	return VisibilityPublic
}

// User is a person authenticated via OIDC (one row per issuer+subject).
type User struct {
	ID        string    `json:"id"`
	Issuer    string    `json:"issuer"`
	Subject   string    `json:"subject"`
	Email     string    `json:"email,omitempty"`
	Name      string    `json:"name,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// AuthSettings is the single-row OIDC configuration edited via the UI.
// ClientSecret never leaves the server (sanitized view below).
type AuthSettings struct {
	Issuer       string `json:"issuer"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"`
	Enabled      bool   `json:"enabled"`
	RequireAuth  bool   `json:"require_auth"`
}

// PublicSettings is the browser-safe view (secret replaced by a flag).
type PublicSettings struct {
	Issuer      string `json:"issuer"`
	ClientID    string `json:"client_id"`
	HasSecret   bool   `json:"has_secret"`
	Enabled     bool   `json:"enabled"`
	RequireAuth bool   `json:"require_auth"`
}

// Sanitized drops the secret for browser responses.
func (s AuthSettings) Sanitized() PublicSettings {
	return PublicSettings{
		Issuer:      s.Issuer,
		ClientID:    s.ClientID,
		HasSecret:   s.ClientSecret != "",
		Enabled:     s.Enabled,
		RequireAuth: s.RequireAuth,
	}
}

// BoardMember grants a user a role on one board.
type BoardMember struct {
	BoardID string `json:"board_id"`
	UserID  string `json:"user_id"`
	Role    string `json:"role"` // viewer | editor
}

// Session is a server-side login session. Only the SHA-256 hash of the
// cookie token is stored; the raw token lives in an HttpOnly cookie.
type Session struct {
	TokenHash string    `json:"-"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NewUser builds a user row for an OIDC identity.
func NewUser(issuer, subject, email, name string) User {
	return User{ID: NewID(), Issuer: issuer, Subject: subject, Email: email, Name: name, CreatedAt: now()}
}
