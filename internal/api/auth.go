package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/crueber/loom/internal/auth"
	"github.com/crueber/loom/internal/db"
	"github.com/crueber/loom/internal/models"
	"github.com/crueber/loom/internal/oauth"
)

// AuthAPI handles authentication endpoints
type AuthAPI struct {
	db             *db.DB
	sessionManager *auth.SessionManager
	oauthClient    *oauth.Client
}

// NewAuthAPI creates a new authentication API handler
func NewAuthAPI(database *db.DB, sessionManager *auth.SessionManager, oauthClient *oauth.Client) *AuthAPI {
	return &AuthAPI{
		db:             database,
		sessionManager: sessionManager,
		oauthClient:    oauthClient,
	}
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// UserResponse represents a user response
type UserResponse struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

// HandleLogin handles user login
func (a *AuthAPI) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate input
	if req.Username == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "Username and password are required")
		return
	}

	// Get user from database
	user, err := a.db.GetUserByUsername(req.Username)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	if user == nil {
		respondError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	// Verify password
	valid, err := auth.VerifyPassword(req.Password, user.PasswordHash)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Authentication error")
		return
	}

	if !valid {
		respondError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	// Create session
	if err := a.sessionManager.CreateSession(w, r, user.ID); err != nil {
		println("DEBUG: Failed to create session:", err.Error())
		respondError(w, http.StatusInternalServerError, "Failed to create session")
		return
	}

	println("DEBUG: Session created successfully for user:", user.ID)
	respondJSON(w, http.StatusOK, UserResponse{
		ID:       user.ID,
		Username: user.Username,
	})
}

// HandleRegister handles user registration
func (a *AuthAPI) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate input
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		respondError(w, http.StatusBadRequest, "Username is required")
		return
	}
	if len(req.Username) < 3 || len(req.Username) > 50 {
		respondError(w, http.StatusBadRequest, "Username must be between 3 and 50 characters")
		return
	}
	if req.Password == "" {
		respondError(w, http.StatusBadRequest, "Password is required")
		return
	}
	if len(req.Password) < 8 {
		respondError(w, http.StatusBadRequest, "Password must be at least 8 characters")
		return
	}

	// Check if user already exists
	existingUser, err := a.db.GetUserByUsername(req.Username)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	if existingUser != nil {
		respondError(w, http.StatusConflict, "Username already exists")
		return
	}

	// Hash password
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	// Create user
	user, err := a.db.CreateUser(req.Username, passwordHash)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	// Create session
	if err := a.sessionManager.CreateSession(w, r, user.ID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create session")
		return
	}

	respondJSON(w, http.StatusCreated, UserResponse{
		ID:       user.ID,
		Username: user.Username,
	})
}

// HandleLogout handles user logout
func (a *AuthAPI) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if err := a.sessionManager.DestroySession(w, r); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to destroy session")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleGetUser returns the current user's information
func (a *AuthAPI) HandleGetUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.sessionManager.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	user, err := a.db.GetUserByID(userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	if user == nil {
		respondError(w, http.StatusNotFound, "User not found")
		return
	}

	respondJSON(w, http.StatusOK, UserResponse{
		ID:       user.ID,
		Username: user.Username,
	})
}

// AuthMiddleware checks if the user is authenticated
func (a *AuthAPI) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := a.sessionManager.GetUserID(r)
		if !ok {
			respondError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		// Add user ID to context
		ctx := r.Context()
		ctx = setUserID(ctx, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// HandleOAuthLogin redirects the user to the OAuth2 provider
func (a *AuthAPI) HandleOAuthLogin(w http.ResponseWriter, r *http.Request) {
	// Generate random state for CSRF protection
	state := generateRandomState()

	// Store state in session
	session, _ := a.sessionManager.GetSession(r)
	session.Values["oauth_state"] = state
	if err := a.sessionManager.SaveSession(w, r, session); err != nil {
		log.Printf("Failed to save oauth state to session: %v", err)
		http.Error(w, "Failed to initiate login", http.StatusInternalServerError)
		return
	}

	// Redirect to OAuth2 provider
	authURL := a.oauthClient.AuthCodeURL(state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// HandleOAuthCallback handles the OAuth2 callback
func (a *AuthAPI) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get state from session
	session, _ := a.sessionManager.GetSession(r)
	expectedState, ok := session.Values["oauth_state"].(string)
	if !ok || expectedState == "" {
		http.Error(w, "Invalid session state", http.StatusBadRequest)
		return
	}

	// Verify state parameter (CSRF protection)
	actualState := r.URL.Query().Get("state")
	if actualState != expectedState {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	// Exchange authorization code for token
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	token, err := a.oauthClient.Exchange(ctx, code)
	if err != nil {
		log.Printf("Failed to exchange token: %v", err)
		http.Error(w, "Failed to exchange authorization code", http.StatusInternalServerError)
		return
	}

	// Extract ID token
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		log.Printf("No id_token in token response. Token type: %T, Available extras: %+v", token.Extra("id_token"), token)
		http.Error(w, "No id_token in response", http.StatusInternalServerError)
		return
	}

	log.Printf("DEBUG: Raw ID token length: %d, first 50 chars: %s", len(rawIDToken), rawIDToken[:min(50, len(rawIDToken))])

	// Verify ID token
	idToken, err := a.oauthClient.VerifyIDToken(ctx, rawIDToken)
	if err != nil {
		log.Printf("Failed to verify ID token: %v", err)
		http.Error(w, "Failed to verify ID token", http.StatusInternalServerError)
		return
	}

	// Extract user info from claims
	userInfo, err := a.oauthClient.GetUserInfo(ctx, idToken)
	if err != nil {
		log.Printf("Failed to extract user info: %v", err)
		http.Error(w, "Failed to extract user info", http.StatusInternalServerError)
		return
	}

	// Email is required
	if userInfo.Email == "" {
		http.Error(w, "No email in token claims", http.StatusBadRequest)
		return
	}

	// Get or create user (auto-provisioning)
	user, err := a.provisionUser(ctx, userInfo)
	if err != nil {
		log.Printf("Failed to provision user: %v", err)
		http.Error(w, "Failed to provision user", http.StatusInternalServerError)
		return
	}

	// Create session
	delete(session.Values, "oauth_state") // Clear state
	session.Values["user_id"] = user.ID
	if err := a.sessionManager.SaveSession(w, r, session); err != nil {
		log.Printf("Failed to create session: %v", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Redirect to app
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// provisionUser gets existing user or creates new one with default board
func (a *AuthAPI) provisionUser(ctx context.Context, userInfo *oauth.UserInfo) (*models.User, error) {
	// Try to get existing user by email
	user, err := a.db.GetUserByEmail(userInfo.Email)
	if err == nil {
		// User exists, return it
		return user, nil
	}

	// User doesn't exist, create new user
	provider := "authentik"
	user, err = a.db.CreateOAuthUser(userInfo.Email, provider, userInfo.Sub)
	if err != nil {
		return nil, err
	}

	log.Printf("Created new user via OAuth2: %s (ID: %d)", userInfo.Email, user.ID)

	// Create default board for new user
	if err := a.createDefaultBoard(user.ID); err != nil {
		// Log error but don't fail - user was created successfully
		log.Printf("Warning: failed to create default board for user %d: %v", user.ID, err)
	}

	return user, nil
}

// createDefaultBoard creates the default board for a new user
func (a *AuthAPI) createDefaultBoard(userID int) error {
	_, err := a.db.CreateBoard(userID, "My Bookmarks")
	return err
}

// generateRandomState generates a random state string for OAuth2 CSRF protection
func generateRandomState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
