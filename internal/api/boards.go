package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/crueber/loom/internal/db"
	"github.com/go-chi/chi/v5"
)

// GetBoards returns all boards for the authenticated user
func GetBoards(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := getUserID(r.Context())
		if !ok {
			http.Error(w, "Not authenticated", http.StatusUnauthorized)
			return
		}

		boards, err := database.GetBoards(userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(boards)
	}
}

// GetBoard returns a specific board by ID
func GetBoard(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := getUserID(r.Context())
		if !ok {
			http.Error(w, "Not authenticated", http.StatusUnauthorized)
			return
		}
		boardID, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Invalid board ID", http.StatusBadRequest)
			return
		}

		board, err := database.GetBoardByID(boardID, userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if board == nil {
			http.Error(w, "Board not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(board)
	}
}

// CreateBoard creates a new board
func CreateBoard(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := getUserID(r.Context())
		if !ok {
			http.Error(w, "Not authenticated", http.StatusUnauthorized)
			return
		}

		var req struct {
			Title string `json:"title"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Title == "" {
			req.Title = "New Board"
		}

		if len(req.Title) > 100 {
			http.Error(w, "Title must be 100 characters or less", http.StatusBadRequest)
			return
		}

		board, err := database.CreateBoard(userID, req.Title)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(board)
	}
}

// UpdateBoard updates a board's title
func UpdateBoard(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := getUserID(r.Context())
		if !ok {
			http.Error(w, "Not authenticated", http.StatusUnauthorized)
			return
		}
		boardID, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Invalid board ID", http.StatusBadRequest)
			return
		}

		var req struct {
			Title string `json:"title"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Title == "" {
			http.Error(w, "Title is required", http.StatusBadRequest)
			return
		}

		if len(req.Title) > 100 {
			http.Error(w, "Title must be 100 characters or less", http.StatusBadRequest)
			return
		}

		err = database.UpdateBoard(boardID, userID, req.Title)
		if err != nil {
			if err.Error() == "board not found" {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// DeleteBoard deletes a board
func DeleteBoard(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := getUserID(r.Context())
		if !ok {
			http.Error(w, "Not authenticated", http.StatusUnauthorized)
			return
		}
		boardID, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Invalid board ID", http.StatusBadRequest)
			return
		}

		err = database.DeleteBoard(boardID, userID)
		if err != nil {
			if err.Error() == "board not found" {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			if err.Error() == "cannot delete default board" {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// GetBoardData returns all data for a board (board info, lists, and bookmarks)
func GetBoardData(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := getUserID(r.Context())
		if !ok {
			http.Error(w, "Not authenticated", http.StatusUnauthorized)
			return
		}
		boardID, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Invalid board ID", http.StatusBadRequest)
			return
		}

		// Get all boards for the user (for board switcher)
		boards, err := database.GetBoards(userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Verify board ownership
		board, err := database.GetBoardByID(boardID, userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if board == nil {
			http.Error(w, "Board not found", http.StatusNotFound)
			return
		}

		// Get lists for this board
		lists, err := database.GetListsByBoard(userID, boardID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Get all items for this user (we'll filter on frontend)
		// Or we could join through lists to get only items for this board
		items, err := database.GetAllItems(userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Filter items to only those in lists for this board
		listIDs := make(map[int]bool)
		for _, list := range lists {
			listIDs[list.ID] = true
		}

		filteredItems := make(map[int]interface{})
		for _, item := range items {
			if listIDs[item.ListID] {
				if _, exists := filteredItems[item.ListID]; !exists {
					filteredItems[item.ListID] = []interface{}{}
				}
				listItems := filteredItems[item.ListID].([]interface{})
				filteredItems[item.ListID] = append(listItems, item)
			}
		}

		response := map[string]interface{}{
			"board":  board,
			"boards": boards,
			"lists":  lists,
			"items":  filteredItems,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
