package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/crueber/loom/internal/db"
	"github.com/crueber/loom/internal/favicon"
	"github.com/crueber/loom/internal/models"
)

func TestHandleCreateItem_BookmarkEmptyTitleAssignsAutoTitle(t *testing.T) {
	itemsAPI, listID, userID, cleanup := newItemsAPITestFixture(t)
	defer cleanup()

	originalFetcher := bookmarkTitleFetcher
	bookmarkTitleFetcher = func(rawURL string) (string, error) {
		return "Example &amp; <b>Title</b>", nil
	}
	defer func() {
		bookmarkTitleFetcher = originalFetcher
	}()

	rec := performCreateItemRequest(t, itemsAPI, userID, map[string]any{
		"list_id":     listID,
		"type":        "bookmark",
		"title":       "",
		"url":         "https://example.com/path",
		"icon_source": "loom",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var item models.Item
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("unmarshal created item: %v", err)
	}
	if item.Title == nil {
		t.Fatalf("title = nil, want auto-generated title")
	}
	if got, want := *item.Title, "Example & Title"; got != want {
		t.Fatalf("title = %q, want %q", got, want)
	}
}

func TestHandleCreateItem_BookmarkOmittedTitleAssignsAutoTitle(t *testing.T) {
	itemsAPI, listID, userID, cleanup := newItemsAPITestFixture(t)
	defer cleanup()

	originalFetcher := bookmarkTitleFetcher
	bookmarkTitleFetcher = func(rawURL string) (string, error) {
		return "Omitted &amp; <i>Title</i>", nil
	}
	defer func() {
		bookmarkTitleFetcher = originalFetcher
	}()

	rec := performCreateItemRequest(t, itemsAPI, userID, map[string]any{
		"list_id":     listID,
		"type":        "bookmark",
		"url":         "https://example.com/omitted-title",
		"icon_source": "loom",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var item models.Item
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("unmarshal created item: %v", err)
	}
	if item.Title == nil {
		t.Fatalf("title = nil, want auto-generated title")
	}
	if got, want := *item.Title, "Omitted & Title"; got != want {
		t.Fatalf("title = %q, want %q", got, want)
	}
}

func TestHandleCreateItem_BookmarkNonEmptyTitlePreserved(t *testing.T) {
	itemsAPI, listID, userID, cleanup := newItemsAPITestFixture(t)
	defer cleanup()

	originalFetcher := bookmarkTitleFetcher
	fetchCalled := false
	bookmarkTitleFetcher = func(rawURL string) (string, error) {
		fetchCalled = true
		return "should-not-be-used", nil
	}
	defer func() {
		bookmarkTitleFetcher = originalFetcher
	}()

	rec := performCreateItemRequest(t, itemsAPI, userID, map[string]any{
		"list_id":     listID,
		"type":        "bookmark",
		"title":       "  User Title  ",
		"url":         "https://example.com/path",
		"icon_source": "loom",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	if fetchCalled {
		t.Fatalf("title fetcher called for non-empty title")
	}

	var item models.Item
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("unmarshal created item: %v", err)
	}
	if item.Title == nil {
		t.Fatalf("title = nil, want preserved user title")
	}
	if got, want := *item.Title, "User Title"; got != want {
		t.Fatalf("title = %q, want %q", got, want)
	}
}

func TestHandleCreateItem_BookmarkTitleExtractionFailureFallsBack(t *testing.T) {
	itemsAPI, listID, userID, cleanup := newItemsAPITestFixture(t)
	defer cleanup()

	originalFetcher := bookmarkTitleFetcher
	bookmarkTitleFetcher = func(rawURL string) (string, error) {
		return "", errors.New("fetch failed")
	}
	defer func() {
		bookmarkTitleFetcher = originalFetcher
	}()

	rec := performCreateItemRequest(t, itemsAPI, userID, map[string]any{
		"list_id":     listID,
		"type":        "bookmark",
		"title":       "",
		"url":         "https://fallback.example.org/some/path",
		"icon_source": "loom",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var item models.Item
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("unmarshal created item: %v", err)
	}
	if item.Title == nil {
		t.Fatalf("title = nil, want fallback title")
	}
	if got, want := *item.Title, "fallback.example.org"; got != want {
		t.Fatalf("title = %q, want %q", got, want)
	}
}

func TestHandleCreateItem_BookmarkInvalidURLReturnsSameValidationError(t *testing.T) {
	itemsAPI, listID, userID, cleanup := newItemsAPITestFixture(t)
	defer cleanup()

	originalFetcher := bookmarkTitleFetcher
	fetchCalled := false
	bookmarkTitleFetcher = func(rawURL string) (string, error) {
		fetchCalled = true
		return "unused", nil
	}
	defer func() {
		bookmarkTitleFetcher = originalFetcher
	}()

	rec := performCreateItemRequest(t, itemsAPI, userID, map[string]any{
		"list_id":     listID,
		"type":        "bookmark",
		"title":       "",
		"url":         "notaurl",
		"icon_source": "loom",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	if fetchCalled {
		t.Fatalf("title fetcher called for invalid URL")
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if got, want := errResp.Error, "Invalid URL"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestNormalizeBookmarkTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "entity decoding",
			input: "A &amp; B",
			want:  "A & B",
		},
		{
			name:  "whitespace normalization",
			input: "  Hello\n\t   world   ",
			want:  "Hello world",
		},
		{
			name:  "plain text output strips html tags",
			input: "<div>Hello <strong>world</strong></div>",
			want:  "Hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeBookmarkTitle(tt.input)
			if got != tt.want {
				t.Fatalf("normalizeBookmarkTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if strings.Contains(got, "<") || strings.Contains(got, ">") {
				t.Fatalf("normalizeBookmarkTitle(%q) returned non-plain-text output %q", tt.input, got)
			}
		})
	}
}

func newItemsAPITestFixture(t *testing.T) (*ItemsAPI, int, int, func()) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "bookmarks.db")
	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("create test db: %v", err)
	}

	user, err := database.CreateUser("test-user", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	board, err := database.CreateBoard(user.ID, "Test Board", true)
	if err != nil {
		t.Fatalf("create board: %v", err)
	}

	list, err := database.CreateList(user.ID, board.ID, "Test List", "#ffffff", 0)
	if err != nil {
		t.Fatalf("create list: %v", err)
	}

	itemsAPI := NewItemsAPI(database, favicon.New())

	cleanup := func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	}

	return itemsAPI, list.ID, user.ID, cleanup
}

func performCreateItemRequest(t *testing.T, itemsAPI *ItemsAPI, userID int, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/items", bytes.NewReader(body))
	req = req.WithContext(setUserID(req.Context(), userID))
	rec := httptest.NewRecorder()

	itemsAPI.HandleCreateItem(rec, req)

	return rec
}
