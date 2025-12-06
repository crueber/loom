package api

import (
	"encoding/json"
	"net/http"

	"github.com/crueber/home-links/internal/db"
	"github.com/crueber/home-links/internal/models"
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
	Lists     []*models.List                `json:"lists"`
	Bookmarks map[int][]*models.Bookmark    `json:"bookmarks"` // keyed by list_id
}

// HandleGetAllData returns all lists and bookmarks for the authenticated user in a single request
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

	// Fetch all bookmarks in single query
	allBookmarks, err := api.db.GetAllBookmarks(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Group bookmarks by list_id for easier frontend consumption
	bookmarksByList := make(map[int][]*models.Bookmark)
	for _, bm := range allBookmarks {
		bookmarksByList[bm.ListID] = append(bookmarksByList[bm.ListID], bm)
	}

	response := DataResponse{
		Lists:     lists,
		Bookmarks: bookmarksByList,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
