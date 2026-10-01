package model

import (
	"encoding/json"
	"time"
)

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
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
}

// AuthSettings is the single-row OIDC configuration edited via the UI.
// ClientSecret never leaves the server (sanitized view below).
// OIDCBackfillDone records that the first-sign-in backfill ran
// (OSS-71): legacy unowned boards were claimed once, later sign-ins
// are a no-op. Managed by Store.ClaimUnownedBoards, never by the UI.
type AuthSettings struct {
	Issuer           string `json:"issuer"`
	ClientID         string `json:"client_id"`
	ClientSecret     string `json:"client_secret,omitempty"`
	Enabled          bool   `json:"enabled"`
	RequireAuth      bool   `json:"require_auth"`
	OIDCBackfillDone bool   `json:"oidc_backfill_done,omitempty"`
	PublicURL        string `json:"public_url,omitempty"`
}

// PublicSettings is the browser-safe view (secret replaced by a flag).
type PublicSettings struct {
	Issuer      string `json:"issuer"`
	ClientID    string `json:"client_id"`
	HasSecret   bool   `json:"has_secret"`
	Enabled     bool   `json:"enabled"`
	RequireAuth bool   `json:"require_auth"`
	PublicURL   string `json:"public_url,omitempty"`
	CallbackURL string `json:"callback_url,omitempty"`
}

// Sanitized drops the secret for browser responses.
func (s AuthSettings) Sanitized() PublicSettings {
	return PublicSettings{
		Issuer:      s.Issuer,
		ClientID:    s.ClientID,
		HasSecret:   s.ClientSecret != "",
		Enabled:     s.Enabled,
		RequireAuth: s.RequireAuth,
		PublicURL:   s.PublicURL,
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

// UserPrefs is the per-user preference set (OSS-83). Theme mirrors the
// board background ids (web/app.js SWATCHES); language is EN-only for
// now but kept as a field so the picker shape is functional.
// Animations (OSS-158) toggles collapse/expand motion; default ON.
type UserPrefs struct {
	Theme      string `json:"theme"`
	Language   string `json:"language"`
	Animations bool   `json:"animations"`
}

// DefaultUserPrefs is the zero/anonymous fallback.
func DefaultUserPrefs() UserPrefs {
	return UserPrefs{Theme: "paper", Language: "en", Animations: true}
}

// UnmarshalJSON defaults animations to ON when the key is absent so
// pre-OSS-158 stored prefs (no animations key) keep animating; an
// explicit false is preserved.
func (p *UserPrefs) UnmarshalJSON(b []byte) error {
	type raw struct {
		Theme      string `json:"theme"`
		Language   string `json:"language"`
		Animations *bool  `json:"animations"`
	}
	var r raw
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	p.Theme = r.Theme
	p.Language = r.Language
	if r.Animations == nil {
		p.Animations = true
	} else {
		p.Animations = *r.Animations
	}
	*p = p.Normalize()
	return nil
}

// Normalize clamps unknown themes to paper (client normBg semantics)
// and unknown languages to en.
func (p UserPrefs) Normalize() UserPrefs {
	switch p.Theme {
	case "paper", "honey", "sage", "sky", "rose", "slate":
	default:
		p.Theme = "paper"
	}
	if p.Language != "en" {
		p.Language = "en"
	}
	return p
}
