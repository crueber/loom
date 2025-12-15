package main

import (
	"embed"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/crueber/loom/internal/api"
	"github.com/crueber/loom/internal/auth"
	"github.com/crueber/loom/internal/db"
	"github.com/crueber/loom/internal/oauth"
)

//go:embed static
var staticFiles embed.FS

// BuildVersion is set at build time via -ldflags
var BuildVersion string = "dev"

func main() {
	// Load build version from embedded version file
	if versionData, err := staticFiles.ReadFile("static/dist/version.txt"); err == nil {
		BuildVersion = strings.TrimSpace(string(versionData))
	}
	log.Printf("Build version: %s", BuildVersion)

	// Load and validate configuration
	cfg, err := LoadConfig(BuildVersion)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize core services
	database, sessionManager, oauthClient := initializeServices(cfg)
	defer database.Close()

	// Setup application handler
	appHandler := NewAppHandler(staticFiles, database, sessionManager, cfg.BuildVersion)

	// Setup API handlers
	authAPI := api.NewAuthAPI(database, sessionManager, oauthClient)
	dataAPI := api.NewDataAPI(database)

	// Configure router
	router := SetupRouter(&RouterDependencies{
		StaticFiles: staticFiles,
		Database:    database,
		AuthAPI:     authAPI,
		DataAPI:     dataAPI,
		AppHandler:  appHandler,
	})

	// Start background cleanup routine
	startCleanupRoutine(database)

	// Start server
	startServer(cfg.Port, router)
}

// initializeServices initializes database, session manager, and OAuth2 client
func initializeServices(cfg *Config) (*db.DB, *auth.SessionManager, *oauth.Client) {
	// Initialize database
	database, err := db.New(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize session manager
	sessionManager := auth.NewSessionManager(
		cfg.AuthKey,
		cfg.EncryptionKey,
		cfg.SessionMaxAge,
		cfg.SecureCookie,
	)

	// Initialize OAuth2 client
	oauthClient, err := oauth.NewClient(
		cfg.OAuth2IssuerURL,
		cfg.OAuth2ClientID,
		cfg.OAuth2ClientSecret,
		cfg.OAuth2RedirectURL,
	)
	if err != nil {
		log.Fatalf("Failed to initialize OAuth2 client: %v", err)
	}
	log.Printf("OAuth2 client initialized with issuer: %s", cfg.OAuth2IssuerURL)

	return database, sessionManager, oauthClient
}

// startCleanupRoutine starts a background goroutine to clean expired sessions
func startCleanupRoutine(database *db.DB) {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			if err := database.CleanExpiredSessions(); err != nil {
				log.Printf("Failed to clean expired sessions: %v", err)
			}
		}
	}()
}

// startServer starts the HTTP server
func startServer(port string, handler http.Handler) {
	addr := ":" + port
	log.Printf("Server starting on http://localhost%s", addr)

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
