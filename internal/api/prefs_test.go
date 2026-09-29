package api

import (
	"encoding/json"
	"testing"

	"github.com/crueber/loom-rebuild/internal/model"
)

// User prefs (OSS-83): anonymous -> 401, signed-in GET defaults,
// PUT roundtrip persists, unknown theme normalizes to paper.
func TestMePrefsRoundtrip(t *testing.T) {
	_, mux, toks := authFixture(t)

	if rec := doAuth(t, mux, "GET", "/api/me/prefs", nil, ""); rec.Code != 401 {
		t.Fatalf("anonymous GET /api/me/prefs: want 401, got %d", rec.Code)
	}
	if rec := doAuth(t, mux, "PUT", "/api/me/prefs", map[string]string{"theme": "sky"}, ""); rec.Code != 401 {
		t.Fatalf("anonymous PUT /api/me/prefs: want 401, got %d", rec.Code)
	}

	rec := doAuth(t, mux, "GET", "/api/me/prefs", nil, toks["owner"])
	if rec.Code != 200 {
		t.Fatalf("GET defaults: %d %s", rec.Code, rec.Body.String())
	}
	var def model.UserPrefs
	if err := json.Unmarshal(rec.Body.Bytes(), &def); err != nil {
		t.Fatal(err)
	}
	if def.Theme != "paper" || def.Language != "en" {
		t.Fatalf("defaults: got %+v", def)
	}

	rec = doAuth(t, mux, "PUT", "/api/me/prefs", map[string]string{"theme": "sky", "language": "xx"}, toks["owner"])
	if rec.Code != 200 {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body.String())
	}
	var stored model.UserPrefs
	if err := json.Unmarshal(rec.Body.Bytes(), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Theme != "sky" || stored.Language != "en" {
		t.Fatalf("stored: got %+v", stored)
	}

	rec = doAuth(t, mux, "GET", "/api/me/prefs", nil, toks["owner"])
	var back model.UserPrefs
	if err := json.Unmarshal(rec.Body.Bytes(), &back); err != nil {
		t.Fatal(err)
	}
	if back != stored {
		t.Fatalf("roundtrip: wrote %+v, read %+v", stored, back)
	}

	// Unknown theme falls back to paper.
	rec = doAuth(t, mux, "PUT", "/api/me/prefs", map[string]string{"theme": "neon"}, toks["owner"])
	var normed model.UserPrefs
	if err := json.Unmarshal(rec.Body.Bytes(), &normed); err != nil {
		t.Fatal(err)
	}
	if normed.Theme != "paper" {
		t.Fatalf("unknown theme: got %+v", normed)
	}
}
