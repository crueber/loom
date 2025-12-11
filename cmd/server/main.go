package main

import (
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/crueber/loom/internal/api"
	"github.com/crueber/loom/internal/auth"
	"github.com/crueber/loom/internal/db"
	"github.com/crueber/loom/internal/favicon"
	"github.com/crueber/loom/internal/oauth"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed static
var staticFiles embed.FS

// BuildVersion is set at build time via -ldflags
var BuildVersion string = "dev"

// cacheControlMiddleware adds cache headers for static assets
func cacheControlMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		hasVersion := r.URL.Query().Get("v") != ""

		// Cache versioned assets for 1 year (they won't change)
		if strings.HasPrefix(path, "/static/") && hasVersion {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else if strings.HasPrefix(path, "/static/lib/") {
			// Cache third-party libs for 1 year (they're already versioned by library authors)
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else if strings.HasPrefix(path, "/static/") {
			// Cache unversioned assets for 5 minutes
			w.Header().Set("Cache-Control", "public, max-age=300")
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	// Load build version from embedded version file
	if versionData, err := staticFiles.ReadFile("static/dist/version.txt"); err == nil {
		BuildVersion = strings.TrimSpace(string(versionData))
	}
	log.Printf("Build version: %s", BuildVersion)

	// Load configuration from environment variables
	dbPath := getEnv("DATABASE_PATH", "./data/bookmarks.db")
	port := getEnv("PORT", "8080")
	sessionMaxAge, _ := strconv.Atoi(getEnv("SESSION_MAX_AGE", "31536000")) // 1 year default

	// Determine if we're in production (HTTPS)
	secureCookie := getEnv("SECURE_COOKIE", "false") == "true"

	// OAuth2 configuration (mandatory)
	oauth2IssuerURL := os.Getenv("OAUTH2_ISSUER_URL")
	oauth2ClientID := os.Getenv("OAUTH2_CLIENT_ID")
	oauth2ClientSecret := os.Getenv("OAUTH2_CLIENT_SECRET")
	oauth2RedirectURL := os.Getenv("OAUTH2_REDIRECT_URL")

	if oauth2IssuerURL == "" || oauth2ClientID == "" || oauth2ClientSecret == "" || oauth2RedirectURL == "" {
		log.Fatal("OAUTH2_ISSUER_URL, OAUTH2_CLIENT_ID, OAUTH2_CLIENT_SECRET, and OAUTH2_REDIRECT_URL must be set")
	}

	// Initialize database
	database, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Load session keys (mandatory - no auto-generation)
	sessionKeyHex := os.Getenv("SESSION_KEY")
	encryptionKeyHex := os.Getenv("ENCRYPTION_KEY")

	if sessionKeyHex == "" || encryptionKeyHex == "" {
		log.Fatal("SESSION_KEY and ENCRYPTION_KEY must be set for persistent sessions.\n" +
			"Generate keys with: openssl rand -hex 32")
	}

	// Decode and validate session key
	authKey, err := hex.DecodeString(sessionKeyHex)
	if err != nil {
		log.Fatalf("Failed to decode SESSION_KEY: %v", err)
	}
	if len(authKey) != 32 {
		log.Fatalf("SESSION_KEY must be 32 bytes (64 hex characters), got %d bytes", len(authKey))
	}

	// Decode and validate encryption key
	encryptionKey, err := hex.DecodeString(encryptionKeyHex)
	if err != nil {
		log.Fatalf("Failed to decode ENCRYPTION_KEY: %v", err)
	}
	if len(encryptionKey) != 32 {
		log.Fatalf("ENCRYPTION_KEY must be 32 bytes (64 hex characters), got %d bytes", len(encryptionKey))
	}

	// Initialize session manager
	sessionManager := auth.NewSessionManager(authKey, encryptionKey, sessionMaxAge, secureCookie)

	// Initialize OAuth2 client
	oauthClient, err := oauth.NewClient(oauth2IssuerURL, oauth2ClientID, oauth2ClientSecret, oauth2RedirectURL)
	if err != nil {
		log.Fatalf("Failed to initialize OAuth2 client: %v", err)
	}
	log.Printf("OAuth2 client initialized with issuer: %s", oauth2IssuerURL)

	// Initialize favicon fetcher
	faviconFetcher := favicon.New()

	// Initialize API handlers
	authAPI := api.NewAuthAPI(database, sessionManager, oauthClient)
	listsAPI := api.NewListsAPI(database)
	bookmarksAPI := api.NewBookmarksAPI(database, faviconFetcher)
	itemsAPI := api.NewItemsAPI(database, faviconFetcher)
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

		// Inject version query strings for cache busting
		html := string(data)
		html = strings.ReplaceAll(html,
			`src="/static/dist/app.bundle.js"`,
			fmt.Sprintf(`src="/static/dist/app.bundle.js?v=%s"`, BuildVersion))
		html = strings.ReplaceAll(html,
			`href="/static/styles.css"`,
			fmt.Sprintf(`href="/static/styles.css?v=%s"`, BuildVersion))

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
	}
	r.Get("/", serveApp)
	r.Get("/boards/{id}", serveApp)

	// OAuth2 routes (outside /api)
	r.Get("/auth/login", authAPI.HandleOAuthLogin)
	r.Get("/auth/callback", authAPI.HandleOAuthCallback)

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Public routes (deprecated - will be removed)
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
			r.Post("/lists/{id}/copy-or-move", listsAPI.HandleCopyOrMoveList)

			// Bookmarks (backward compatibility, deprecated)
			r.Get("/lists/{list_id}/bookmarks", bookmarksAPI.HandleGetBookmarks)
			r.Post("/bookmarks", bookmarksAPI.HandleCreateBookmark)
			r.Put("/bookmarks/{id}", bookmarksAPI.HandleUpdateBookmark)
			r.Delete("/bookmarks/{id}", bookmarksAPI.HandleDeleteBookmark)
			r.Put("/bookmarks/reorder", bookmarksAPI.HandleReorderBookmarks)

			// Items (unified bookmarks and notes)
			r.Get("/lists/{list_id}/items", itemsAPI.HandleGetItems)
			r.Post("/items", itemsAPI.HandleCreateItem)
			r.Put("/items/{id}", itemsAPI.HandleUpdateItem)
			r.Delete("/items/{id}", itemsAPI.HandleDeleteItem)
			r.Put("/items/reorder", itemsAPI.HandleReorderItems)

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
