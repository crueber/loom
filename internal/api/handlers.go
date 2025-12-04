package api

import (
	"context"
	"encoding/json"
	"net/http"
)

// contextKey is a custom type for context keys
type contextKey string

const userIDKey contextKey = "user_id"

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError sends an error response
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, ErrorResponse{Error: message})
}

// setUserID adds the user ID to the context
func setUserID(ctx context.Context, userID int) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// getUserID retrieves the user ID from the context
func getUserID(ctx context.Context) (int, bool) {
	userID, ok := ctx.Value(userIDKey).(int)
	return userID, ok
}
