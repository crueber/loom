package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// cacheControlMiddleware adds appropriate cache headers for static assets
func cacheControlMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		hasVersion := r.URL.Query().Get("v") != ""

		// Cache versioned assets for 1 year (immutable)
		if strings.HasPrefix(path, "/static/") && hasVersion {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else if strings.HasPrefix(path, "/static/lib/") {
			// Cache third-party libs for 1 year (already versioned by library authors)
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else if strings.HasPrefix(path, "/static/") {
			// Cache unversioned assets for 5 minutes
			w.Header().Set("Cache-Control", "public, max-age=300")
		}

		next.ServeHTTP(w, r)
	})
}

// cacheInvalidationMiddleware invalidates the app cache on POST, PUT, DELETE requests
func cacheInvalidationMiddleware(appHandler *AppHandler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only intercept mutations
			if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodDelete {
				next.ServeHTTP(w, r)
				return
			}

			// Get userID from session
			userID, ok := appHandler.sessionManager.GetUserID(r)
			if !ok && appHandler.isStandalone {
				user, err := appHandler.database.GetUserByEmail("user@standalone")
				if err == nil && user != nil {
					userID = user.ID
					ok = true
				}
			}

			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			// We need to capture the boardID.
			// The URL could be /api/boards/{id}, /api/lists/{id}, /api/items/{id}, etc.
			path := r.URL.Path
			var boardID int

			if strings.HasPrefix(path, "/api/boards/") {
				idStr := chi.URLParam(r, "id")
				if idStr == "" {
					// Fallback if chi param not yet populated (middleware order)
					parts := strings.Split(path, "/")
					if len(parts) >= 4 {
						idStr = parts[3]
					}
				}
				boardID, _ = strconv.Atoi(idStr)
			} else if strings.HasPrefix(path, "/api/lists") {
				if r.Method == http.MethodPost {
					// For POST /api/lists, the board_id is in the request body
					body, err := io.ReadAll(r.Body)
					if err == nil {
						r.Body = io.NopCloser(bytes.NewBuffer(body))
						var req struct {
							BoardID int `json:"board_id"`
						}
						if err := json.Unmarshal(body, &req); err == nil && req.BoardID > 0 {
							boardID = req.BoardID
						}
					}
				} else {
					// For lists, we might need to fetch the list to find the boardID
					idStr := chi.URLParam(r, "id")
					if idStr == "" {
						parts := strings.Split(path, "/")
						if len(parts) >= 4 {
							idStr = parts[3]
						}
					}
					listID, _ := strconv.Atoi(idStr)
					if listID > 0 {
						list, err := appHandler.database.GetList(listID, userID)
						if err == nil && list != nil {
							boardID = list.BoardID
						}
					}
				}
			} else if strings.HasPrefix(path, "/api/items") {
				if r.Method == http.MethodPost {
					// For POST /api/items, the list_id is in the request body
					body, err := io.ReadAll(r.Body)
					if err == nil {
						// Restore body for the next handler
						r.Body = io.NopCloser(bytes.NewBuffer(body))

						var req struct {
							ListID int `json:"list_id"`
						}
						if err := json.Unmarshal(body, &req); err == nil && req.ListID > 0 {
							list, err := appHandler.database.GetList(req.ListID, userID)
							if err == nil && list != nil {
								boardID = list.BoardID
							}
						}
					}
				} else {
					idStr := chi.URLParam(r, "id")
					if idStr == "" {
						parts := strings.Split(path, "/")
						if len(parts) >= 4 {
							idStr = parts[3]
						}
					}
					itemID, _ := strconv.Atoi(idStr)
					if itemID > 0 {
						item, err := appHandler.database.GetItem(itemID)
						if err == nil && item != nil {
							list, err := appHandler.database.GetList(item.ListID, userID)
							if err == nil && list != nil {
								boardID = list.BoardID
							}
						}
					}
				}
			} else if strings.HasPrefix(path, "/api/user/locale") || strings.HasPrefix(path, "/api/user/theme") {
				// Invalidate all boards for this user if they change global settings
				appHandler.InvalidateUserCache(userID)
			}

			// Call next handler first to ensure the operation succeeds
			// Actually, we should invalidate regardless if it's a mutation attempt on these paths
			// but usually we do it after. However, middleware runs before.
			// We can use a custom response writer to check status code if we wanted to be precise.
			// For simplicity, we'll invalidate now.
			if boardID > 0 {
				appHandler.InvalidateCache(userID, boardID)
			} else if r.Method == http.MethodPost && strings.HasPrefix(path, "/api/boards") {
				// Creating a new board doesn't invalidate an existing one, but might affect the board list.
				// However, our cache is per boardID.
			}

			next.ServeHTTP(w, r)
		})
	}
}
