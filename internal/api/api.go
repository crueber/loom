// Package api implements the JSON API served by the single binary.
// Routes (all under /api):
//
//	GET    /api/boards            list boards
//	POST   /api/boards            {title,background?}
//	GET    /api/boards/{id}       full tree (board + columns + cards)
//	PATCH  /api/boards/{id}       {title,background}
//	DELETE /api/boards/{id}
//	POST   /api/boards/{id}/columns        {title,color}
//	PATCH  /api/columns/{id}               {title,color,position,collapsed}
//	DELETE /api/columns/{id}
//	POST   /api/columns/{id}/cards         {blocks}
//	PATCH  /api/cards/{id}                 full card (blocks edited inline)
//	DELETE /api/cards/{id}
//	POST   /api/cards/{id}/move            {column_id,position}
//	POST   /api/import/v1                  v1 export JSON -> board tree
//	GET    /api/export                     v2 export (boards + trees)
package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/crueber/loom-rebuild/internal/auth"
	"github.com/crueber/loom-rebuild/internal/images"
	"github.com/crueber/loom-rebuild/internal/model"
	"github.com/crueber/loom-rebuild/internal/store"
)

// ImageBackend is implemented by stores that can keep uploads
// (currently SQLite; the file backend returns 501).
type ImageBackend interface {
	SaveImage(contentType string, w, h, tw, th int, blob, thumb []byte) (store.ImageRecord, error)
	GetImage(id string) (store.ImageRecord, error)
}

// Handler wires the store to HTTP routes on a stdlib mux.
type Handler struct {
	Store store.Store

	mu     sync.Mutex
	logins *auth.Logins
}

// pending returns the in-flight OIDC login tracker (lazy, mutex-guarded
// so handlers built as &Handler{Store: st} in tests work unchanged).
func (h *Handler) pending() *auth.Logins {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.logins == nil {
		h.logins = auth.NewLogins()
	}
	return h.logins
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

// Register mounts all API routes onto mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/boards", h.boards)
	mux.HandleFunc("/api/boards/", h.boardSub)
	mux.HandleFunc("/api/columns/", h.columnSub)
	mux.HandleFunc("/api/cards/", h.cardSub)
	mux.HandleFunc("/api/import/v1", h.importV1)
	mux.HandleFunc("/api/export", h.export)
	mux.HandleFunc("/api/images", h.uploadImage)
	mux.HandleFunc("/images/", h.serveImage)
	h.authRoutes(mux)
}

// requireWriteByBoard enforces content-write rights on a board:
// 404 unknown, 401 anonymous, 403 read-only (viewer / outsider).
func (h *Handler) requireWriteByBoard(w http.ResponseWriter, boardID string, user *model.User) (model.Board, bool) {
	board, role, s, err := h.getBoardAuth(boardID, user)
	if err != nil {
		writeErr(w, http.StatusNotFound, "board not found")
		return model.Board{}, false
	}
	if !canWrite(s, board, user, role) {
		if user == nil {
			writeErr(w, http.StatusUnauthorized, "login required")
		} else {
			writeErr(w, http.StatusForbidden, "read-only access")
		}
		return model.Board{}, false
	}
	return board, true
}

func (h *Handler) boards(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		boards, err := h.visibleBoards(h.CurrentUser(r))
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		if boards == nil {
			boards = []model.Board{}
		}
		writeJSON(w, 200, boards)
	case http.MethodPost:
		var body struct {
			Title      string `json:"title"`
			Background string `json:"background"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		if strings.TrimSpace(body.Title) == "" {
			writeErr(w, http.StatusBadRequest, "title is required")
			return
		}
		// New boards are private + owned when auth is on (anonymous
		// creation would leave an ownerless private board).
		var b model.Board
		var err error
		if s := h.authSettings(); s.Enabled {
			user := h.CurrentUser(r)
			if user == nil {
				writeErr(w, http.StatusUnauthorized, "login required")
				return
			}
			b, err = h.Store.CreateBoardWithOwner(body.Title, user.ID, model.VisibilityPrivate)
		} else {
			b, err = h.Store.CreateBoard(body.Title)
		}
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		// OSS-70: the client (OSS-58) already POSTs the current
		// background so new boards inherit it; persist it here.
		// Store interface stays stable: create then UpdateBoard.
		// Unknown ids fall back to '' (client normBg renders paper).
		if bg := normalizeBackground(body.Background); bg != "" {
			if updated, uerr := h.Store.UpdateBoard(b.ID, b.Title, bg); uerr == nil {
				b = updated
			}
		}
		writeJSON(w, 201, b)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// boardSub handles /api/boards/{id}, /api/boards/{id}/columns and
// /api/boards/{id}/members...
func (h *Handler) boardSub(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/boards/")
	id, sub, _ := strings.Cut(rest, "/")
	if id == "" {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if sub == "members" || strings.HasPrefix(sub, "members/") {
		h.boardMembers(w, r, id, sub)
		return
	}
	if sub == "columns" {
		if r.Method != http.MethodPost {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if _, ok := h.requireWriteByBoard(w, id, h.CurrentUser(r)); !ok {
			return
		}
		var body struct {
			Title string `json:"title"`
			Color string `json:"color"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		col, err := h.Store.CreateColumn(id, body.Title, body.Color)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		writeJSON(w, 201, col)
		return
	}
	if sub != "" {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	user := h.CurrentUser(r)
	switch r.Method {
	case http.MethodGet:
		if _, ok := h.requireRead(w, id, user); !ok {
			return
		}
		tree, err := h.Store.GetTree(id)
		if err != nil {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, 200, tree)
	case http.MethodPatch:
		var body struct {
			Title      *string `json:"title"`
			Background *string `json:"background"`
			Visibility *string `json:"visibility"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		if body.Title == nil && body.Background == nil && body.Visibility == nil {
			writeErr(w, http.StatusBadRequest, "nothing to update")
			return
		}
		// Merge with stored values so partial patches never wipe fields.
		board, _, s, err := h.getBoardAuth(id, user)
		if err != nil {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		if body.Visibility != nil {
			if !canAdmin(s, board, user) {
				if user == nil {
					writeErr(w, http.StatusUnauthorized, "login required")
				} else {
					writeErr(w, http.StatusForbidden, "only the board owner changes visibility")
				}
				return
			}
			vis := strings.TrimSpace(*body.Visibility)
			updated, err := h.Store.SetBoardVisibility(id, vis)
			if err != nil {
				writeErr(w, http.StatusBadRequest, err.Error())
				return
			}
			board = updated
			// First authenticated setter on a legacy unowned board
			// becomes its owner, so the board stays manageable.
			if board.OwnerID == "" && user != nil && s.Enabled {
				if owned, err := h.Store.SetBoardOwner(id, user.ID); err == nil {
					board = owned
				}
			}
		}
		if body.Title != nil || body.Background != nil {
			if _, ok := h.requireWriteByBoard(w, id, user); !ok {
				return
			}
			cur, err := h.Store.GetTree(id)
			if err != nil {
				writeErr(w, http.StatusNotFound, err.Error())
				return
			}
			title, bg := cur.Board.Title, cur.Board.Background
			if body.Title != nil {
				title = strings.TrimSpace(*body.Title)
				if title == "" {
					writeErr(w, http.StatusBadRequest, "title must not be empty")
					return
				}
			}
			if body.Background != nil {
				bg = *body.Background
			}
			b, err := h.Store.UpdateBoard(id, title, bg)
			if err != nil {
				writeErr(w, http.StatusNotFound, err.Error())
				return
			}
			// Preserve the visibility/owner changes above in one reply.
			b.Visibility, b.OwnerID = board.Visibility, board.OwnerID
			writeJSON(w, 200, b)
			return
		}
		writeJSON(w, 200, board)
	case http.MethodDelete:
		board, _, s, err := h.getBoardAuth(id, user)
		if err != nil {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		if !canAdmin(s, board, user) {
			if user == nil {
				writeErr(w, http.StatusUnauthorized, "login required")
			} else {
				writeErr(w, http.StatusForbidden, "only the board owner deletes it")
			}
			return
		}
		if err := h.Store.DeleteBoard(id); err != nil {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// columnSub handles /api/columns/{id} and /api/columns/{id}/cards.
func (h *Handler) columnSub(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/columns/")
	id, sub, _ := strings.Cut(rest, "/")
	if id == "" {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if sub == "cards" {
		if r.Method != http.MethodPost {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		boardID, err := h.Store.BoardIDForColumn(id)
		if err != nil {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		if _, ok := h.requireWriteByBoard(w, boardID, h.CurrentUser(r)); !ok {
			return
		}
		var body struct {
			Blocks []model.Block `json:"blocks"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		card, err := h.Store.CreateCard(id, body.Blocks)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		writeJSON(w, 201, card)
		return
	}
	if sub != "" {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	boardID, err := h.Store.BoardIDForColumn(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	if _, ok := h.requireWriteByBoard(w, boardID, h.CurrentUser(r)); !ok {
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var col model.Column
		if !decodeJSON(w, r, &col) {
			return
		}
		col.ID = id
		updated, err := h.Store.UpdateColumn(col)
		if err != nil {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, 200, updated)
	case http.MethodDelete:
		if err := h.Store.DeleteColumn(id); err != nil {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// cardSub handles /api/cards/{id} and /api/cards/{id}/move.
func (h *Handler) cardSub(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/cards/")
	id, sub, _ := strings.Cut(rest, "/")
	if id == "" {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	boardID, err := h.Store.BoardIDForCard(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	if _, ok := h.requireWriteByBoard(w, boardID, h.CurrentUser(r)); !ok {
		return
	}
	if sub == "move" {
		if r.Method != http.MethodPost {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var body struct {
			ColumnID string `json:"column_id"`
			Position int    `json:"position"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		card, err := h.Store.MoveCard(id, body.ColumnID, body.Position)
		if err != nil {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, 200, card)
		return
	}
	if sub != "" {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var card model.Card
		if !decodeJSON(w, r, &card) {
			return
		}
		card.ID = id
		updated, err := h.Store.UpdateCard(card)
		if err != nil {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, 200, updated)
	case http.MethodDelete:
		if err := h.Store.DeleteCard(id); err != nil {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) importV1(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	defer r.Body.Close()
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	tree, err := model.ImportV1(raw)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	// Imports land private + owned when auth is on, like new boards.
	if s := h.authSettings(); s.Enabled {
		user := h.CurrentUser(r)
		if user == nil {
			writeErr(w, http.StatusUnauthorized, "login required")
			return
		}
		tree.Board.OwnerID = user.ID
		tree.Board.Visibility = model.VisibilityPrivate
	}
	stored, err := h.Store.ImportTree(tree)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, stored)
}

func (h *Handler) export(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	boards, err := h.visibleBoards(h.CurrentUser(r))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	type v2 struct {
		Version int               `json:"version"`
		Boards  []model.BoardTree `json:"boards"`
	}
	out := v2{Version: 2, Boards: []model.BoardTree{}}
	for _, b := range boards {
		tree, err := h.Store.GetTree(b.ID)
		if err != nil {
			continue
		}
		out.Boards = append(out.Boards, tree)
	}
	writeJSON(w, 200, out)
}

// uploadImage accepts one multipart file field ("file", jpeg/png/gif,
// <=12MB), stores original + JPEG thumbnail, and returns the image
// block payload to attach to a card.
func (h *Handler) uploadImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	// Uploads attach to cards later, so they need a session when auth
	// is on; serving stays open (image ids are unguessable).
	if s := h.authSettings(); s.Enabled && h.CurrentUser(r) == nil {
		writeErr(w, http.StatusUnauthorized, "login required")
		return
	}
	backend, ok := h.Store.(ImageBackend)
	if !ok {
		writeErr(w, http.StatusNotImplemented, "image uploads require the SQLite backend")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, images.MaxUploadBytes+1024)
	if err := r.ParseMultipartForm(images.MaxUploadBytes); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid multipart upload")
		return
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "cannot read upload")
		return
	}
	proc, err := images.Process(raw, hdr.Header.Get("Content-Type"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	rec, err := backend.SaveImage(proc.ContentType, proc.Width, proc.Height, proc.ThumbWidth, proc.ThumbHeight, proc.Blob, proc.Thumb)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{
		"id":        rec.ID,
		"url":       "/images/" + rec.ID,
		"thumb_url": "/images/" + rec.ID + "/thumb",
		"width":     rec.Width, "height": rec.Height,
	})
}

// serveImage serves /images/{id} (original) and /images/{id}/thumb
// (JPEG thumbnail) with immutable long-cache headers. Thumbnails keep
// card grids fast; full originals load only on click-through.
func (h *Handler) serveImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	backend, ok := h.Store.(ImageBackend)
	if !ok {
		writeErr(w, http.StatusNotImplemented, "image serving requires the SQLite backend")
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/images/")
	id, sub, _ := strings.Cut(rest, "/")
	if id == "" || (sub != "" && sub != "thumb") {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	rec, err := backend.GetImage(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "image not found")
		return
	}
	blob, ctype := rec.Blob, rec.ContentType
	if sub == "thumb" {
		blob, ctype = rec.ThumbBlob, "image/jpeg"
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Length", itoa(len(blob)))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.WriteHeader(200)
	_, _ = w.Write(blob)
}

// normalizeBackground validates a board background id against the
// client SWATCHES ids (web/app.js). ” and 'paper' both render as
// paper, so they normalize to ” (no persist needed); unknown ids
// also fall back to ” and are never stored.
func normalizeBackground(bg string) string {
	switch strings.TrimSpace(bg) {
	case "honey", "sage", "sky", "rose", "slate":
		return strings.TrimSpace(bg)
	default:
		return ""
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
