package api

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/crueber/loom/internal/store"
)

func withLinkTitle(t *testing.T, fn func(string) (string, error)) {
	t.Helper()
	old := FetchLinkTitle
	FetchLinkTitle = fn
	t.Cleanup(func() { FetchLinkTitle = old })
}

func linkMetaRequest(t *testing.T, h *Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/linkmeta?url="+url.QueryEscape(target), nil)
	h.linkMeta(w, r)
	return w
}

func newLinkMetaHandler(t *testing.T) *Handler {
	t.Helper()
	st, err := store.OpenFile("")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return &Handler{Store: st}
}

func TestLinkMetaReturnsTitle(t *testing.T) {
	var got string
	withLinkTitle(t, func(raw string) (string, error) {
		got = raw
		return "Example Domain", nil
	})
	h := newLinkMetaHandler(t)
	w := linkMetaRequest(t, h, "https://example.com/page")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if strings.TrimSpace(w.Body.String()) != `{"title":"Example Domain"}` {
		t.Fatalf("body = %q", w.Body.String())
	}
	if got != "https://example.com/page" {
		t.Fatalf("fetched %q", got)
	}
}

func TestLinkMetaSoftFailsOnFetchError(t *testing.T) {
	withLinkTitle(t, func(string) (string, error) { return "", errors.New("boom") })
	w := linkMetaRequest(t, newLinkMetaHandler(t), "https://example.com/")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if strings.TrimSpace(w.Body.String()) != `{"title":""}` {
		t.Fatalf("body = %q", w.Body.String())
	}
}

func TestLinkMetaRejectsBadURLs(t *testing.T) {
	fetched := 0
	withLinkTitle(t, func(string) (string, error) { fetched++; return "x", nil })
	h := newLinkMetaHandler(t)
	bad := []string{
		"",
		"ftp://example.com/file",
		"file:///etc/passwd",
		"javascript:alert(1)",
		"http://localhost/",
		"http://app.localhost/",
		"http://printer.local/",
		"http://127.0.0.1:8080/",
		"http://10.0.0.5/",
		"http://169.254.169.254/latest/meta-data/",
		"http://192.168.1.1/",
		"http://[::1]/",
		"http://[fd00::1]/",
		"http://user:pass@example.com/",
		"https://" + strings.Repeat("a", 2048) + ".com/",
	}
	for _, raw := range bad {
		if w := linkMetaRequest(t, h, raw); w.Code != http.StatusBadRequest {
			t.Errorf("%q: status = %d, want 400", raw, w.Code)
		}
	}
	if fetched != 0 {
		t.Fatalf("fetched %d times, want 0", fetched)
	}
}

func TestLinkMetaMethodAndAuth(t *testing.T) {
	h := newLinkMetaHandler(t)
	w := httptest.NewRecorder()
	h.linkMeta(w, httptest.NewRequest("POST", "/api/linkmeta?url=https://example.com", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", w.Code)
	}
}

func TestBlockedIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.0.1", "169.254.169.254",
		"100.64.0.1", "0.0.0.0", "224.0.0.1", "::1", "fe80::1", "fd00::1", "ff02::1",
	}
	for _, raw := range blocked {
		if ip := net.ParseIP(raw); !blockedIP(ip) {
			t.Errorf("%s should be blocked", raw)
		}
	}
	allowed := []string{"93.184.216.34", "8.8.8.8", "2606:4700:4700::1111"}
	for _, raw := range allowed {
		if ip := net.ParseIP(raw); blockedIP(ip) {
			t.Errorf("%s should be allowed", raw)
		}
	}
}

func TestParseHTMLTitle(t *testing.T) {
	cases := []struct {
		doc  string
		want string
	}{
		{`<html><head><title>Example Domain</title></head></html>`, "Example Domain"},
		{"<TITLE>  Spaced\n\tOut  </TITLE>", "Spaced Out"},
		{`<title>Tom &amp; Jerry &#8212; Cartoons</title>`, "Tom & Jerry — Cartoons"},
		{`<title></title>`, ""},
		{`<html><head></head><body>no title</body></html>`, ""},
		{`<meta property="og:title" content="OG Fallback">`, "OG Fallback"},
		{`<title>Use &lt;this&gt; one</title><meta property="og:title" content="no">`, "Use <this> one"},
		{`<title><b>Nested</b> tags</title>`, "Nested tags"},
		{`<title>` + strings.Repeat("x", 300) + `</title>`, strings.Repeat("x", maxLinkTitleLen)},
	}
	for i, c := range cases {
		got := parseHTMLTitle(c.doc)
		if got != c.want {
			t.Errorf("case %d: got %q, want %q", i, got, c.want)
		}
	}
}

func TestFetchLinkTitleParsesHTML(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>From &amp; Page</title></head></html>`))
	}))
	defer srv.Close()
	title, err := fetchLinkTitleWith(srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if title != "From & Page" {
		t.Fatalf("title = %q", title)
	}
}

func TestFetchLinkTitleRejectsNonHTML(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.7"))
	}))
	defer srv.Close()
	if _, err := fetchLinkTitleWith(srv.Client(), srv.URL); err == nil {
		t.Fatal("non-HTML response should fail")
	}
}

func TestFetchLinkTitleRejectsPrivate(t *testing.T) {
	if _, err := fetchLinkTitle("http://127.0.0.1/"); err == nil {
		t.Fatal("loopback fetch should fail")
	}
	if _, err := fetchLinkTitle("http://169.254.169.254/latest/meta-data/"); err == nil {
		t.Fatal("link-local fetch should fail")
	}
}
