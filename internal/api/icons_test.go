package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/crueber/loom-rebuild/internal/store"
)

func newIconMux(t *testing.T, st store.Store) *http.ServeMux {
	t.Helper()
	h := &Handler{Store: st}
	mux := http.NewServeMux()
	h.Register(mux)
	return mux
}

func stubFetch(t *testing.T, calls *int, ctype string, blob []byte, err error) {
	t.Helper()
	orig := FetchIcon
	*calls = 0
	FetchIcon = func(host string) (string, []byte, error) {
		*calls++
		return ctype, blob, err
	}
	t.Cleanup(func() { FetchIcon = orig })
}

func TestIconMissCachesAndHitSkipsFetch(t *testing.T) {
	st, err := store.OpenFile("")
	if err != nil {
		t.Fatal(err)
	}
	mux := newIconMux(t, st)
	png := []byte{0x89, 'P', 'N', 'G', 1, 2, 3}
	var calls int
	stubFetch(t, &calls, "image/png", png, nil)

	// Miss: fetches upstream, serves bytes with 7-day cache headers.
	req := httptest.NewRequest("GET", "/icons/example.com.ico", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("miss: %d %s", rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=604800" {
		t.Fatalf("Cache-Control = %q", cc)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("Content-Type = %q", ct)
	}
	if rec.Body.String() != string(png) {
		t.Fatal("miss body mismatch")
	}
	before, _ := st.GetIcon("example.com")

	// Hit: no second upstream fetch, fetched_at unchanged.
	req2 := httptest.NewRequest("GET", "/icons/example.com.ico", nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != 200 || rec2.Body.String() != string(png) {
		t.Fatalf("hit: %d %q", rec2.Code, rec2.Body.String())
	}
	if calls != 1 {
		t.Fatalf("fetch calls = %d, want 1", calls)
	}
	after, _ := st.GetIcon("example.com")
	if !after.FetchedAt.Equal(before.FetchedAt) {
		t.Fatal("fetched_at changed on cache hit")
	}
}

func TestIconStaleFailureServesStale(t *testing.T) {
	st, err := store.OpenSQLite(filepath.Join(t.TempDir(), "icons.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	mux := newIconMux(t, st)
	old := []byte{1, 2, 3, 4}
	if _, err := st.SaveIcon("stale.com", "image/x-icon", old); err != nil {
		t.Fatal(err)
	}
	if err := st.SetIconFetchedAt("stale.com", time.Now().UTC().Add(-8*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	var calls int
	stubFetch(t, &calls, "", nil, errors.New("upstream down"))

	req := httptest.NewRequest("GET", "/icons/stale.com.ico", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("stale fallback: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != string(old) {
		t.Fatal("stale body mismatch")
	}
	if calls != 1 {
		t.Fatalf("stale should trigger one refetch, got %d", calls)
	}
}

func TestIconStaleSuccessRefreshes(t *testing.T) {
	st, err := store.OpenFile("")
	if err != nil {
		t.Fatal(err)
	}
	mux := newIconMux(t, st)
	old := []byte{9, 9, 9}
	fresh := []byte{7, 7, 7, 7}
	if _, err := st.SaveIcon("refresh.com", "image/x-icon", old); err != nil {
		t.Fatal(err)
	}
	if err := st.SetIconFetchedAt("refresh.com", time.Now().UTC().Add(-8*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	var calls int
	stubFetch(t, &calls, "image/png", fresh, nil)

	req := httptest.NewRequest("GET", "/icons/refresh.com.ico", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != string(fresh) {
		t.Fatalf("refresh: %d %q", rec.Code, rec.Body.String())
	}
	if calls != 1 {
		t.Fatalf("fetch calls = %d, want 1", calls)
	}
}

func TestIconInvalidHostsNoFetch(t *testing.T) {
	st, err := store.OpenFile("")
	if err != nil {
		t.Fatal(err)
	}
	mux := newIconMux(t, st)
	var calls int
	stubFetch(t, &calls, "image/png", []byte{1}, nil)

	for _, path := range []string{
		"/icons/.ico",
		"/icons/http:example.com.ico",
		"/icons/example.com/foo.ico",
		"/icons/bad_host!.ico",
		"/icons/nodot.ico",
		"/icons/example.com.png",
	} {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != 400 && rec.Code != 404 {
			t.Fatalf("%s: got %d, want 400/404", path, rec.Code)
		}
	}
	if calls != 0 {
		t.Fatalf("invalid hosts triggered %d fetches", calls)
	}
}

func TestIconMissUpstreamFailureIs404(t *testing.T) {
	st, err := store.OpenFile("")
	if err != nil {
		t.Fatal(err)
	}
	mux := newIconMux(t, st)
	var calls int
	stubFetch(t, &calls, "", nil, errors.New("down"))
	req := httptest.NewRequest("GET", "/icons/missing.com.ico", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatalf("got %d, want 404", rec.Code)
	}
}

func TestNormalizeIconHost(t *testing.T) {
	if h, ok := normalizeIconHost("Example.COM:8080"); !ok || h != "example.com" {
		t.Fatalf("port strip = %q,%v", h, ok)
	}
	if _, ok := normalizeIconHost("localhost"); !ok {
		t.Fatal("localhost should be allowed")
	}
	for _, bad := range []string{"", "http://x.com", "a/b", "x_y.com", "nodot", strings.Repeat("a", 254) + ".com"} {
		if _, ok := normalizeIconHost(bad); ok {
			t.Fatalf("%q should be invalid", bad)
		}
	}
}
