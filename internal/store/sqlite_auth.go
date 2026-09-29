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
	err := row.Scan(&u.ID, &u.Issuer, &u.Subject, &u.Email, &u.Name, &ca)
	if err == sql.ErrNoRows {
		return u, fmt.Errorf("user not found")
	}
	u.CreatedAt = parseTS(ca)
	return u, err
}

func (s *SQLiteStore) GetAuthSettings() (model.AuthSettings, error) {
	var a model.AuthSettings
	var enabled, requireAuth int
	err := s.db.QueryRow(`SELECT COALESCE(issuer,''),COALESCE(client_id,''),COALESCE(client_secret,''),enabled,require_auth FROM auth_settings WHERE id=1`).
		Scan(&a.Issuer, &a.ClientID, &a.ClientSecret, &enabled, &requireAuth)
	if err == sql.ErrNoRows {
		return model.AuthSettings{}, nil
	}
	a.Enabled, a.RequireAuth = enabled != 0, requireAuth != 0
	return a, err
}

func (s *SQLiteStore) UpdateAuthSettings(a model.AuthSettings) (model.AuthSettings, error) {
	_, err := s.db.Exec(`INSERT INTO auth_settings(id,issuer,client_id,client_secret,enabled,require_auth) VALUES(1,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET issuer=excluded.issuer,client_id=excluded.client_id,client_secret=excluded.client_secret,enabled=excluded.enabled,require_auth=excluded.require_auth`,
		a.Issuer, a.ClientID, a.ClientSecret, boolInt(a.Enabled), boolInt(a.RequireAuth))
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
		_, err := s.db.Exec(`INSERT INTO users(id,issuer,subject,email,name,created_at) VALUES(?,?,?,?,?,?)`,
			u.ID, u.Issuer, u.Subject, u.Email, u.Name, ts(u.CreatedAt))
		return u, err
	}
	if err != nil {
		return model.User{}, err
	}
	if _, err := s.db.Exec(`UPDATE users SET email=CASE WHEN ?='' THEN email ELSE ? END, name=CASE WHEN ?='' THEN name ELSE ? END WHERE id=?`,
		email, email, name, name, id); err != nil {
		return model.User{}, err
	}
	return scanUser(s.db.QueryRow(`SELECT id,issuer,subject,email,name,created_at FROM users WHERE id=?`, id))
}

func (s *SQLiteStore) GetUser(id string) (model.User, error) {
	return scanUser(s.db.QueryRow(`SELECT id,issuer,subject,email,name,created_at FROM users WHERE id=?`, id))
}

func (s *SQLiteStore) GetUserByEmail(email string) (model.User, error) {
	return scanUser(s.db.QueryRow(`SELECT id,issuer,subject,email,name,created_at FROM users WHERE email=? AND email<>''`, email))
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
