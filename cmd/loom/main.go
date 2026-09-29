package main

import (
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
		page := strings.Replace(string(indexTmpl), "/*__BOOTSTRAP_DATA__*/null", string(boot), 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	})

	log.Printf("loom-rebuild listening on %s (web=%s data=%s)", *addr, *webDir, *data)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
