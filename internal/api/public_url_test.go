package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// OSS-137: configurable public base URL + callback display.
func TestPublicURLSettingsAndLogin(t *testing.T) {
	_, mux := newTestServer(t)
	var prov *httptest.Server
	prov = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"issuer":%q,"authorization_endpoint":"%s/auth","token_endpoint":"%s/token","jwks_uri":"%s/jwks"}`,
			prov.URL, prov.URL, prov.URL, prov.URL)
	}))
	defer prov.Close()

	// Trailing slash is normalized away.
	rec := do(t, mux, "PUT", "/api/auth/settings", map[string]any{
		"issuer": "https://issuer.test", "client_id": "cid",
		"client_secret": "shh", "public_url": "https://public.example.com/",
	})
	if rec.Code != 200 {
		t.Fatalf("PUT public_url: want 200, got %d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		PublicURL   string `json:"public_url"`
		CallbackURL string `json:"callback_url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.PublicURL != "https://public.example.com" {
		t.Fatalf("trailing slash not trimmed: %q", got.PublicURL)
	}

	// Non-http(s) rejected with 400.
	if rec := do(t, mux, "PUT", "/api/auth/settings", map[string]any{"public_url": "ftp://x/"}); rec.Code != 400 {
		t.Fatalf("bad public_url: want 400, got %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, mux, "PUT", "/api/auth/settings", map[string]any{"public_url": "notaurl"}); rec.Code != 400 {
		t.Fatalf("bare public_url: want 400, got %d %s", rec.Code, rec.Body.String())
	}

	// GET returns effective callback for that request.
	rec = do(t, mux, "GET", "/api/auth/settings", nil)
	var get struct {
		PublicURL   string `json:"public_url"`
		CallbackURL string `json:"callback_url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &get); err != nil {
		t.Fatal(err)
	}
	if get.CallbackURL != "https://public.example.com/api/auth/callback" {
		t.Fatalf("callback_url mismatch: %q", get.CallbackURL)
	}

	// Empty unsets (fallback mode).
	if rec := do(t, mux, "PUT", "/api/auth/settings", map[string]any{"public_url": "  "}); rec.Code != 200 {
		t.Fatalf("clear public_url: want 200, got %d", rec.Code)
	}
	rec = do(t, mux, "GET", "/api/auth/settings", nil)
	var get2 struct {
		PublicURL   string `json:"public_url"`
		CallbackURL string `json:"callback_url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &get2); err != nil {
		t.Fatal(err)
	}
	if get2.PublicURL != "" {
		t.Fatalf("expected unset public_url, got %q", get2.PublicURL)
	}
	if get2.CallbackURL != "http://example.com/api/auth/callback" {
		t.Fatalf("fallback callback mismatch: %q", get2.CallbackURL)
	}

	// Login redirect_uri uses public_url when set (stub discovery, no external provider).
	rec = do(t, mux, "PUT", "/api/auth/settings", map[string]any{
		"issuer": prov.URL, "client_id": "cid", "client_secret": "shh",
		"enabled": true, "public_url": "https://public.example.com/",
	})
	if rec.Code != 200 {
		t.Fatalf("enable with stub issuer: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, mux, "GET", "/api/auth/login", nil)
	if rec.Code != http.StatusFound {
		t.Fatalf("login: want 302, got %d %s", rec.Code, rec.Body.String())
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	ru := loc.Query().Get("redirect_uri")
	if ru != "https://public.example.com/api/auth/callback" {
		t.Fatalf("redirect_uri mismatch: %q (loc %q)", ru, rec.Header().Get("Location"))
	}
	if !strings.HasPrefix(loc.String(), prov.URL+"/auth") {
		t.Fatalf("expected stub provider auth endpoint, got %q", loc.String())
	}
}
