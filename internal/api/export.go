package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/crueber/loom/internal/db"
	"github.com/crueber/loom/internal/models"
)

// ExportAPI handles export/import endpoints
type ExportAPI struct {
	db *db.DB
}

// NewExportAPI creates a new export API handler
func NewExportAPI(database *db.DB) *ExportAPI {
	return &ExportAPI{db: database}
}

// ImportRequest represents an import request
type ImportRequest struct {
	Data models.ExportData `json:"data"`
	Mode string            `json:"mode"` // "merge" or "replace"
}

// HandleExport exports all user data as JSON
func (e *ExportAPI) HandleExport(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	// Get all lists for the user
	lists, err := e.db.GetLists(userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get lists")
		return
	}

	// Build export data
	exportLists := []models.ExportList{}
	for _, list := range lists {
		// Get items for each list
		items, err := e.db.GetItems(list.ID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to get items")
			return
		}

		// Convert items to export format
		exportItems := []models.ExportItem{}
		exportBookmarks := []models.ExportBookmark{} // For backward compatibility
		for _, item := range items {
			exportItems = append(exportItems, models.ExportItem{
				ID:         item.ID,
				Type:       item.Type,
				Title:      item.Title,
				URL:        item.URL,
				Content:    item.Content,
				FaviconURL: item.FaviconURL,
				Position:   item.Position,
			})

			// Also populate legacy bookmarks field if it's a bookmark
			if item.Type == "bookmark" {
				title := ""
				if item.Title != nil {
					title = *item.Title
				}
				url := ""
				if item.URL != nil {
					url = *item.URL
				}
				exportBookmarks = append(exportBookmarks, models.ExportBookmark{
					ID:         item.ID,
					Title:      title,
					URL:        url,
					FaviconURL: item.FaviconURL,
					Position:   item.Position,
				})
			}
		}

		exportLists = append(exportLists, models.ExportList{
			ID:        list.ID,
			Title:     list.Title,
			Color:     list.Color,
			Position:  list.Position,
			Collapsed: list.Collapsed,
			Items:     exportItems,
			Bookmarks: exportBookmarks,
		})
	}

	exportData := models.ExportData{
		Version:    1,
		ExportedAt: time.Now(),
		Lists:      exportLists,
	}

	// Set content disposition header for download
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=loom-export-%s.json", time.Now().Format("2006-01-02")))
	respondJSON(w, http.StatusOK, exportData)
}

// HandleImport imports user data from JSON
func (e *ExportAPI) HandleImport(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req ImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate mode
	if req.Mode != "merge" && req.Mode != "replace" {
		respondError(w, http.StatusBadRequest, "Invalid import mode (must be 'merge' or 'replace')")
		return
	}

	// Validate version
	if req.Data.Version != 1 {
		respondError(w, http.StatusBadRequest, "Unsupported export version")
		return
	}

	// Get or create default board for this user
	defaultBoard, err := e.db.GetDefaultBoard(userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get default board")
		return
	}

	// Handle replace mode: delete all existing data
	if req.Mode == "replace" {
		lists, err := e.db.GetLists(userID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to get existing lists")
			return
		}

		for _, list := range lists {
			if err := e.db.DeleteList(list.ID, userID); err != nil {
				respondError(w, http.StatusInternalServerError, "Failed to delete existing data")
				return
			}
		}
	}

	// Import lists and bookmarks
	listIDMap := make(map[int]int) // old ID -> new ID

	for _, exportList := range req.Data.Lists {
		// In merge mode, check if list exists
		var newList *models.List
		var err error

		if req.Mode == "merge" {
			// Try to get existing list by ID
			existingList, err := e.db.GetList(exportList.ID, userID)
			if err != nil {
				respondError(w, http.StatusInternalServerError, "Database error")
				return
			}

			if existingList != nil {
				// Update existing list
				title := exportList.Title
				color := exportList.Color
				collapsed := exportList.Collapsed
				if err := e.db.UpdateList(exportList.ID, userID, &title, &color, &collapsed); err != nil {
					respondError(w, http.StatusInternalServerError, "Failed to update list")
					return
				}
				newList = existingList
				newList.Title = title
				newList.Color = color
				newList.Collapsed = collapsed
			}
		}

		// Create new list if it doesn't exist
		if newList == nil {
			newList, err = e.db.CreateList(userID, defaultBoard.ID, exportList.Title, exportList.Color, exportList.Position)
			if err != nil {
				respondError(w, http.StatusInternalServerError, "Failed to create list")
				return
			}
		}

		listIDMap[exportList.ID] = newList.ID

		// Import bookmarks
		for _, exportBookmark := range exportList.Bookmarks {
			if req.Mode == "merge" {
				// Try to get existing bookmark
				existingBookmark, err := e.db.GetBookmark(exportBookmark.ID)
				if err != nil {
					respondError(w, http.StatusInternalServerError, "Database error")
					return
				}

				if existingBookmark != nil {
					// Update existing bookmark
					title := exportBookmark.Title
					url := exportBookmark.URL
					faviconPtr := exportBookmark.FaviconURL
					faviconURL := &faviconPtr
					if err := e.db.UpdateBookmark(exportBookmark.ID, &title, &url, faviconURL); err != nil {
						respondError(w, http.StatusInternalServerError, "Failed to update bookmark")
						return
					}
					continue
				}
			}

			// Create new bookmark
			_, err := e.db.CreateBookmark(newList.ID, exportBookmark.Title, exportBookmark.URL, exportBookmark.FaviconURL, exportBookmark.Position)
			if err != nil {
				respondError(w, http.StatusInternalServerError, "Failed to create bookmark")
				return
			}
		}
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("Successfully imported %d lists", len(req.Data.Lists)),
	})
}
