// Server-generated user avatars: GET /avatar/{seed}.svg serves the
// official DiceBear Landscape SVG for the seed, rendered in-process by
// dicebear-go (byte-identical to every other DiceBear library — the same
// seed yields the same avatar as the old HTTP API did). No external
// service, no storage. The seed is only passed as an option string, never
// reflected or touched as a path, so any non-empty string (email, sub,
// name fallback) is safe.
package api

import (
	"net/http"
	"strings"
	"sync"

	dicebear "github.com/dicebear/dicebear-go/v10"
	"github.com/dicebear/styles/v10"
)

// avatarSeedMax bounds the option input; seeds are small opaque strings.
const avatarSeedMax = 256

var landscapeOnce sync.Once
var landscapeStyle *dicebear.Style
var landscapeErr error

// getLandscape parses the Landscape style definition once and reuses it
// for every avatar (Style is immutable after NewStyle).
func getLandscape() (*dicebear.Style, error) {
	landscapeOnce.Do(func() {
		landscapeStyle, landscapeErr = dicebear.NewStyle([]byte(styles.Landscape))
	})
	return landscapeStyle, landscapeErr
}

// avatarArt renders the deterministic Landscape SVG for a seed.
func avatarArt(seed string) ([]byte, error) {
	style, err := getLandscape()
	if err != nil {
		return nil, err
	}
	av, err := dicebear.NewAvatar(style, map[string]any{"seed": seed})
	if err != nil {
		return nil, err
	}
	return []byte(av.SVG()), nil
}

// serveAvatar serves /avatar/{seed}.svg with a generated landscape.
func (h *Handler) serveAvatar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/avatar/")
	if !strings.HasSuffix(rest, ".svg") || strings.Contains(rest, "/") {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	seed := strings.TrimSuffix(rest, ".svg")
	if seed == "" || len(seed) > avatarSeedMax {
		writeErr(w, http.StatusBadRequest, "invalid seed")
		return
	}
	art, err := avatarArt(seed)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "avatar unavailable")
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Content-Length", itoa(len(art)))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(art)
}
