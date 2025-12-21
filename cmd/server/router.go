package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	"github.com/crueber/loom/internal/api"
	"github.com/crueber/loom/internal/db"
	"github.com/crueber/loom/internal/favicon"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// RouterDependencies holds all dependencies needed for route setup
type RouterDependencies struct {
	StaticFiles embed.FS
	Database    *db.DB
	AuthAPI     *api.AuthAPI
	DataAPI     *api.DataAPI
	AppHandler  *AppHandler
}

// SetupRouter configures all routes and middleware
func SetupRouter(deps *RouterDependencies) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// Setup static file serving
	setupStaticFiles(r, deps.StaticFiles)

	// Setup application routes
	setupAppRoutes(r, deps.AppHandler)

	// Setup OAuth2 routes
	setupOAuthRoutes(r, deps.AuthAPI)

	// Setup API routes
	setupAPIRoutes(r, deps.Database, deps.AuthAPI, deps.DataAPI)

	return r
}

// setupStaticFiles configures static file serving with cache control
func setupStaticFiles(r *chi.Mux, staticFiles embed.FS) {
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to load static files: %v", err)
	}

	r.Handle("/static/*", cacheControlMiddleware(
		http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))),
	))
}

// setupAppRoutes configures main application page routes
func setupAppRoutes(r *chi.Mux, appHandler *AppHandler) {
	r.Get("/", appHandler.ServeApp)
	r.Get("/boards/{id}", appHandler.ServeApp)
}

// setupOAuthRoutes configures OAuth2 authentication routes
func setupOAuthRoutes(r *chi.Mux, authAPI *api.AuthAPI) {
	r.Get("/auth/login", authAPI.HandleOAuthLogin)
	r.Get("/auth/callback", authAPI.HandleOAuthCallback)
}

// setupAPIRoutes configures all API endpoints
func setupAPIRoutes(r *chi.Mux, database *db.DB, authAPI *api.AuthAPI, dataAPI *api.DataAPI) {
	// Initialize API handlers
	listsAPI := api.NewListsAPI(database)
	bookmarksAPI := api.NewBookmarksAPI(database, favicon.New())
	itemsAPI := api.NewItemsAPI(database, favicon.New())
	exportAPI := api.NewExportAPI(database)

	r.Route("/api", func(r chi.Router) {
		// Public routes (deprecated - will be removed)
		r.Post("/login", authAPI.HandleLogin)
		r.Post("/register", authAPI.HandleRegister)

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(authAPI.AuthMiddleware)

			// Auth endpoints
			setupAuthEndpoints(r, authAPI)

			// Data endpoints
			setupDataEndpoints(r, database, dataAPI)

			// Board endpoints
			setupBoardEndpoints(r, database)

			// List endpoints
			setupListEndpoints(r, listsAPI)

			// Bookmark endpoints (deprecated)
			setupBookmarkEndpoints(r, bookmarksAPI)

			// Item endpoints
			setupItemEndpoints(r, itemsAPI)

			// Export/Import endpoints
			setupExportEndpoints(r, exportAPI)
		})
	})
}

// setupAuthEndpoints configures authentication-related endpoints
func setupAuthEndpoints(r chi.Router, authAPI *api.AuthAPI) {
	r.Post("/logout", authAPI.HandleLogout)
	r.Get("/user", authAPI.HandleGetUser)
	r.Post("/user/locale", authAPI.HandleUpdateLocale)
	r.Post("/user/theme", authAPI.HandleUpdateTheme)
}

// setupDataEndpoints configures combined data endpoints
func setupDataEndpoints(r chi.Router, database *db.DB, dataAPI *api.DataAPI) {
	r.Get("/data", dataAPI.HandleGetAllData)
}

// setupBoardEndpoints configures board-related endpoints
func setupBoardEndpoints(r chi.Router, database *db.DB) {
	r.Get("/boards", api.GetBoards(database))
	r.Post("/boards", api.CreateBoard(database))
	r.Get("/boards/{id}", api.GetBoard(database))
	r.Put("/boards/{id}", api.UpdateBoard(database))
	r.Delete("/boards/{id}", api.DeleteBoard(database))
	r.Get("/boards/{id}/data", api.GetBoardData(database))
}

// setupListEndpoints configures list-related endpoints
func setupListEndpoints(r chi.Router, listsAPI *api.ListsAPI) {
	r.Get("/lists", listsAPI.HandleGetLists)
	r.Post("/lists", listsAPI.HandleCreateList)
	r.Put("/lists/{id}", listsAPI.HandleUpdateList)
	r.Delete("/lists/{id}", listsAPI.HandleDeleteList)
	r.Put("/lists/reorder", listsAPI.HandleReorderLists)
	r.Post("/lists/{id}/copy-or-move", listsAPI.HandleCopyOrMoveList)
}

// setupBookmarkEndpoints configures bookmark-related endpoints (deprecated)
func setupBookmarkEndpoints(r chi.Router, bookmarksAPI *api.BookmarksAPI) {
	r.Get("/lists/{list_id}/bookmarks", bookmarksAPI.HandleGetBookmarks)
	r.Post("/bookmarks", bookmarksAPI.HandleCreateBookmark)
	r.Put("/bookmarks/{id}", bookmarksAPI.HandleUpdateBookmark)
	r.Delete("/bookmarks/{id}", bookmarksAPI.HandleDeleteBookmark)
	r.Put("/bookmarks/reorder", bookmarksAPI.HandleReorderBookmarks)
}

// setupItemEndpoints configures item-related endpoints (unified bookmarks and notes)
func setupItemEndpoints(r chi.Router, itemsAPI *api.ItemsAPI) {
	r.Get("/lists/{list_id}/items", itemsAPI.HandleGetItems)
	r.Post("/items", itemsAPI.HandleCreateItem)
	r.Put("/items/{id}", itemsAPI.HandleUpdateItem)
	r.Delete("/items/{id}", itemsAPI.HandleDeleteItem)
	r.Put("/items/reorder", itemsAPI.HandleReorderItems)
}

// setupExportEndpoints configures export/import endpoints
func setupExportEndpoints(r chi.Router, exportAPI *api.ExportAPI) {
	r.Get("/export", exportAPI.HandleExport)
	r.Post("/import", exportAPI.HandleImport)
}
