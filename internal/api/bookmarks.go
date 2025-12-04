package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/crueber/home-links/internal/db"
	"github.com/crueber/home-links/internal/favicon"
	"github.com/crueber/home-links/internal/models"
	"github.com/go-chi/chi/v5"
)

// BookmarksAPI handles bookmark endpoints
type BookmarksAPI struct {
	db             *db.DB
	faviconFetcher *favicon.Fetcher
}

// NewBookmarksAPI creates a new bookmarks API handler
func NewBookmarksAPI(database *db.DB, faviconFetcher *favicon.Fetcher) *BookmarksAPI {
	return &BookmarksAPI{
		db:             database,
		faviconFetcher: faviconFetcher,
	}
}

// CreateBookmarkRequest represents a request to create a bookmark
type CreateBookmarkRequest struct {
	ListID int    `json:"list_id"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

// UpdateBookmarkRequest represents a request to update a bookmark
type UpdateBookmarkRequest struct {
	Title *string `json:"title,omitempty"`
	URL   *string `json:"url,omitempty"`
}

// ReorderBookmarksRequest represents a request to reorder bookmarks
type ReorderBookmarksRequest struct {
	Bookmarks []struct {
		ID       int `json:"id"`
		Position int `json:"position"`
		ListID   int `json:"list_id"`
	} `json:"bookmarks"`
}

// isValidURL checks if a URL is valid
func isValidURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// HandleGetBookmarks returns all bookmarks for a list
func (b *BookmarksAPI) HandleGetBookmarks(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	listID, err := strconv.Atoi(chi.URLParam(r, "list_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid list ID")
		return
	}

	// Verify list ownership
	exists, err := b.db.VerifyListOwnership(listID, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	if !exists {
		respondError(w, http.StatusNotFound, "List not found")
		return
	}

	bookmarks, err := b.db.GetBookmarks(listID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get bookmarks")
		return
	}

	if bookmarks == nil {
		bookmarks = []*models.Bookmark{}
	}

	respondJSON(w, http.StatusOK, bookmarks)
}

// HandleCreateBookmark creates a new bookmark
func (b *BookmarksAPI) HandleCreateBookmark(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req CreateBookmarkRequest
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
	if len(req.Title) > 200 {
		respondError(w, http.StatusBadRequest, "Title must be less than 200 characters")
		return
	}

	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		respondError(w, http.StatusBadRequest, "URL is required")
		return
	}
	if !isValidURL(req.URL) {
		respondError(w, http.StatusBadRequest, "Invalid URL")
		return
	}

	// Verify list ownership
	exists, err := b.db.VerifyListOwnership(req.ListID, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	if !exists {
		respondError(w, http.StatusNotFound, "List not found")
		return
	}

	// Get current max position
	bookmarks, err := b.db.GetBookmarks(req.ListID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get bookmarks")
		return
	}

	position := 0
	if len(bookmarks) > 0 {
		position = bookmarks[len(bookmarks)-1].Position + 1
	}

	// Fetch favicon
	faviconURL := b.faviconFetcher.FetchFaviconURL(req.URL)

	// Create bookmark
	bookmark, err := b.db.CreateBookmark(req.ListID, req.Title, req.URL, faviconURL, position)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create bookmark")
		return
	}

	respondJSON(w, http.StatusCreated, bookmark)
}

// HandleUpdateBookmark updates a bookmark
func (b *BookmarksAPI) HandleUpdateBookmark(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	bookmarkID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid bookmark ID")
		return
	}

	// Verify ownership
	exists, err := b.db.VerifyBookmarkOwnership(bookmarkID, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	if !exists {
		respondError(w, http.StatusNotFound, "Bookmark not found")
		return
	}

	var req UpdateBookmarkRequest
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
		if len(*req.Title) > 200 {
			respondError(w, http.StatusBadRequest, "Title must be less than 200 characters")
			return
		}
	}

	var newFaviconURL **string
	if req.URL != nil {
		*req.URL = strings.TrimSpace(*req.URL)
		if *req.URL == "" {
			respondError(w, http.StatusBadRequest, "URL cannot be empty")
			return
		}
		if !isValidURL(*req.URL) {
			respondError(w, http.StatusBadRequest, "Invalid URL")
			return
		}

		// Fetch new favicon if URL changed
		faviconURL := b.faviconFetcher.FetchFaviconURL(*req.URL)
		newFaviconURL = &faviconURL
	}

	// Update bookmark
	if err := b.db.UpdateBookmark(bookmarkID, req.Title, req.URL, newFaviconURL); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update bookmark")
		return
	}

	// Get updated bookmark
	bookmark, err := b.db.GetBookmark(bookmarkID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get bookmark")
		return
	}

	respondJSON(w, http.StatusOK, bookmark)
}

// HandleDeleteBookmark deletes a bookmark
func (b *BookmarksAPI) HandleDeleteBookmark(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	bookmarkID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid bookmark ID")
		return
	}

	// Verify ownership
	exists, err := b.db.VerifyBookmarkOwnership(bookmarkID, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	if !exists {
		respondError(w, http.StatusNotFound, "Bookmark not found")
		return
	}

	// Delete bookmark
	if err := b.db.DeleteBookmark(bookmarkID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete bookmark")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleReorderBookmarks updates the positions of multiple bookmarks
func (b *BookmarksAPI) HandleReorderBookmarks(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req ReorderBookmarksRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.Bookmarks) == 0 {
		respondError(w, http.StatusBadRequest, "No bookmarks to reorder")
		return
	}

	// Verify ownership of all bookmarks and lists
	for _, item := range req.Bookmarks {
		exists, err := b.db.VerifyBookmarkOwnership(item.ID, userID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		if !exists {
			respondError(w, http.StatusNotFound, "Bookmark not found")
			return
		}

		exists, err = b.db.VerifyListOwnership(item.ListID, userID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		if !exists {
			respondError(w, http.StatusNotFound, "List not found")
			return
		}
	}

	// Build positions map
	positions := make(map[int]struct {
		Position int
		ListID   int
	})
	for _, item := range req.Bookmarks {
		positions[item.ID] = struct {
			Position int
			ListID   int
		}{
			Position: item.Position,
			ListID:   item.ListID,
		}
	}

	// Update positions
	if err := b.db.UpdateBookmarkPositions(positions); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to reorder bookmarks")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
