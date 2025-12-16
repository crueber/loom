package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/crueber/loom/internal/db"
	"github.com/crueber/loom/internal/favicon"
	"github.com/crueber/loom/internal/models"
	"github.com/go-chi/chi/v5"
)

// ItemsAPI handles item endpoints (unified bookmarks and notes)
type ItemsAPI struct {
	db             *db.DB
	faviconFetcher *favicon.Fetcher
}

// NewItemsAPI creates a new items API handler
func NewItemsAPI(database *db.DB, faviconFetcher *favicon.Fetcher) *ItemsAPI {
	return &ItemsAPI{
		db:             database,
		faviconFetcher: faviconFetcher,
	}
}

// CreateItemRequest represents a request to create an item
type CreateItemRequest struct {
	ListID        int     `json:"list_id"`
	Type          string  `json:"type"` // "bookmark" or "note"
	Title         *string `json:"title,omitempty"`
	URL           *string `json:"url,omitempty"`
	Content       *string `json:"content,omitempty"`
	IconSource    string  `json:"icon_source,omitempty"`     // "auto", "custom", "service"
	CustomIconURL *string `json:"custom_icon_url,omitempty"` // Custom icon URL or service slug
}

// UpdateItemRequest represents a request to update an item
type UpdateItemRequest struct {
	Title         *string `json:"title,omitempty"`
	URL           *string `json:"url,omitempty"`
	Content       *string `json:"content,omitempty"`
	IconSource    *string `json:"icon_source,omitempty"`     // "auto", "custom", "service"
	CustomIconURL *string `json:"custom_icon_url,omitempty"` // Custom icon URL or service slug
}

// ReorderItemsRequest represents a request to reorder items
type ReorderItemsRequest struct {
	Items []struct {
		ID       int `json:"id"`
		Position int `json:"position"`
		ListID   int `json:"list_id"`
	} `json:"items"`
}

// HandleGetItems returns all items for a list
func (api *ItemsAPI) HandleGetItems(w http.ResponseWriter, r *http.Request) {
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
	exists, err := api.db.VerifyListOwnership(listID, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	if !exists {
		respondError(w, http.StatusNotFound, "List not found")
		return
	}

	items, err := api.db.GetItems(listID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get items")
		return
	}

	if items == nil {
		items = []*models.Item{}
	}

	respondJSON(w, http.StatusOK, items)
}

// HandleCreateItem creates a new item (bookmark or note)
func (api *ItemsAPI) HandleCreateItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req CreateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate type
	if req.Type != "bookmark" && req.Type != "note" {
		respondError(w, http.StatusBadRequest, "Type must be 'bookmark' or 'note'")
		return
	}

	// Verify list ownership
	exists, err := api.db.VerifyListOwnership(req.ListID, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	if !exists {
		respondError(w, http.StatusNotFound, "List not found")
		return
	}

	// Set default icon source if not provided
	iconSource := req.IconSource
	if iconSource == "" {
		iconSource = "auto"
	}

	// Type-specific validation
	var faviconURL *string
	if req.Type == "bookmark" {
		// Validate bookmark fields
		if req.Title == nil || strings.TrimSpace(*req.Title) == "" {
			respondError(w, http.StatusBadRequest, "Title is required for bookmarks")
			return
		}
		*req.Title = strings.TrimSpace(*req.Title)
		if len(*req.Title) > 200 {
			respondError(w, http.StatusBadRequest, "Title must be less than 200 characters")
			return
		}

		if req.URL == nil || strings.TrimSpace(*req.URL) == "" {
			respondError(w, http.StatusBadRequest, "URL is required for bookmarks")
			return
		}
		*req.URL = strings.TrimSpace(*req.URL)
		if !isValidURL(*req.URL) {
			respondError(w, http.StatusBadRequest, "Invalid URL")
			return
		}

		// Fetch favicon based on icon source
		domain := extractDomainFromURL(*req.URL)
		faviconURL, _ = api.faviconFetcher.FetchIcon(iconSource, req.CustomIconURL, domain)
	} else if req.Type == "note" {
		// Validate note fields
		if req.Content == nil || strings.TrimSpace(*req.Content) == "" {
			respondError(w, http.StatusBadRequest, "Content is required for notes")
			return
		}
		*req.Content = strings.TrimSpace(*req.Content)
	}

	// Get next position efficiently
	position, err := api.db.GetNextItemPosition(req.ListID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get next position")
		return
	}

	// Create item
	item, err := api.db.CreateItem(req.ListID, req.Type, req.Title, req.URL, req.Content, faviconURL, iconSource, req.CustomIconURL, position)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create item")
		return
	}

	respondJSON(w, http.StatusCreated, item)
}

// HandleUpdateItem updates an item
func (api *ItemsAPI) HandleUpdateItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	itemID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid item ID")
		return
	}

	// Verify ownership
	exists, err := api.db.VerifyItemOwnership(itemID, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	if !exists {
		respondError(w, http.StatusNotFound, "Item not found")
		return
	}

	// Get item to check type
	item, err := api.db.GetItem(itemID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	if item == nil {
		respondError(w, http.StatusNotFound, "Item not found")
		return
	}

	var req UpdateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// DEBUG: Log the request
	log.Printf("UpdateItem request for item %d: IconSource=%v, CustomIconURL=%v", itemID, req.IconSource, req.CustomIconURL)

	updates := make(map[string]interface{})

	// Type-specific validation
	if item.Type == "bookmark" {
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
			updates["title"] = req.Title
		}

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
			updates["url"] = req.URL
		}

		// Handle icon source and custom icon URL changes
		iconSourceChanged := req.IconSource != nil
		customIconURLChanged := req.CustomIconURL != nil

		if iconSourceChanged {
			updates["icon_source"] = req.IconSource
		}
		if customIconURLChanged {
			updates["custom_icon_url"] = req.CustomIconURL
		}

		// Re-fetch favicon if icon source or custom icon URL changed
		if iconSourceChanged || customIconURLChanged {
			// Determine which URL to use for favicon fetching
			var urlForFavicon string
			if req.URL != nil {
				urlForFavicon = *req.URL
			} else if item.URL != nil {
				urlForFavicon = *item.URL
			}

			// Determine which icon source to use
			iconSource := item.IconSource
			if req.IconSource != nil {
				iconSource = *req.IconSource
			}

			// Determine which custom icon URL to use
			var customIconURL *string
			if iconSource == "auto" {
				// Always use nil for auto mode to fetch from domain
				customIconURL = nil
			} else if req.CustomIconURL != nil {
				customIconURL = req.CustomIconURL
			} else {
				customIconURL = item.CustomIconURL
			}

			// Fetch new favicon
			if urlForFavicon != "" {
				domain := extractDomainFromURL(urlForFavicon)
				faviconURL, err := api.faviconFetcher.FetchIcon(iconSource, customIconURL, domain)
				if err != nil {
					// Clear favicon_url if fetch fails
					updates["favicon_url"] = nil
				} else if faviconURL != nil {
					updates["favicon_url"] = faviconURL
				}
			}
		}
	} else if item.Type == "note" {
		if req.Content != nil {
			*req.Content = strings.TrimSpace(*req.Content)
			updates["content"] = req.Content
		}
	}

	// Update item
	if err := api.db.UpdateItemFields(itemID, updates); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update item")
		return
	}

	// Get updated item
	updatedItem, err := api.db.GetItem(itemID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get item")
		return
	}

	respondJSON(w, http.StatusOK, updatedItem)
}

// HandleDeleteItem deletes an item
func (api *ItemsAPI) HandleDeleteItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	itemID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid item ID")
		return
	}

	// Verify ownership
	exists, err := api.db.VerifyItemOwnership(itemID, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	if !exists {
		respondError(w, http.StatusNotFound, "Item not found")
		return
	}

	// Delete item
	if err := api.db.DeleteItem(itemID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete item")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleReorderItems reorders items
func (api *ItemsAPI) HandleReorderItems(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req ReorderItemsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.Items) == 0 {
		respondError(w, http.StatusBadRequest, "No items to reorder")
		return
	}

	// Collect item IDs and list IDs for bulk verification
	itemIDs := make([]int, 0, len(req.Items))
	listIDs := make([]int, 0, len(req.Items))
	listIDSet := make(map[int]bool)
	itemPositions := make(map[int]struct {
		Position int
		ListID   int
	})

	for _, item := range req.Items {
		itemIDs = append(itemIDs, item.ID)
		if !listIDSet[item.ListID] {
			listIDs = append(listIDs, item.ListID)
			listIDSet[item.ListID] = true
		}
		itemPositions[item.ID] = struct {
			Position int
			ListID   int
		}{
			Position: item.Position,
			ListID:   item.ListID,
		}
	}

	// Verify ownership of all items and lists in a single query
	owned, err := api.db.VerifyItemsAndListsOwnership(itemIDs, listIDs, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	if !owned {
		respondError(w, http.StatusNotFound, "One or more items or lists not found")
		return
	}

	// Update positions
	if err := api.db.UpdateItemPositions(itemPositions); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update item positions")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// extractDomainFromURL extracts the domain from a URL string
func extractDomainFromURL(rawURL string) string {
	// Add scheme if missing
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	// Simple extraction without net/url to avoid import
	// Find the position after the scheme
	schemeEnd := strings.Index(rawURL, "://")
	if schemeEnd == -1 {
		return ""
	}

	// Start after "://"
	domainStart := schemeEnd + 3

	// Find the end of the domain (first /, :, or end of string)
	domainEnd := len(rawURL)
	for i := domainStart; i < len(rawURL); i++ {
		if rawURL[i] == '/' || rawURL[i] == ':' {
			domainEnd = i
			break
		}
	}

	return rawURL[domainStart:domainEnd]
}
