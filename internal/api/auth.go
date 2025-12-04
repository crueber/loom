package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/crueber/home-links/internal/auth"
	"github.com/crueber/home-links/internal/db"
)

// AuthAPI handles authentication endpoints
type AuthAPI struct {
	db             *db.DB
	sessionManager *auth.SessionManager
}

// NewAuthAPI creates a new authentication API handler
func NewAuthAPI(database *db.DB, sessionManager *auth.SessionManager) *AuthAPI {
	return &AuthAPI{
		db:             database,
		sessionManager: sessionManager,
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
