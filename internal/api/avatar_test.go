package api

import (
	"strings"
	"testing"
)

func TestAvatarDeterministicAndSeeded(t *testing.T) {
	_, mux := newTestServer(t)
	get := func(seed string) (int, string, string) {
		rec := do(t, mux, "GET", "/avatar/"+seed+".svg", nil)
		return rec.Code, rec.Header().Get("Content-Type"), rec.Body.String()
	}
	code, ct, a := get("abc123")
	if code != 200 {
		t.Fatalf("got %d, want 200", code)
	}
	if ct != "image/svg+xml" {
		t.Fatalf("content-type = %q, want image/svg+xml", ct)
	}
	if _, _, b := get("abc123"); b != a {
		t.Fatalf("same seed gave different bytes")
	}
	if _, _, c := get("xyz789"); c == a {
		t.Fatalf("different seeds gave identical bytes")
	}
	if !strings.HasPrefix(a, "<svg") || len(a) < 500 {
		t.Fatalf("not a landscape svg (%d bytes): %q", len(a), a[:80])
	}
}

func TestAvatarEmailSeed(t *testing.T) {
	_, mux := newTestServer(t)
	rec := do(t, mux, "GET", "/avatar/user@example.com.svg", nil)
	if rec.Code != 200 {
		t.Fatalf("got %d, want 200", rec.Code)
	}
}

func TestAvatarInvalid(t *testing.T) {
	_, mux := newTestServer(t)
	for path, want := range map[string]int{
		"/avatar/.svg": 400,
		"/avatar/" + strings.Repeat("x", 300) + ".svg": 400,
		"/avatar/a/b.svg":  404,
		"/avatar/nosuffix": 404,
	} {
		rec := do(t, mux, "GET", path, nil)
		if rec.Code != want {
			t.Fatalf("%s: got %d, want %d", path, rec.Code, want)
		}
	}
	rec := do(t, mux, "POST", "/avatar/abc.svg", nil)
	if rec.Code != 405 {
		t.Fatalf("POST: got %d, want 405", rec.Code)
	}
}
