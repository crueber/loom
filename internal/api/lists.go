package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/crueber/home-links/internal/db"
	"github.com/crueber/home-links/internal/models"
	"github.com/go-chi/chi/v5"
)

// ListsAPI handles list endpoints
type ListsAPI struct {
	db *db.DB
}

// NewListsAPI creates a new lists API handler
func NewListsAPI(database *db.DB) *ListsAPI {
	return &ListsAPI{db: database}
}

// CreateListRequest represents a request to create a list
type CreateListRequest struct {
	Title string `json:"title"`
	Color string `json:"color"`
}

// UpdateListRequest represents a request to update a list
type UpdateListRequest struct {
	Title     *string `json:"title,omitempty"`
	Color     *string `json:"color,omitempty"`
	Collapsed *bool   `json:"collapsed,omitempty"`
}

// ReorderListsRequest represents a request to reorder lists
type ReorderListsRequest struct {
	Lists []struct {
		ID       int `json:"id"`
		Position int `json:"position"`
	} `json:"lists"`
}

var validColors = []string{
	"#3D6D95", // Blue (darker)
	"#4D7831", // Green (darker)
	"#B85720", // Orange (darker)
	"#A43529", // Red (darker)
	"#6B3D7D", // Purple (darker)
	"#924F7D", // Pink (darker)
	"#358178", // Teal (darker)
	"#697374", // Gray (darker)
}

// isValidColor checks if a color is in the valid palette
func isValidColor(color string) bool {
	for _, c := range validColors {
		if c == color {
			return true
		}
	}
	return false
}

// HandleGetLists returns all lists for the authenticated user
func (l *ListsAPI) HandleGetLists(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	lists, err := l.db.GetLists(userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get lists")
		return
	}

	if lists == nil {
		lists = []*models.List{}
	}

	respondJSON(w, http.StatusOK, lists)
}

// HandleCreateList creates a new list
func (l *ListsAPI) HandleCreateList(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req CreateListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate input
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		respondError(w, http.StatusBadRequest, "Title is required")
		return
	}
	if len(req.Title) > 100 {
		respondError(w, http.StatusBadRequest, "Title must be less than 100 characters")
		return
	}

	if !isValidColor(req.Color) {
		respondError(w, http.StatusBadRequest, "Invalid color")
		return
	}

	// Get current max position
	lists, err := l.db.GetLists(userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get lists")
		return
	}

	position := 0
	if len(lists) > 0 {
		position = lists[len(lists)-1].Position + 1
	}

	// Create list
	list, err := l.db.CreateList(userID, req.Title, req.Color, position)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create list")
		return
	}

	respondJSON(w, http.StatusCreated, list)
}

// HandleUpdateList updates a list
func (l *ListsAPI) HandleUpdateList(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	listID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid list ID")
		return
	}

	// Verify ownership
	exists, err := l.db.VerifyListOwnership(listID, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	if !exists {
		respondError(w, http.StatusNotFound, "List not found")
		return
	}

	var req UpdateListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate input
	if req.Title != nil {
		*req.Title = strings.TrimSpace(*req.Title)
		if *req.Title == "" {
			respondError(w, http.StatusBadRequest, "Title cannot be empty")
			return
		}
		if len(*req.Title) > 100 {
			respondError(w, http.StatusBadRequest, "Title must be less than 100 characters")
			return
		}
	}

	if req.Color != nil && !isValidColor(*req.Color) {
		respondError(w, http.StatusBadRequest, "Invalid color")
		return
	}

	// Update list
	if err := l.db.UpdateList(listID, userID, req.Title, req.Color, req.Collapsed); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update list")
		return
	}

	// Get updated list
	list, err := l.db.GetList(listID, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get list")
		return
	}

	respondJSON(w, http.StatusOK, list)
}

// HandleDeleteList deletes a list
func (l *ListsAPI) HandleDeleteList(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	listID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid list ID")
		return
	}

	// Delete list
	if err := l.db.DeleteList(listID, userID); err != nil {
		if err.Error() == "list not found" {
			respondError(w, http.StatusNotFound, "List not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to delete list")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleReorderLists updates the positions of multiple lists
func (l *ListsAPI) HandleReorderLists(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req ReorderListsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.Lists) == 0 {
		respondError(w, http.StatusBadRequest, "No lists to reorder")
		return
	}

	// Build positions map
	positions := make(map[int]int)
	for _, item := range req.Lists {
		positions[item.ID] = item.Position
	}

	// Update positions
	if err := l.db.UpdateListPositions(userID, positions); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to reorder lists")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
