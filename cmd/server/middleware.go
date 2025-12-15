package main

import (
	"net/http"
	"strings"
)

// cacheControlMiddleware adds appropriate cache headers for static assets
func cacheControlMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		hasVersion := r.URL.Query().Get("v") != ""

		// Cache versioned assets for 1 year (immutable)
		if strings.HasPrefix(path, "/static/") && hasVersion {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else if strings.HasPrefix(path, "/static/lib/") {
			// Cache third-party libs for 1 year (already versioned by library authors)
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else if strings.HasPrefix(path, "/static/") {
			// Cache unversioned assets for 5 minutes
			w.Header().Set("Cache-Control", "public, max-age=300")
		}

		next.ServeHTTP(w, r)
	})
}
