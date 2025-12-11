package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
)

const (
	sessionName = "loom-session"
	sessionKey  = "user_id"
)

// SessionManager handles user sessions
type SessionManager struct {
	store        *sessions.CookieStore
	maxAge       int
	secureCookie bool
}

// NewSessionManager creates a new session manager
func NewSessionManager(authKey, encryptionKey []byte, maxAge int, secureCookie bool) *SessionManager {
	store := sessions.NewCookieStore(authKey, encryptionKey)

	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secureCookie, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode, // Lax allows cookies on OAuth redirects
	}

	return &SessionManager{
		store:        store,
		maxAge:       maxAge,
		secureCookie: secureCookie,
	}
}

// CreateSession creates a new session for the user
func (sm *SessionManager) CreateSession(w http.ResponseWriter, r *http.Request, userID int) error {
	session, err := sm.store.Get(r, sessionName)
	if err != nil {
		println("DEBUG: Failed to get session:", err.Error())
		// Create a new session if the existing one is invalid
		session, err = sm.store.New(r, sessionName)
		if err != nil {
			println("DEBUG: Failed to create new session:", err.Error())
			return err
		}
		println("DEBUG: Created new session successfully")
	}

	session.Values[sessionKey] = userID
	println("DEBUG: About to save session for user ID:", userID)
	err = session.Save(r, w)
	if err != nil {
		println("DEBUG: Failed to save session:", err.Error())
		return err
	}
	println("DEBUG: Session saved successfully")
	return nil
}

// GetUserID retrieves the user ID from the session
func (sm *SessionManager) GetUserID(r *http.Request) (int, bool) {
	session, err := sm.store.Get(r, sessionName)
	if err != nil {
		return 0, false
	}

	userID, ok := session.Values[sessionKey].(int)
	if !ok {
		return 0, false
	}

	return userID, true
}

// DestroySession destroys the user's session
func (sm *SessionManager) DestroySession(w http.ResponseWriter, r *http.Request) error {
	session, err := sm.store.Get(r, sessionName)
	if err != nil {
		return nil // Session doesn't exist, nothing to destroy
	}

	session.Options.MaxAge = -1
	return session.Save(r, w)
}

// GenerateSessionID generates a random session ID
func GenerateSessionID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateKey generates a random key of the specified length
func GenerateKey(length int) ([]byte, error) {
	key := make([]byte, length)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

// SessionExpiry calculates the expiry time for a session
func SessionExpiry(maxAge int) time.Time {
	return time.Now().Add(time.Duration(maxAge) * time.Second)
}

// GetSession gets the session from the request
func (sm *SessionManager) GetSession(r *http.Request) (*sessions.Session, error) {
	return sm.store.Get(r, sessionName)
}

// SaveSession saves the session
func (sm *SessionManager) SaveSession(w http.ResponseWriter, r *http.Request, session *sessions.Session) error {
	return session.Save(r, w)
}
