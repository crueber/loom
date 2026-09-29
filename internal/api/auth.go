// Auth routes + board permission scoping (OSS-50, UI-configured OIDC).
//
// Model:
//   - Auth DISABLED (default): today's open behavior, no gating at all.
//   - Auth ENABLED: boards list shows only readable boards (public for
//     anonymous, public + owned + member for sessions); private trees
//     need owner/member session (401 anonymous, 403 wrong user);
//     viewer reads, editor writes, owner (or any authenticated user on
//     legacy unowned boards) manages visibility + members.
//   - Secrets never leave the server: settings endpoints serve a
//     sanitized view, and an empty secret on PUT keeps the stored one.
package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/crueber/loom-rebuild/internal/auth"
	"github.com/crueber/loom-rebuild/internal/model"
)

// authEnabled loads settings; on store error auth stays off (fail open
// keeps today's behavior rather than locking the server out).
func (h *Handler) authSettings() model.AuthSettings {
	s, err := h.Store.GetAuthSettings()
	if err != nil {
		return model.AuthSettings{}
	}
	return s
}

// CurrentUser resolves the session cookie to a user (nil when anonymous
// or auth is off and no cookie is present).
func (h *Handler) CurrentUser(r *http.Request) *model.User {
	raw := auth.TokenFromRequest(r)
	if raw == "" {
		return nil
	}
	se, err := h.Store.GetSession(auth.HashToken(raw))
	if err != nil {
		return nil
	}
	u, err := h.Store.GetUser(se.UserID)
	if err != nil {
		return nil
	}
	return &u
}

// canRead reports whether user may see board.
func canRead(s model.AuthSettings, board model.Board, user *model.User, role string) bool {
	if !s.Enabled {
		return true
	}
	if board.EffectiveVisibility() == model.VisibilityPublic && !s.RequireAuth {
		return true
	}
	if user == nil {
		return false
	}
	if board.EffectiveVisibility() == model.VisibilityPublic {
		return true
	}
	if board.OwnerID == "" {
		return true // legacy unowned board: any session reads
	}
	return board.OwnerID == user.ID || role != ""
}

// canWrite reports whether user may mutate board content.
func canWrite(s model.AuthSettings, board model.Board, user *model.User, role string) bool {
	if !s.Enabled {
		return true
	}
	if user == nil {
		return false
	}
	if board.OwnerID == "" {
		return true // legacy unowned board: any session writes
	}
	if board.OwnerID == user.ID {
		return true
	}
	return role == model.RoleEditor
}

// canAdmin reports whether user may change visibility/members.
func canAdmin(s model.AuthSettings, board model.Board, user *model.User) bool {
	if !s.Enabled {
		return true
	}
	if user == nil {
		return false
	}
	return board.OwnerID == "" || board.OwnerID == user.ID
}

// boardRole loads the user's membership role ("" = none).
func (h *Handler) boardRole(boardID string, user *model.User) string {
	if user == nil {
		return ""
	}
	m, err := h.Store.GetBoardMember(boardID, user.ID)
	if err != nil {
		return ""
	}
	return m.Role
}

// getBoardAuth loads a board plus the caller's rights on it.
func (h *Handler) getBoardAuth(boardID string, user *model.User) (model.Board, string, model.AuthSettings, error) {
	s := h.authSettings()
	tree, err := h.Store.GetTree(boardID)
	if err != nil {
		return model.Board{}, "", s, err
	}
	return tree.Board, h.boardRole(boardID, user), s, nil
}

// requireRead enforces tree visibility: 404 unknown, 401 anonymous,
// 403 authenticated without access.
func (h *Handler) requireRead(w http.ResponseWriter, boardID string, user *model.User) (model.Board, bool) {
	board, role, s, err := h.getBoardAuth(boardID, user)
	if err != nil {
		writeErr(w, http.StatusNotFound, "board not found")
		return model.Board{}, false
	}
	if !canRead(s, board, user, role) {
		if user == nil {
			writeErr(w, http.StatusUnauthorized, "login required")
		} else {
			writeErr(w, http.StatusForbidden, "no access to this board")
		}
		return model.Board{}, false
	}
	return board, true
}

func (h *Handler) authRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/auth/status", h.authStatus)
	mux.HandleFunc("/api/auth/settings", h.authSettingsRoute)
	mux.HandleFunc("/api/me", h.me)
	mux.HandleFunc("/api/me/prefs", h.mePrefs)
	mux.HandleFunc("/api/auth/login", h.login)
	mux.HandleFunc("/api/auth/callback", h.callback)
	mux.HandleFunc("/api/auth/logout", h.logout)
}

func (h *Handler) authStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	s := h.authSettings()
	user := h.CurrentUser(r)
	out := map[string]any{
		"enabled":       s.Enabled,
		"require_auth":  s.RequireAuth,
		"authenticated": user != nil,
	}
	if user != nil {
		out["user"] = user
	}
	writeJSON(w, 200, out)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user := h.CurrentUser(r)
	if user == nil {
		writeErr(w, http.StatusUnauthorized, "login required")
		return
	}
	writeJSON(w, 200, user)
}

// mePrefs serves per-user preferences (OSS-83). Auth required: anonymous
// callers get 401 and must use the localStorage cache instead. GET returns
// stored prefs (or defaults); PUT merges a partial {theme?,language?},
// normalizes, stores and returns the stored value.
func (h *Handler) mePrefs(w http.ResponseWriter, r *http.Request) {
	user := h.CurrentUser(r)
	if user == nil {
		writeErr(w, http.StatusUnauthorized, "login required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		p, err := h.Store.GetUserPrefs(user.ID)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, p)
	case http.MethodPut:
		var body struct {
			Theme    *string `json:"theme"`
			Language *string `json:"language"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		cur, err := h.Store.GetUserPrefs(user.ID)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		if body.Theme != nil {
			cur.Theme = *body.Theme
		}
		if body.Language != nil {
			cur.Language = *body.Language
		}
		stored, err := h.Store.UpdateUserPrefs(user.ID, cur.Normalize())
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, stored)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// settingsGate allows open editing while auth is disabled (so the admin
// can enable OIDC purely via the UI) and requires a session once on.
func (h *Handler) settingsGate(w http.ResponseWriter, r *http.Request, s model.AuthSettings) *model.User {
	if !s.Enabled {
		return h.CurrentUser(r)
	}
	user := h.CurrentUser(r)
	if user == nil {
		writeErr(w, http.StatusUnauthorized, "login required")
		return nil
	}
	return user
}

func (h *Handler) authSettingsRoute(w http.ResponseWriter, r *http.Request) {
	s := h.authSettings()
	switch r.Method {
	case http.MethodGet:
		if h.settingsGate(w, r, s) == nil && s.Enabled {
			return
		}
		writeJSON(w, 200, s.Sanitized())
	case http.MethodPut:
		user := h.settingsGate(w, r, s)
		if user == nil && s.Enabled {
			return
		}
		var body struct {
			Issuer       *string `json:"issuer"`
			ClientID     *string `json:"client_id"`
			ClientSecret *string `json:"client_secret"`
			Enabled      *bool   `json:"enabled"`
			RequireAuth  *bool   `json:"require_auth"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		next := s
		if body.Issuer != nil {
			next.Issuer = strings.TrimSpace(*body.Issuer)
		}
		if body.ClientID != nil {
			next.ClientID = strings.TrimSpace(*body.ClientID)
		}
		if body.ClientSecret != nil && *body.ClientSecret != "" {
			next.ClientSecret = *body.ClientSecret
		}
		if body.Enabled != nil {
			next.Enabled = *body.Enabled
		}
		if body.RequireAuth != nil {
			next.RequireAuth = *body.RequireAuth
		}
		if next.Enabled && (next.Issuer == "" || next.ClientID == "" || next.ClientSecret == "") {
			writeErr(w, http.StatusBadRequest, "issuer, client ID and secret are required to enable auth")
			return
		}
		stored, err := h.Store.UpdateAuthSettings(next)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, stored.Sanitized())
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func callbackBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	s := h.authSettings()
	if !s.Enabled {
		writeErr(w, http.StatusBadRequest, "auth is not enabled")
		return
	}
	if s.Issuer == "" || s.ClientID == "" {
		writeErr(w, http.StatusBadRequest, "OIDC is not configured")
		return
	}
	d, err := auth.Discover(s.Issuer)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	state, err := auth.NewState()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	verifier, err := auth.NewVerifier()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	nonce, err := auth.NewState()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	redirectURI := callbackBaseURL(r) + "/api/auth/callback"
	h.pending().Create(state, verifier, nonce, redirectURI)
	http.Redirect(w, r, auth.LoginURL(d, s.ClientID, redirectURI, state, nonce, auth.ChallengeS256(verifier)), http.StatusFound)
}

func (h *Handler) callback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	s := h.authSettings()
	if !s.Enabled {
		writeErr(w, http.StatusBadRequest, "auth is not enabled")
		return
	}
	q := r.URL.Query()
	if q.Get("error") != "" {
		writeErr(w, http.StatusBadRequest, "provider: "+q.Get("error"))
		return
	}
	pend, ok := h.pending().Consume(q.Get("state"))
	if !ok {
		writeErr(w, http.StatusBadRequest, "unknown or expired login")
		return
	}
	d, err := auth.Discover(s.Issuer)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	idToken, err := auth.Exchange(d.TokenEndpoint, s.ClientID, s.ClientSecret, q.Get("code"), pend.Verifier, pend.RedirectURI)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	id, err := auth.VerifyIDToken(d.JwksURI, d.Issuer, s.ClientID, idToken, pend.Nonce)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "bad id_token: "+err.Error())
		return
	}
	user, err := h.Store.UpsertUserBySubject(s.Issuer, id.Subject, id.Email, id.Name)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	raw, hash, err := auth.MintToken()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	exp := time.Now().Add(auth.SessionTTL)
	if _, err := h.Store.CreateSession(user.ID, hash, exp); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	auth.SetSessionCookie(w, raw, exp)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if raw := auth.TokenFromRequest(r); raw != "" {
		_ = h.Store.DeleteSession(auth.HashToken(raw))
	}
	auth.ClearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// boardMembers handles /api/boards/{id}/members... It is called from
// boardSub (same mux pattern) when the sub-path starts with "members".
func (h *Handler) boardMembers(w http.ResponseWriter, r *http.Request, id, sub string) {
	user := h.CurrentUser(r)
	s := h.authSettings()
	tree, err := h.Store.GetTree(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "board not found")
		return
	}
	if !canAdmin(s, tree.Board, user) {
		if user == nil {
			writeErr(w, http.StatusUnauthorized, "login required")
		} else {
			writeErr(w, http.StatusForbidden, "only the board owner manages members")
		}
		return
	}
	rest2 := strings.TrimPrefix(sub, "members")
	if rest2 == "" || rest2 == "/" {
		switch r.Method {
		case http.MethodGet:
			members, err := h.Store.ListBoardMembers(id)
			if err != nil {
				writeErr(w, 500, err.Error())
				return
			}
			// Enrich with email/name so the share dialog can show people,
			// not just opaque user ids.
			type memberView struct {
				model.BoardMember
				Email string `json:"email,omitempty"`
				Name  string `json:"name,omitempty"`
			}
			views := make([]memberView, 0, len(members))
			for _, m := range members {
				v := memberView{BoardMember: m}
				if u, err := h.Store.GetUser(m.UserID); err == nil {
					v.Email, v.Name = u.Email, u.Name
				}
				views = append(views, v)
			}
			writeJSON(w, 200, views)
		case http.MethodPost:
			var body struct {
				UserID string `json:"user_id"`
				Email  string `json:"email"`
				Role   string `json:"role"`
			}
			if !decodeJSON(w, r, &body) {
				return
			}
			uid := strings.TrimSpace(body.UserID)
			if uid == "" && strings.TrimSpace(body.Email) != "" {
				u, err := h.Store.GetUserByEmail(strings.TrimSpace(body.Email))
				if err != nil {
					writeErr(w, http.StatusNotFound, "no such user (they must log in once first)")
					return
				}
				uid = u.ID
			}
			if uid == "" {
				writeErr(w, http.StatusBadRequest, "user_id or email is required")
				return
			}
			if _, err := h.Store.GetUser(uid); err != nil {
				writeErr(w, http.StatusNotFound, "no such user")
				return
			}
			m, err := h.Store.SetBoardMember(id, uid, body.Role)
			if err != nil {
				writeErr(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, 201, m)
		default:
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}
	uid := strings.TrimPrefix(rest2, "/")
	if strings.Contains(uid, "/") || uid == "" {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var body struct {
			Role string `json:"role"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		m, err := h.Store.SetBoardMember(id, uid, body.Role)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, 200, m)
	case http.MethodDelete:
		if err := h.Store.RemoveBoardMember(id, uid); err != nil {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// BootstrapBoard returns the first board tree visible to the requester
// (nil when none is visible); used for __BOOTSTRAP_DATA__ so private
// boards never leak into the app shell.
func (h *Handler) BootstrapBoard(r *http.Request) *model.BoardTree {
	user := h.CurrentUser(r)
	s := h.authSettings()
	boards, err := h.Store.ListBoards()
	if err != nil {
		return nil
	}
	for _, b := range boards {
		if !canRead(s, b, user, h.boardRole(b.ID, user)) {
			continue
		}
		if t, err := h.Store.GetTree(b.ID); err == nil {
			return &t
		}
	}
	return nil
}

// visibleBoards filters ListBoards to what the requester may see.
func (h *Handler) visibleBoards(user *model.User) ([]model.Board, error) {
	s := h.authSettings()
	boards, err := h.Store.ListBoards()
	if err != nil {
		return nil, err
	}
	out := []model.Board{}
	for _, b := range boards {
		if canRead(s, b, user, h.boardRole(b.ID, user)) {
			out = append(out, b)
		}
	}
	return out, nil
}
