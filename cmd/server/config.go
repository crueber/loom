package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
)

// Config holds all application configuration
type Config struct {
	// Server settings
	Port          string
	BuildVersion  string
	SecureCookie  bool
	SessionMaxAge int

	// Database
	DatabasePath string

	// Session keys
	AuthKey       []byte
	EncryptionKey []byte

	// OAuth2 settings
	OAuth2IssuerURL    string
	OAuth2ClientID     string
	OAuth2ClientSecret string
	OAuth2RedirectURL  string

	// Standalone mode
	IsStandalone bool
}

// LoadConfig loads and validates configuration from environment variables
func LoadConfig(buildVersion string) (*Config, error) {
	cfg := &Config{
		BuildVersion: buildVersion,
		Port:         getEnv("PORT", "8080"),
		DatabasePath: getEnv("DATABASE_PATH", "./data/bookmarks.db"),
		SecureCookie: getEnv("SECURE_COOKIE", "false") == "true",
	}

	// Parse session max age
	sessionMaxAge, err := strconv.Atoi(getEnv("SESSION_MAX_AGE", "31536000"))
	if err != nil {
		return nil, fmt.Errorf("invalid SESSION_MAX_AGE: %w", err)
	}
	cfg.SessionMaxAge = sessionMaxAge

	// Load OAuth2 configuration
	cfg.OAuth2IssuerURL = os.Getenv("OAUTH2_ISSUER_URL")
	cfg.OAuth2ClientID = os.Getenv("OAUTH2_CLIENT_ID")
	cfg.OAuth2ClientSecret = os.Getenv("OAUTH2_CLIENT_SECRET")
	cfg.OAuth2RedirectURL = os.Getenv("OAUTH2_REDIRECT_URL")

	// Check for standalone mode
	if cfg.OAuth2IssuerURL == "" {
		cfg.IsStandalone = true
		log.Println("OAUTH2_ISSUER_URL not set - running in STANDALONE mode")
	} else {
		if cfg.OAuth2ClientID == "" || cfg.OAuth2ClientSecret == "" || cfg.OAuth2RedirectURL == "" {
			return nil, fmt.Errorf("OAUTH2_CLIENT_ID, OAUTH2_CLIENT_SECRET, and OAUTH2_REDIRECT_URL must be set when OAUTH2_ISSUER_URL is provided")
		}
	}

	// Load and validate session keys (mandatory)
	sessionKeyHex := os.Getenv("SESSION_KEY")
	encryptionKeyHex := os.Getenv("ENCRYPTION_KEY")

	if sessionKeyHex == "" || encryptionKeyHex == "" {
		return nil, fmt.Errorf("SESSION_KEY and ENCRYPTION_KEY must be set for persistent sessions\nGenerate keys with: openssl rand -hex 32")
	}

	// Decode and validate session key
	authKey, err := hex.DecodeString(sessionKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode SESSION_KEY: %w", err)
	}
	if len(authKey) != 32 {
		return nil, fmt.Errorf("SESSION_KEY must be 32 bytes (64 hex characters), got %d bytes", len(authKey))
	}
	cfg.AuthKey = authKey

	// Decode and validate encryption key
	encryptionKey, err := hex.DecodeString(encryptionKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ENCRYPTION_KEY: %w", err)
	}
	if len(encryptionKey) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be 32 bytes (64 hex characters), got %d bytes", len(encryptionKey))
	}
	cfg.EncryptionKey = encryptionKey

	log.Printf("Configuration loaded - Port: %s, Database: %s", cfg.Port, cfg.DatabasePath)
	return cfg, nil
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
