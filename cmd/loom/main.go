package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/crueber/loom-rebuild/internal/api"
	"github.com/crueber/loom-rebuild/internal/model"
	"github.com/crueber/loom-rebuild/internal/store"
)

// Single binary: JSON API + static web shell with server bootstrap.
// Instant-load strategy (hybrid A+B):
//   - service worker caches the app shell (see web/sw.js)
//   - first paint renders cache-first from localStorage
//   - __BOOTSTRAP_DATA__ inlines the current board tree as fallback/
//     revalidation seed, so a cold tab still paints without a spinner.
//
// assetVersion stamps shell asset URLs so deploys self-invalidate every
// cache (service worker + HTTP): the hash changes exactly when the bytes
// do, so a fresh deploy can never keep serving stale JS/CSS. Missing
// assets yield "" (serve unversioned) so the server always starts.
func assetVersion(webDir string) string {
	h := sha256.New()
	for _, name := range []string{"app.js", "styles.css"} {
		b, err := os.ReadFile(filepath.Join(webDir, name))
		if err != nil {
			return ""
		}
		if _, err := h.Write(b); err != nil {
			return ""
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// versionedShell rewrites the shell asset refs with the deploy hash.
func versionedShell(tmpl, ver string) string {
	if ver == "" {
		return tmpl
	}
	t := strings.Replace(tmpl, `src="/app.js"`, `src="/app.js?v=`+ver+`"`, 1)
	return strings.Replace(t, `href="/styles.css"`, `href="/styles.css?v=`+ver+`"`, 1)
}

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	data := flag.String("data", "loom-data.json", "data file path (.db/.sqlite/.sqlite3 = SQLite, else JSON file, empty = in-memory)")
	webDir := flag.String("web", "web", "web assets directory")
	flag.Parse()

	// Store selection: SQLite for *.db/*.sqlite/*.sqlite3 (plan default, single
	// binary via cgo), JSON file backend otherwise.
	var st store.Store
	var closer func() error
	if strings.HasSuffix(*data, ".db") || strings.HasSuffix(*data, ".sqlite") || strings.HasSuffix(*data, ".sqlite3") {
		sq, err := store.OpenSQLite(*data)
		if err != nil {
			log.Fatalf("open sqlite: %v", err)
		}
		closer = sq.Close
		st = sq
	} else {
		fs, err := store.OpenFile(*data)
		if err != nil {
			log.Fatalf("open store: %v", err)
		}
		st, closer = fs, func() error { return nil }
	}
	defer closer()

	// Seed a starter board on first run so a new tab is never empty.
	boards, err := st.ListBoards()
	if err != nil {
		log.Fatalf("list boards: %v", err)
	}
	if len(boards) == 0 {
		b, _ := st.CreateBoard("Home")
		col, _ := st.CreateColumn(b.ID, "Start here", "#4c8dff")
		_, _ = st.CreateCard(col.ID, []model.Block{
			model.NewNoteBlock("Welcome to Loom. **Cards** hold links, notes and images. Edit inline — click any text."),
			model.NewLinkBlock("https://columns.app/", "Columns.app — design reference"),
		})
		boards, _ = st.ListBoards()
	}

	h := &api.Handler{Store: st}
	mux := http.NewServeMux()
	h.Register(mux)

	indexTmpl, err := os.ReadFile(filepath.Join(*webDir, "index.html"))
	if err != nil {
		log.Fatalf("read index.html: %v", err)
	}
	// Deploy-pinned shell: versioned asset URLs change with the bytes,
	// so browsers/SW caches can never stick on stale JS/CSS.
	indexBase := versionedShell(string(indexTmpl), assetVersion(*webDir))

	// App shell with bootstrap: inline the first board tree visible to
	// the requester (nil for anonymous users when nothing is public,
	// so private boards never leak into the app shell).
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			// Static assets, contained in webDir (no ../ escapes).
			p := filepath.Join(*webDir, filepath.Clean(strings.TrimPrefix(r.URL.Path, "/")))
			if rel, err := filepath.Rel(*webDir, p); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				http.NotFound(w, r)
				return
			}
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				// Deploy-pinned URLs are immutable; unversioned refs
				// must revalidate so a refresh always finds the deploy.
				if r.URL.Query().Get("v") != "" {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				} else {
					w.Header().Set("Cache-Control", "no-cache")
				}
				http.ServeFile(w, r, p)
				return
			}
			http.NotFound(w, r)
			return
		}
		tree := map[string]any{"board": nil}
		if t := h.BootstrapBoard(r); t != nil {
			tree["board"] = *t
		}
		boot, _ := json.Marshal(tree["board"])
		page := strings.Replace(indexBase, "/*__BOOTSTRAP_DATA__*/null", string(boot), 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	})

	log.Printf("loom-rebuild listening on %s (web=%s data=%s)", *addr, *webDir, *data)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
