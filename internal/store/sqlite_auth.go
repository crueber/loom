// Auth + permission methods for SQLiteStore (OSS-50, UI-configured OIDC).
package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/crueber/loom-rebuild/internal/model"
)

func scanUser(row *sql.Row) (model.User, error) {
	var u model.User
	var ca string
	var admin int
	err := row.Scan(&u.ID, &u.Issuer, &u.Subject, &u.Email, &u.Name, &admin, &ca)
	if err == sql.ErrNoRows {
		return u, fmt.Errorf("user not found")
	}
	u.IsAdmin = admin != 0
	u.CreatedAt = parseTS(ca)
	return u, err
}

func (s *SQLiteStore) GetAuthSettings() (model.AuthSettings, error) {
	var a model.AuthSettings
	var enabled, requireAuth, backfillDone int
	err := s.db.QueryRow(`SELECT COALESCE(issuer,''),COALESCE(client_id,''),COALESCE(client_secret,''),enabled,require_auth,COALESCE(oidc_backfill_done,0) FROM auth_settings WHERE id=1`).
		Scan(&a.Issuer, &a.ClientID, &a.ClientSecret, &enabled, &requireAuth, &backfillDone)
	if err == sql.ErrNoRows {
		return model.AuthSettings{}, nil
	}
	a.Enabled, a.RequireAuth, a.OIDCBackfillDone = enabled != 0, requireAuth != 0, backfillDone != 0
	return a, err
}

func (s *SQLiteStore) UpdateAuthSettings(a model.AuthSettings) (model.AuthSettings, error) {
	_, err := s.db.Exec(`INSERT INTO auth_settings(id,issuer,client_id,client_secret,enabled,require_auth,oidc_backfill_done) VALUES(1,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET issuer=excluded.issuer,client_id=excluded.client_id,client_secret=excluded.client_secret,enabled=excluded.enabled,require_auth=excluded.require_auth,oidc_backfill_done=excluded.oidc_backfill_done`,
		a.Issuer, a.ClientID, a.ClientSecret, boolInt(a.Enabled), boolInt(a.RequireAuth), boolInt(a.OIDCBackfillDone))
	if err != nil {
		return model.AuthSettings{}, err
	}
	return s.GetAuthSettings()
}

func (s *SQLiteStore) UpsertUserBySubject(issuer, subject, email, name string) (model.User, error) {
	var id string
	err := s.db.QueryRow(`SELECT id FROM users WHERE issuer=? AND subject=?`, issuer, subject).Scan(&id)
	if err == sql.ErrNoRows {
		u := model.NewUser(issuer, subject, email, name)
		_, err := s.db.Exec(`INSERT INTO users(id,issuer,subject,email,name,is_admin,created_at) VALUES(?,?,?,?,?,?,?)`,
			u.ID, u.Issuer, u.Subject, u.Email, u.Name, 0, ts(u.CreatedAt))
		return u, err
	}
	if err != nil {
		return model.User{}, err
	}
	if _, err := s.db.Exec(`UPDATE users SET email=CASE WHEN ?='' THEN email ELSE ? END, name=CASE WHEN ?='' THEN name ELSE ? END WHERE id=?`,
		email, email, name, name, id); err != nil {
		return model.User{}, err
	}
	return scanUser(s.db.QueryRow(`SELECT id,issuer,subject,email,name,COALESCE(is_admin,0),created_at FROM users WHERE id=?`, id))
}

func (s *SQLiteStore) GetUser(id string) (model.User, error) {
	return scanUser(s.db.QueryRow(`SELECT id,issuer,subject,email,name,COALESCE(is_admin,0),created_at FROM users WHERE id=?`, id))
}

func (s *SQLiteStore) GetUserByEmail(email string) (model.User, error) {
	return scanUser(s.db.QueryRow(`SELECT id,issuer,subject,email,name,COALESCE(is_admin,0),created_at FROM users WHERE email=? AND email<>''`, email))
}

// SetUserAdmin flips the admin bit for one user (OSS-136).
func (s *SQLiteStore) SetUserAdmin(userID string, admin bool) (model.User, error) {
	res, err := s.db.Exec(`UPDATE users SET is_admin=? WHERE id=?`, boolInt(admin), userID)
	if err != nil {
		return model.User{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.User{}, fmt.Errorf("user not found")
	}
	return s.GetUser(userID)
}

// CountAdmins returns the number of admin users (OSS-136).
func (s *SQLiteStore) CountAdmins() (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE COALESCE(is_admin,0)!=0`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *SQLiteStore) CreateBoardWithOwner(title, ownerID, visibility string) (model.Board, error) {
	vis, err := normalizeVisibility(visibility)
	if err != nil {
		return model.Board{}, err
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM boards`).Scan(&n); err != nil {
		return model.Board{}, err
	}
	b := model.NewBoard(title, n)
	b.OwnerID, b.Visibility = ownerID, vis
	_, err = s.db.Exec(`INSERT INTO boards(id,title,background,position,owner_id,visibility,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`,
		b.ID, b.Title, b.Background, b.Position, b.OwnerID, b.Visibility, ts(b.CreatedAt), ts(b.UpdatedAt))
	return b, err
}

func (s *SQLiteStore) SetBoardVisibility(id, visibility string) (model.Board, error) {
	vis, err := normalizeVisibility(visibility)
	if err != nil {
		return model.Board{}, err
	}
	res, err := s.db.Exec(`UPDATE boards SET visibility=?, updated_at=? WHERE id=?`, vis, ts(time.Now().UTC()), id)
	if err != nil {
		return model.Board{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.Board{}, fmt.Errorf("board not found")
	}
	return s.getBoard(id)
}

func (s *SQLiteStore) SetBoardOwner(id, ownerID string) (model.Board, error) {
	res, err := s.db.Exec(`UPDATE boards SET owner_id=?, updated_at=? WHERE id=?`, ownerID, ts(time.Now().UTC()), id)
	if err != nil {
		return model.Board{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.Board{}, fmt.Errorf("board not found")
	}
	return s.getBoard(id)
}

func (s *SQLiteStore) ListBoardMembers(boardID string) ([]model.BoardMember, error) {
	rows, err := s.db.Query(`SELECT board_id,user_id,role FROM board_members WHERE board_id=?`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.BoardMember{}
	for rows.Next() {
		var m model.BoardMember
		if err := rows.Scan(&m.BoardID, &m.UserID, &m.Role); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) SetBoardMember(boardID, userID, role string) (model.BoardMember, error) {
	if role != model.RoleViewer && role != model.RoleEditor {
		return model.BoardMember{}, fmt.Errorf("role must be viewer or editor")
	}
	_, err := s.db.Exec(`INSERT INTO board_members(board_id,user_id,role) VALUES(?,?,?)
		ON CONFLICT(board_id,user_id) DO UPDATE SET role=excluded.role`, boardID, userID, role)
	if err != nil {
		return model.BoardMember{}, err
	}
	return model.BoardMember{BoardID: boardID, UserID: userID, Role: role}, nil
}

func (s *SQLiteStore) RemoveBoardMember(boardID, userID string) error {
	res, err := s.db.Exec(`DELETE FROM board_members WHERE board_id=? AND user_id=?`, boardID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("member not found")
	}
	return nil
}

func (s *SQLiteStore) GetBoardMember(boardID, userID string) (model.BoardMember, error) {
	var m model.BoardMember
	err := s.db.QueryRow(`SELECT board_id,user_id,role FROM board_members WHERE board_id=? AND user_id=?`, boardID, userID).
		Scan(&m.BoardID, &m.UserID, &m.Role)
	if err == sql.ErrNoRows {
		return m, fmt.Errorf("member not found")
	}
	return m, err
}

func (s *SQLiteStore) CreateSession(userID, tokenHash string, expires time.Time) (model.Session, error) {
	se := model.Session{TokenHash: tokenHash, UserID: userID, CreatedAt: time.Now().UTC(), ExpiresAt: expires}
	_, err := s.db.Exec(`INSERT INTO sessions(token_hash,user_id,created_at,expires_at) VALUES(?,?,?,?)`,
		se.TokenHash, se.UserID, ts(se.CreatedAt), ts(se.ExpiresAt))
	return se, err
}

func (s *SQLiteStore) GetSession(tokenHash string) (model.Session, error) {
	var se model.Session
	var ca, ea string
	err := s.db.QueryRow(`SELECT token_hash,user_id,created_at,expires_at FROM sessions WHERE token_hash=?`, tokenHash).
		Scan(&se.TokenHash, &se.UserID, &ca, &ea)
	if err == sql.ErrNoRows {
		return se, fmt.Errorf("session not found")
	}
	if err != nil {
		return se, err
	}
	se.CreatedAt, se.ExpiresAt = parseTS(ca), parseTS(ea)
	if !se.ExpiresAt.IsZero() && time.Now().UTC().After(se.ExpiresAt) {
		return model.Session{}, fmt.Errorf("session expired")
	}
	return se, nil
}

func (s *SQLiteStore) DeleteSession(tokenHash string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash=?`, tokenHash)
	return err
}

// ClaimUnownedBoards assigns every board with an empty owner to userID
// on the first call and records the backfill flag; later calls are a
// no-op. Boards that already have an owner are never touched.
func (s *SQLiteStore) ClaimUnownedBoards(userID string) (int, error) {
	if userID == "" {
		return 0, fmt.Errorf("user id required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var done int
	if err := tx.QueryRow(`SELECT COALESCE(oidc_backfill_done,0) FROM auth_settings WHERE id=1`).Scan(&done); err != nil {
		return 0, err
	}
	if done != 0 {
		return 0, nil
	}
	res, err := tx.Exec(`UPDATE boards SET owner_id=?, updated_at=? WHERE COALESCE(owner_id,'')=''`, userID, ts(time.Now().UTC()))
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	if _, err := tx.Exec(`UPDATE auth_settings SET oidc_backfill_done=1 WHERE id=1`); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(n), nil
}

// GetUserPrefs returns stored prefs or defaults when absent (OSS-83).
func (s *SQLiteStore) GetUserPrefs(userID string) (model.UserPrefs, error) {
	var p model.UserPrefs
	err := s.db.QueryRow(`SELECT COALESCE(theme,'paper'),COALESCE(language,'en') FROM user_prefs WHERE user_id=?`, userID).
		Scan(&p.Theme, &p.Language)
	if err == sql.ErrNoRows {
		return model.DefaultUserPrefs(), nil
	}
	if err != nil {
		return model.UserPrefs{}, err
	}
	return p.Normalize(), nil
}

// UpdateUserPrefs normalizes, upserts and returns stored prefs (OSS-83).
func (s *SQLiteStore) UpdateUserPrefs(userID string, prefs model.UserPrefs) (model.UserPrefs, error) {
	if userID == "" {
		return model.UserPrefs{}, fmt.Errorf("user id required")
	}
	prefs = prefs.Normalize()
	_, err := s.db.Exec(`INSERT INTO user_prefs(user_id,theme,language) VALUES(?,?,?)
		ON CONFLICT(user_id) DO UPDATE SET theme=excluded.theme,language=excluded.language`,
		userID, prefs.Theme, prefs.Language)
	if err != nil {
		return model.UserPrefs{}, err
	}
	return prefs, nil
}

func (s *SQLiteStore) BoardIDForColumn(columnID string) (string, error) {
	var boardID string
	err := s.db.QueryRow(`SELECT board_id FROM columns WHERE id=?`, columnID).Scan(&boardID)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("column not found")
	}
	return boardID, err
}

func (s *SQLiteStore) BoardIDForCard(cardID string) (string, error) {
	var colID string
	err := s.db.QueryRow(`SELECT column_id FROM cards WHERE id=?`, cardID).Scan(&colID)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("card not found")
	}
	if err != nil {
		return "", err
	}
	return s.BoardIDForColumn(colID)
}
