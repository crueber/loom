package api

import (
	"encoding/json"
	"net/http"

	"github.com/crueber/loom/internal/db"
	"github.com/crueber/loom/internal/models"
)

// DataAPI handles the combined data endpoint
type DataAPI struct {
	db *db.DB
}

// NewDataAPI creates a new data API handler
func NewDataAPI(database *db.DB) *DataAPI {
	return &DataAPI{db: database}
}

// DataResponse represents the combined response with all user data
type DataResponse struct {
	Lists     []*models.List         `json:"lists"`
	Bookmarks map[int][]*models.Item `json:"bookmarks"` // keyed by list_id, contains all items (bookmarks and notes)
}

// HandleGetAllData returns all lists and items for the authenticated user in a single request
func (api *DataAPI) HandleGetAllData(w http.ResponseWriter, r *http.Request) {
	// Get user ID from session
	userID, ok := getUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Fetch all lists
	lists, err := api.db.GetLists(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch all items (bookmarks and notes) in single query
	allItems, err := api.db.GetAllItems(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Group items by list_id for easier frontend consumption
	itemsByList := make(map[int][]*models.Item)
	for _, item := range allItems {
		itemsByList[item.ListID] = append(itemsByList[item.ListID], item)
	}

	response := DataResponse{
		Lists:     lists,
		Bookmarks: itemsByList, // Keep field name for backward compatibility with frontend
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
