// Server-cached link icons (OSS-82 child scope).
// GET /icons/{host}.ico serves DuckDuckGo ip3 icons cached 7 days.
package api

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/crueber/loom-rebuild/internal/store"
)

// IconBackend is implemented by stores that can keep cached icons
// (SQLite and file backends both implement it).
type IconBackend interface {
	GetIcon(host string) (store.IconRecord, error)
	SaveIcon(host, contentType string, blob []byte) (store.IconRecord, error)
}

// FetchIcon fetches raw icon bytes for host from the upstream provider.
// Injectable so tests never hit the network. Default builds only the
// DuckDuckGo ip3 URL from the validated host — never the user's raw
// link URL (SSRF guard).
var FetchIcon = func(host string) (string, []byte, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	url := "https://icons.duckduckgo.com/ip3/" + host + ".ico"
	resp, err := client.Get(url)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("upstream %d", resp.StatusCode)
	}
	ctype := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ctype, "image/") {
		return "", nil, fmt.Errorf("upstream content-type %q", ctype)
	}
	blob, err := io.ReadAll(io.LimitReader(resp.Body, store.IconMaxBytes+1))
	if err != nil {
		return "", nil, err
	}
	if len(blob) > store.IconMaxBytes {
		return "", nil, fmt.Errorf("icon too large")
	}
	return ctype, blob, nil
}

var validHostRe = regexp.MustCompile(`^[a-z0-9.-]+$`)

// normalizeIconHost lowercases, strips a port, and validates.
func normalizeIconHost(raw string) (string, bool) {
	h := strings.ToLower(strings.TrimSpace(raw))
	if h == "" {
		return "", false
	}
	// Reject scheme/slash/anything with URL or path structure.
	if strings.ContainsAny(h, "/:?#@") {
		// Allow a single trailing :port — strip it, reject the rest.
		if strings.Count(h, ":") == 1 && !strings.ContainsAny(h, "/?#@") {
			if idx := strings.LastIndex(h, ":"); idx >= 0 {
				h = h[:idx]
			}
		} else {
			return "", false
		}
	}
	if h == "" || len(h) > 253 {
		return "", false
	}
	if !validHostRe.MatchString(h) {
		return "", false
	}
	if h != "localhost" && !strings.Contains(h, ".") {
		return "", false
	}
	return h, true
}

// serveIcon serves /icons/{host}.ico with 7-day server cache.
func (h *Handler) serveIcon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	backend, ok := h.Store.(IconBackend)
	if !ok {
		writeErr(w, http.StatusNotImplemented, "icon cache unavailable")
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/icons/")
	if !strings.HasSuffix(rest, ".ico") || strings.Contains(rest, "/") {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	raw := strings.TrimSuffix(rest, ".ico")
	host, valid := normalizeIconHost(raw)
	if !valid {
		writeErr(w, http.StatusBadRequest, "invalid host")
		return
	}
	maxAge := int(store.IconTTL.Seconds())
	cached, cerr := backend.GetIcon(host)
	if cerr == nil && time.Since(cached.FetchedAt) < store.IconTTL {
		writeIcon(w, cached.ContentType, cached.Blob, maxAge)
		return
	}
	ctype, blob, ferr := FetchIcon(host)
	if ferr != nil {
		// Upstream failure: serve stale if present, else 404
		// (frontend letter-tile fallback covers it).
		if cerr == nil && len(cached.Blob) > 0 {
			writeIcon(w, cached.ContentType, cached.Blob, maxAge)
			return
		}
		writeErr(w, http.StatusNotFound, "icon not found")
		return
	}
	rec, serr := backend.SaveIcon(host, ctype, blob)
	if serr != nil {
		// Store failure: still serve what we fetched.
		writeIcon(w, ctype, blob, maxAge)
		return
	}
	writeIcon(w, rec.ContentType, rec.Blob, maxAge)
}

func writeIcon(w http.ResponseWriter, ctype string, blob []byte, maxAge int) {
	if ctype == "" {
		ctype = "image/x-icon"
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Length", itoa(len(blob)))
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", maxAge))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(blob)
}
