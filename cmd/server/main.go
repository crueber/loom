package main

import (
	"embed"
	"encoding/hex"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/crueber/loom/internal/api"
	"github.com/crueber/loom/internal/auth"
	"github.com/crueber/loom/internal/db"
	"github.com/crueber/loom/internal/favicon"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed static
var staticFiles embed.FS

// cacheControlMiddleware adds cache headers for static assets
func cacheControlMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Aggressive caching for static assets (JS, CSS, libs)
		if r.URL.Path != "/" && (r.URL.Path == "/static/app.js" ||
			r.URL.Path == "/static/styles.css" ||
			r.URL.Path == "/static/lib/pico.min.css" ||
			r.URL.Path == "/static/lib/sortable.min.js") {
			// Cache for 1 year (assets don't change without rebuild)
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	// Load configuration from environment variables
	dbPath := getEnv("DATABASE_PATH", "./data/bookmarks.db")
	port := getEnv("PORT", "8080")
	sessionMaxAge, _ := strconv.Atoi(getEnv("SESSION_MAX_AGE", "31536000")) // 1 year default

	// Determine if we're in production (HTTPS)
	secureCookie := getEnv("SECURE_COOKIE", "false") == "true"

	// Initialize database
	database, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Generate or load session keys
	var authKey []byte
	sessionKeyHex := getEnv("SESSION_KEY", "")
	if sessionKeyHex == "" {
		authKey, err = auth.GenerateKey(32)
		if err != nil {
			log.Fatalf("Failed to generate auth key: %v", err)
		}
		log.Println("WARNING: Using auto-generated session key. Set SESSION_KEY environment variable for production.")
	} else {
		authKey, err = hex.DecodeString(sessionKeyHex)
		if err != nil {
			log.Fatalf("Failed to decode SESSION_KEY: %v", err)
		}
		if len(authKey) != 32 && len(authKey) != 64 {
			log.Fatalf("SESSION_KEY must be 32 or 64 bytes (64 or 128 hex characters), got %d bytes", len(authKey))
		}
	}

	var encryptionKey []byte
	encryptionKeyHex := getEnv("ENCRYPTION_KEY", "")
	if encryptionKeyHex == "" {
		encryptionKey, err = auth.GenerateKey(32)
		if err != nil {
			log.Fatalf("Failed to generate encryption key: %v", err)
		}
		log.Println("WARNING: Using auto-generated encryption key. Set ENCRYPTION_KEY environment variable for production.")
	} else {
		encryptionKey, err = hex.DecodeString(encryptionKeyHex)
		if err != nil {
			log.Fatalf("Failed to decode ENCRYPTION_KEY: %v", err)
		}
		if len(encryptionKey) != 16 && len(encryptionKey) != 24 && len(encryptionKey) != 32 {
			log.Fatalf("ENCRYPTION_KEY must be 16, 24, or 32 bytes (32, 48, or 64 hex characters), got %d bytes", len(encryptionKey))
		}
	}

	// Initialize session manager
	sessionManager := auth.NewSessionManager(authKey, encryptionKey, sessionMaxAge, secureCookie)

	// Initialize favicon fetcher
	faviconFetcher := favicon.New()

	// Initialize API handlers
	authAPI := api.NewAuthAPI(database, sessionManager)
	listsAPI := api.NewListsAPI(database)
	bookmarksAPI := api.NewBookmarksAPI(database, faviconFetcher)
	exportAPI := api.NewExportAPI(database)
	dataAPI := api.NewDataAPI(database)

	// Set up router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// Static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to load static files: %v", err)
	}
	r.Handle("/static/*", cacheControlMiddleware(http.StripPrefix("/static/", http.FileServer(http.FS(staticFS)))))

	// Serve index.html for root and board routes
	serveApp := func(w http.ResponseWriter, r *http.Request) {
		data, err := staticFiles.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "Failed to load page", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	}
	r.Get("/", serveApp)
	r.Get("/boards/{id}", serveApp)

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Public routes
		r.Post("/login", authAPI.HandleLogin)
		r.Post("/register", authAPI.HandleRegister)

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(authAPI.AuthMiddleware)

			// Auth
			r.Post("/logout", authAPI.HandleLogout)
			r.Get("/user", authAPI.HandleGetUser)

			// Combined data endpoint (single request for all lists + bookmarks)
			r.Get("/data", dataAPI.HandleGetAllData)

			// Boards
			r.Get("/boards", api.GetBoards(database))
			r.Post("/boards", api.CreateBoard(database))
			r.Get("/boards/{id}", api.GetBoard(database))
			r.Put("/boards/{id}", api.UpdateBoard(database))
			r.Delete("/boards/{id}", api.DeleteBoard(database))
			r.Get("/boards/{id}/data", api.GetBoardData(database))

			// Lists
			r.Get("/lists", listsAPI.HandleGetLists)
			r.Post("/lists", listsAPI.HandleCreateList)
			r.Put("/lists/{id}", listsAPI.HandleUpdateList)
			r.Delete("/lists/{id}", listsAPI.HandleDeleteList)
			r.Put("/lists/reorder", listsAPI.HandleReorderLists)

			// Bookmarks
			r.Get("/lists/{list_id}/bookmarks", bookmarksAPI.HandleGetBookmarks)
			r.Post("/bookmarks", bookmarksAPI.HandleCreateBookmark)
			r.Put("/bookmarks/{id}", bookmarksAPI.HandleUpdateBookmark)
			r.Delete("/bookmarks/{id}", bookmarksAPI.HandleDeleteBookmark)
			r.Put("/bookmarks/reorder", bookmarksAPI.HandleReorderBookmarks)

			// Export/Import
			r.Get("/export", exportAPI.HandleExport)
			r.Post("/import", exportAPI.HandleImport)
		})
	})

	// Start cleanup routine for expired sessions
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := database.CleanExpiredSessions(); err != nil {
				log.Printf("Failed to clean expired sessions: %v", err)
			}
		}
	}()

	// Start server
	addr := ":" + port
	log.Printf("Server starting on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
