package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
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
	bootstrapScript := fmt.Sprintf(`<script>window.__BOOTSTRAP_DATA__ = %s;</script>
</head>`, bootstrapData)
	return strings.Replace(html, "</head>", bootstrapScript, 1)
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

	allItems, err := h.database.GetAllItems(userID)
	if err != nil {
		return ""
	}

	// Filter items to only those in lists for this board
	filteredItems := h.filterItemsByBoard(lists, allItems)

	// Build bootstrap data structure
	bootstrapData := map[string]interface{}{
		"board":  board,
		"boards": boards,
		"lists":  lists,
		"items":  filteredItems,
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(bootstrapData)
	if err != nil {
		return ""
	}

	return string(jsonData)
}

// filterItemsByBoard filters items to only include those in the given lists
func (h *AppHandler) filterItemsByBoard(lists []*models.List, allItems []*models.Item) map[int]interface{} {
	// Build a set of list IDs
	listIDs := make(map[int]bool)
	for _, list := range lists {
		listIDs[list.ID] = true
	}

	// Filter items by list membership
	filteredItems := make(map[int]interface{})
	for _, item := range allItems {
		if listIDs[item.ListID] {
			if _, exists := filteredItems[item.ListID]; !exists {
				filteredItems[item.ListID] = []interface{}{}
			}
			listItems := filteredItems[item.ListID].([]interface{})
			filteredItems[item.ListID] = append(listItems, item)
		}
	}

	return filteredItems
}
