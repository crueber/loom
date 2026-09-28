package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/crueber/loom-rebuild/internal/model"
	"github.com/crueber/loom-rebuild/internal/store"
)

func newTestServer(t *testing.T) (*Handler, *http.ServeMux) {
	t.Helper()
	st, err := store.OpenFile("")
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{Store: st}
	mux := http.NewServeMux()
	h.Register(mux)
	return h, mux
}

func do(t *testing.T, mux *http.ServeMux, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var r *bytes.Reader
	if body == nil {
		r = bytes.NewReader(nil)
	} else {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, r)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestBoardColumnCardCRUD(t *testing.T) {
	_, mux := newTestServer(t)

	rec := do(t, mux, "POST", "/api/boards", map[string]string{"title": "Home"})
	if rec.Code != 201 {
		t.Fatalf("create board: %d %s", rec.Code, rec.Body.String())
	}
	var board model.Board
	if err := json.Unmarshal(rec.Body.Bytes(), &board); err != nil {
		t.Fatal(err)
	}

	rec = do(t, mux, "POST", "/api/boards/"+board.ID+"/columns", map[string]string{"title": "Links", "color": "#4c8dff"})
	if rec.Code != 201 {
		t.Fatalf("create column: %d %s", rec.Code, rec.Body.String())
	}
	var col model.Column
	if err := json.Unmarshal(rec.Body.Bytes(), &col); err != nil {
		t.Fatal(err)
	}

	rec = do(t, mux, "POST", "/api/columns/"+col.ID+"/cards", map[string]any{
		"blocks": []model.Block{model.NewLinkBlock("https://example.com", "Example")},
	})
	if rec.Code != 201 {
		t.Fatalf("create card: %d %s", rec.Code, rec.Body.String())
	}
	var card model.Card
	if err := json.Unmarshal(rec.Body.Bytes(), &card); err != nil {
		t.Fatal(err)
	}
	if len(card.Blocks) != 1 || card.Blocks[0].Type != model.BlockLink {
		t.Fatalf("unexpected blocks: %+v", card.Blocks)
	}

	// Inline edit: append a note block, PATCH full card.
	card.Blocks = append(card.Blocks, model.NewNoteBlock("hello **world**"))
	rec = do(t, mux, "PATCH", "/api/cards/"+card.ID, card)
	if rec.Code != 200 {
		t.Fatalf("update card: %d %s", rec.Code, rec.Body.String())
	}

	rec = do(t, mux, "GET", "/api/boards/"+board.ID, nil)
	if rec.Code != 200 {
		t.Fatalf("get tree: %d %s", rec.Code, rec.Body.String())
	}
	var tree model.BoardTree
	if err := json.Unmarshal(rec.Body.Bytes(), &tree); err != nil {
		t.Fatal(err)
	}
	if len(tree.Columns) != 1 || len(tree.Cards[col.ID]) != 1 || len(tree.Cards[col.ID][0].Blocks) != 2 {
		t.Fatalf("unexpected tree: %+v", tree)
	}
}

func TestBoardStylingAndEmptyTreeShape(t *testing.T) {
	_, mux := newTestServer(t)

	rec := do(t, mux, "POST", "/api/boards", map[string]string{"title": "Styled"})
	if rec.Code != 201 {
		t.Fatalf("create board: %d %s", rec.Code, rec.Body.String())
	}
	var board model.Board
	if err := json.Unmarshal(rec.Body.Bytes(), &board); err != nil {
		t.Fatal(err)
	}

	// Empty board tree must encode columns as [] (not null) so the
	// client can render per-board without throwing.
	rec = do(t, mux, "GET", "/api/boards/"+board.ID, nil)
	if rec.Code != 200 {
		t.Fatalf("get tree: %d %s", rec.Code, rec.Body.String())
	}
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if cols, ok := raw["columns"].([]any); !ok || cols == nil {
		t.Fatalf("expected columns [], got %v", raw["columns"])
	}

	// Partial PATCH (background only) must not wipe the title.
	rec = do(t, mux, "PATCH", "/api/boards/"+board.ID, map[string]string{"background": "sage"})
	if rec.Code != 200 {
		t.Fatalf("patch background: %d %s", rec.Code, rec.Body.String())
	}
	var patched model.Board
	if err := json.Unmarshal(rec.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.Background != "sage" || patched.Title != "Styled" {
		t.Fatalf("bad merge: %+v", patched)
	}

	// Title-only PATCH must preserve the background.
	rec = do(t, mux, "PATCH", "/api/boards/"+board.ID, map[string]string{"title": "Renamed"})
	if rec.Code != 200 {
		t.Fatalf("patch title: %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.Title != "Renamed" || patched.Background != "sage" {
		t.Fatalf("bad merge: %+v", patched)
	}
}

func TestV1Import(t *testing.T) {
	_, mux := newTestServer(t)
	v1 := `{"version":1,"lists":[
		{"id":1,"title":"Tech","color":"blue","position":0,"collapsed":false,
		 "bookmarks":[{"id":1,"title":"Ex","url":"https://example.com","position":0}],
		 "notes":[{"id":2,"content":"a note","position":1}],
		 "items":[{"id":3,"type":"bookmark","title":"It","url":"https://item.test","position":2}]}
	]}`
	req := httptest.NewRequest("POST", "/api/import/v1", bytes.NewReader([]byte(v1)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("import v1: %d %s", rec.Code, rec.Body.String())
	}
	var tree model.BoardTree
	if err := json.Unmarshal(rec.Body.Bytes(), &tree); err != nil {
		t.Fatal(err)
	}
	if tree.Columns[0].Color != "blue" {
		t.Fatalf("color not preserved: %+v", tree.Columns[0])
	}
	var cards []model.Card
	for _, cs := range tree.Cards {
		cards = append(cards, cs...)
	}
	if len(cards) != 3 {
		t.Fatalf("expected 3 cards, got %d", len(cards))
	}
	types := map[string]int{}
	for _, c := range cards {
		for _, b := range c.Blocks {
			types[b.Type]++
		}
	}
	if types["link"] != 2 || types["note"] != 1 {
		t.Fatalf("unexpected block types: %v", types)
	}

	rec = do(t, mux, "GET", "/api/export", nil)
	if rec.Code != 200 {
		t.Fatalf("export: %d %s", rec.Code, rec.Body.String())
	}
	var exp struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &exp); err != nil {
		t.Fatal(err)
	}
	if exp.Version != 2 {
		t.Fatalf("expected export v2, got %d", exp.Version)
	}
}
