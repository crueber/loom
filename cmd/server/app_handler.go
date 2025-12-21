package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/crueber/loom/internal/auth"
	"github.com/crueber/loom/internal/db"
	"github.com/crueber/loom/internal/models"
)

// AppHandler handles serving the main application HTML with bootstrapped data
type AppHandler struct {
	staticFiles    embed.FS
	database       *db.DB
	sessionManager *auth.SessionManager
	buildVersion   string
}

// NewAppHandler creates a new app handler
func NewAppHandler(staticFiles embed.FS, database *db.DB, sessionManager *auth.SessionManager, buildVersion string) *AppHandler {
	return &AppHandler{
		staticFiles:    staticFiles,
		database:       database,
		sessionManager: sessionManager,
		buildVersion:   buildVersion,
	}
}

// ServeApp serves the main application HTML with version cache busting and bootstrapped data
func (h *AppHandler) ServeApp(w http.ResponseWriter, r *http.Request) {
	// Load index.html from embedded static files
	data, err := h.staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "Failed to load page", http.StatusInternalServerError)
		return
	}

	// Inject version query strings for cache busting
	html := h.injectVersions(string(data))

	// Inject theme preference
	html = h.injectTheme(html, r)

	// Inject i18n data
	html = h.injectI18nData(html, r)

	// Bootstrap data if user is authenticated
	if bootstrapData := h.getBootstrapData(r); bootstrapData != "" {
		html = h.injectBootstrapData(html, bootstrapData)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// injectVersions adds version query parameters to static assets for cache busting
func (h *AppHandler) injectVersions(html string) string {
	html = strings.ReplaceAll(html,
		`src="/static/dist/app.bundle.js"`,
		fmt.Sprintf(`src="/static/dist/app.bundle.js?v=%s"`, h.buildVersion))
	html = strings.ReplaceAll(html,
		`href="/static/styles.css"`,
		fmt.Sprintf(`href="/static/styles.css?v=%s"`, h.buildVersion))
	return html
}

// injectBootstrapData adds the bootstrap data script to the HTML
func (h *AppHandler) injectBootstrapData(html, bootstrapData string) string {
	bootstrapScript := fmt.Sprintf(`<script>window.__BOOTSTRAP_DATA__ = %s;</script>`, bootstrapData)
	return strings.Replace(html, "<!-- Bootstrap -->", bootstrapScript, 1)
}

// injectTheme adds the data-theme attribute to the HTML tag
func (h *AppHandler) injectTheme(html string, r *http.Request) string {
	theme := "dark" // Default

	if userID, ok := h.sessionManager.GetUserID(r); ok {
		if user, err := h.database.GetUserByID(userID); err == nil && user != nil && user.Theme != "" && user.Theme != "auto" {
			theme = user.Theme
		}
	}
	return strings.Replace(html, `data-theme="dark"`, fmt.Sprintf(`data-theme="%s"`, theme), 1)
}

// injectI18nData adds the i18n data script to the HTML
func (h *AppHandler) injectI18nData(html string, r *http.Request) string {
	locale := h.detectLocale(r)
	translations, err := h.staticFiles.ReadFile(filepath.Join("static/locales", locale+".json"))
	if err != nil {
		// Fallback to English
		translations, _ = h.staticFiles.ReadFile("static/locales/en.json")
	}

	i18nScript := fmt.Sprintf(`<script>window.__I18N_DATA__ = %s;</script>`, string(translations))
	return strings.Replace(html, "<!-- I18n -->", i18nScript, 1)
}

// detectLocale determines the user's locale preference
func (h *AppHandler) detectLocale(r *http.Request) string {
	// 1. Check if user is authenticated and has a preference
	if userID, ok := h.sessionManager.GetUserID(r); ok {
		if user, err := h.database.GetUserByID(userID); err == nil && user != nil && user.Locale != "" {
			return user.Locale
		}
	}

	// 2. Check Accept-Language header
	acceptLang := r.Header.Get("Accept-Language")
	if acceptLang != "" {
		// Simple parser: take the first language tag
		parts := strings.Split(acceptLang, ",")
		if len(parts) > 0 {
			lang := strings.Split(parts[0], "-")[0]
			// Check if we support this language
			if _, err := h.staticFiles.ReadFile(filepath.Join("static/locales", lang+".json")); err == nil {
				return lang
			}
		}
	}

	return "en"
}

// getBootstrapData fetches and serializes bootstrap data for authenticated users
func (h *AppHandler) getBootstrapData(r *http.Request) string {
	// Check if user is authenticated
	userID, ok := h.sessionManager.GetUserID(r)
	if !ok {
		return ""
	}

	// Determine which board to load from the URL
	boardID := h.parseBoardID(r.URL.Path)

	// If no board ID in URL, get the default board
	if boardID == 0 {
		boardID = h.getDefaultBoardID(userID)
	}

	// Fetch and serialize board data
	if boardID > 0 {
		return h.fetchBoardData(userID, boardID)
	}

	return ""
}

// parseBoardID extracts the board ID from the URL path
func (h *AppHandler) parseBoardID(path string) int {
	if strings.HasPrefix(path, "/boards/") {
		parts := strings.Split(path, "/")
		if len(parts) >= 3 {
			if id, err := strconv.Atoi(parts[2]); err == nil {
				return id
			}
		}
	}
	return 0
}

// getDefaultBoardID retrieves the user's default board ID
func (h *AppHandler) getDefaultBoardID(userID int) int {
	boards, err := h.database.GetBoards(userID)
	if err != nil || len(boards) == 0 {
		return 0
	}

	for _, board := range boards {
		if board.IsDefault {
			return board.ID
		}
	}

	return 0
}

// fetchBoardData retrieves all data for a board and serializes it to JSON
func (h *AppHandler) fetchBoardData(userID, boardID int) string {
	// Fetch user data
	user, err := h.database.GetUserByID(userID)
	if err != nil || user == nil {
		return ""
	}

	// Verify board ownership
	board, err := h.database.GetBoardByID(boardID, userID)
	if err != nil || board == nil {
		return ""
	}

	// Fetch all related data
	boards, err := h.database.GetBoards(userID)
	if err != nil {
		return ""
	}

	lists, err := h.database.GetListsByBoard(userID, boardID)
	if err != nil {
		return ""
	}

	// Get items for this board only (efficient single query with JOIN)
	items, err := h.database.GetItemsByBoard(userID, boardID)
	if err != nil {
		return ""
	}

	// Build user object without sensitive data
	userPublic := map[string]any{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
		"locale":   user.Locale,
		"theme":    user.Theme,
	}

	// Build bootstrap data structure
	bootstrapData := map[string]any{
		"user":   userPublic,
		"board":  board,
		"boards": boards,
		"lists":  lists,
		"items":  h.groupItemsByList(items),
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(bootstrapData)
	if err != nil {
		return ""
	}

	return string(jsonData)
}

// groupItemsByList groups items by their list ID
func (h *AppHandler) groupItemsByList(items []*models.Item) map[int]any {
	itemsByList := make(map[int]any)

	for _, item := range items {
		if _, exists := itemsByList[item.ListID]; !exists {
			itemsByList[item.ListID] = []any{}
		}
		listItems := itemsByList[item.ListID].([]any)
		itemsByList[item.ListID] = append(listItems, item)
	}

	return itemsByList
}
