package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/crueber/loom-rebuild/internal/auth"
	"github.com/crueber/loom-rebuild/internal/model"
	"github.com/crueber/loom-rebuild/internal/store"
)

// authFixture builds a server with auth enabled and three users:
// owner (owns a private board), editor, viewer, plus one outsider.
func authFixture(t *testing.T) (*Handler, *http.ServeMux, map[string]string) {
	t.Helper()
	st, err := store.OpenFile("")
	if err != nil {
		t.Fatal(err)
	}
	mkUser := func(sub, email string) (model.User, string) {
		u, err := st.UpsertUserBySubject("https://issuer.test", sub, email, sub)
		if err != nil {
			t.Fatal(err)
		}
		raw, hash, err := auth.MintToken()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.CreateSession(u.ID, hash, time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		return u, raw
	}
	owner, ownerTok := mkUser("owner", "owner@test")
	editor, editorTok := mkUser("editor", "editor@test")
	viewer, viewerTok := mkUser("viewer", "viewer@test")
	_, outsiderTok := mkUser("outsider", "outsider@test")

	if _, err := st.UpdateAuthSettings(model.AuthSettings{
		Issuer: "https://issuer.test", ClientID: "cid", ClientSecret: "shh",
		Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	priv, err := st.CreateBoardWithOwner("Private", owner.ID, model.VisibilityPrivate)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := st.CreateBoard("Public")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetBoardMember(priv.ID, editor.ID, model.RoleEditor); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetBoardMember(priv.ID, viewer.ID, model.RoleViewer); err != nil {
		t.Fatal(err)
	}
	h := &Handler{Store: st}
	mux := http.NewServeMux()
	h.Register(mux)
	toks := map[string]string{"owner": ownerTok, "editor": editorTok, "viewer": viewerTok, "outsider": outsiderTok, "": ""}
	ids := map[string]string{"priv": priv.ID, "pub": pub.ID, "ownerID": owner.ID}
	for k, v := range ids {
		toks[k] = v
	}
	return h, mux, toks
}

func doAuth(t *testing.T, mux *http.ServeMux, method, path string, body any, tok string) *httptest.ResponseRecorder {
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
	if tok != "" {
		req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: tok})
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestAuthDisabledIsOpen(t *testing.T) {
	_, mux := newTestServer(t)
	// No login anywhere: status off, boards open, settings editable.
	rec := do(t, mux, "GET", "/api/auth/status", nil)
	var st struct {
		Enabled       bool `json:"enabled"`
		Authenticated bool `json:"authenticated"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st.Enabled || st.Authenticated {
		t.Fatalf("fresh boot must be disabled+anonymous: %+v", st)
	}
	rec = do(t, mux, "POST", "/api/boards", map[string]string{"title": "Open"})
	if rec.Code != 201 {
		t.Fatalf("anonymous create while disabled: %d", rec.Code)
	}
	rec = do(t, mux, "GET", "/api/boards", nil)
	var boards []model.Board
	if err := json.Unmarshal(rec.Body.Bytes(), &boards); err != nil {
		t.Fatal(err)
	}
	if len(boards) != 1 {
		t.Fatalf("expected 1 board, got %d", len(boards))
	}
	// Admin enables OIDC purely via UI settings (no file/flag edits).
	rec = do(t, mux, "PUT", "/api/auth/settings", map[string]any{
		"issuer": "https://issuer.test", "client_id": "cid",
		"client_secret": "shh", "enabled": true,
	})
	if rec.Code != 200 {
		t.Fatalf("enable via settings: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, mux, "GET", "/api/auth/status", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if !st.Enabled {
		t.Fatalf("expected enabled after PUT: %+v", st)
	}
}

func TestBoardScoping(t *testing.T) {
	_, mux, tk := authFixture(t)
	// Anonymous sees only the public board.
	rec := doAuth(t, mux, "GET", "/api/boards", nil, tk[""])
	var boards []model.Board
	if err := json.Unmarshal(rec.Body.Bytes(), &boards); err != nil {
		t.Fatal(err)
	}
	if len(boards) != 1 || boards[0].ID != tk["pub"] {
		t.Fatalf("anon must see only public: %+v", boards)
	}
	// Private tree: 401 anon, 403 outsider, 200 member/owner.
	if rec := doAuth(t, mux, "GET", "/api/boards/"+tk["priv"], nil, tk[""]); rec.Code != 401 {
		t.Fatalf("anon tree: want 401, got %d", rec.Code)
	}
	if rec := doAuth(t, mux, "GET", "/api/boards/"+tk["priv"], nil, tk["outsider"]); rec.Code != 403 {
		t.Fatalf("outsider tree: want 403, got %d", rec.Code)
	}
	for _, who := range []string{"owner", "editor", "viewer"} {
		if rec := doAuth(t, mux, "GET", "/api/boards/"+tk["priv"], nil, tk[who]); rec.Code != 200 {
			t.Fatalf("%s tree: want 200, got %d", who, rec.Code)
		}
	}
	// Owner sees both boards.
	rec = doAuth(t, mux, "GET", "/api/boards", nil, tk["owner"])
	if err := json.Unmarshal(rec.Body.Bytes(), &boards); err != nil {
		t.Fatal(err)
	}
	if len(boards) != 2 {
		t.Fatalf("owner must see 2 boards, got %d", len(boards))
	}
}

func TestViewerEditorOwnerWrites(t *testing.T) {
	_, mux, tk := authFixture(t)
	// Viewer reads but cannot write.
	if rec := doAuth(t, mux, "POST", "/api/boards/"+tk["priv"]+"/columns", map[string]string{"title": "V"}, tk["viewer"]); rec.Code != 403 {
		t.Fatalf("viewer create column: want 403, got %d", rec.Code)
	}
	// Editor writes.
	rec := doAuth(t, mux, "POST", "/api/boards/"+tk["priv"]+"/columns", map[string]string{"title": "E"}, tk["editor"])
	if rec.Code != 201 {
		t.Fatalf("editor create column: want 201, got %d %s", rec.Code, rec.Body.String())
	}
	var col model.Column
	if err := json.Unmarshal(rec.Body.Bytes(), &col); err != nil {
		t.Fatal(err)
	}
	// Viewer cannot add cards; editor can.
	if rec := doAuth(t, mux, "POST", "/api/columns/"+col.ID+"/cards", map[string]any{"blocks": []any{}}, tk["viewer"]); rec.Code != 403 {
		t.Fatalf("viewer create card: want 403, got %d", rec.Code)
	}
	rec = doAuth(t, mux, "POST", "/api/columns/"+col.ID+"/cards", map[string]any{"blocks": []any{}}, tk["editor"])
	if rec.Code != 201 {
		t.Fatalf("editor create card: want 201, got %d", rec.Code)
	}
	// Visibility: owner only.
	if rec := doAuth(t, mux, "PATCH", "/api/boards/"+tk["priv"], map[string]string{"visibility": "public"}, tk["editor"]); rec.Code != 403 {
		t.Fatalf("editor visibility: want 403, got %d", rec.Code)
	}
	rec = doAuth(t, mux, "PATCH", "/api/boards/"+tk["priv"], map[string]string{"visibility": "public"}, tk["owner"])
	if rec.Code != 200 {
		t.Fatalf("owner visibility: want 200, got %d %s", rec.Code, rec.Body.String())
	}
	// Now public: anonymous reads, but still cannot write.
	if rec := doAuth(t, mux, "GET", "/api/boards/"+tk["priv"], nil, tk[""]); rec.Code != 200 {
		t.Fatalf("public tree anon: want 200, got %d", rec.Code)
	}
	if rec := doAuth(t, mux, "POST", "/api/boards/"+tk["priv"]+"/columns", map[string]string{"title": "X"}, tk[""]); rec.Code != 401 {
		t.Fatalf("anon write public: want 401, got %d", rec.Code)
	}
}

func TestMembersAndSettingsAndMe(t *testing.T) {
	_, mux, tk := authFixture(t)
	// /me: 401 anon, 200 with session.
	if rec := doAuth(t, mux, "GET", "/api/me", nil, tk[""]); rec.Code != 401 {
		t.Fatalf("anon /me: want 401, got %d", rec.Code)
	}
	rec := doAuth(t, mux, "GET", "/api/me", nil, tk["owner"])
	if rec.Code != 200 {
		t.Fatalf("owner /me: want 200, got %d", rec.Code)
	}
	// Settings need a session once enabled, and never leak the secret.
	if rec := doAuth(t, mux, "GET", "/api/auth/settings", nil, tk[""]); rec.Code != 401 {
		t.Fatalf("anon settings: want 401, got %d", rec.Code)
	}
	rec = doAuth(t, mux, "GET", "/api/auth/settings", nil, tk["owner"])
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if _, leaked := raw["client_secret"]; leaked {
		t.Fatalf("secret leaked in settings GET: %v", raw)
	}
	if has, _ := raw["has_secret"].(bool); !has {
		t.Fatalf("expected has_secret=true: %v", raw)
	}
	// PUT without secret keeps the stored one (still enabled-valid).
	rec = doAuth(t, mux, "PUT", "/api/auth/settings", map[string]any{"require_auth": true}, tk["owner"])
	if rec.Code != 200 {
		t.Fatalf("settings PUT w/o secret: want 200, got %d %s", rec.Code, rec.Body.String())
	}
	// Non-owner cannot manage members; owner invites by email.
	if rec := doAuth(t, mux, "POST", "/api/boards/"+tk["priv"]+"/members", map[string]string{"email": "outsider@test", "role": "viewer"}, tk["editor"]); rec.Code != 403 {
		t.Fatalf("editor invite: want 403, got %d", rec.Code)
	}
	rec = doAuth(t, mux, "POST", "/api/boards/"+tk["priv"]+"/members", map[string]string{"email": "outsider@test", "role": "viewer"}, tk["owner"])
	if rec.Code != 201 {
		t.Fatalf("owner invite: want 201, got %d %s", rec.Code, rec.Body.String())
	}
	var invited model.BoardMember
	if err := json.Unmarshal(rec.Body.Bytes(), &invited); err != nil {
		t.Fatal(err)
	}
	// Invited outsider can now read the private tree.
	if rec := doAuth(t, mux, "GET", "/api/boards/"+tk["priv"], nil, tk["outsider"]); rec.Code != 200 {
		t.Fatalf("invited outsider tree: want 200, got %d", rec.Code)
	}
	// Role change + removal.
	if rec := doAuth(t, mux, "PATCH", "/api/boards/"+tk["priv"]+"/members/"+invited.UserID, map[string]string{"role": "editor"}, tk["owner"]); rec.Code != 200 {
		t.Fatalf("role change: want 200, got %d %s", rec.Code, rec.Body.String())
	}
	if rec := doAuth(t, mux, "DELETE", "/api/boards/"+tk["priv"]+"/members/"+invited.UserID, nil, tk["owner"]); rec.Code != 204 {
		t.Fatalf("member remove: want 204, got %d", rec.Code)
	}
	if rec := doAuth(t, mux, "GET", "/api/boards/"+tk["priv"], nil, tk["outsider"]); rec.Code != 403 {
		t.Fatalf("removed outsider tree: want 403, got %d", rec.Code)
	}
	// Logout ends the session.
	if rec := doAuth(t, mux, "POST", "/api/auth/logout", nil, tk["owner"]); rec.Code != 204 {
		t.Fatalf("logout: want 204, got %d", rec.Code)
	}
	if rec := doAuth(t, mux, "GET", "/api/me", nil, tk["owner"]); rec.Code != 401 {
		t.Fatalf("me after logout: want 401, got %d", rec.Code)
	}
}
