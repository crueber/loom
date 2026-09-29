// Auth + permission methods for FileStore (OSS-50, UI-configured OIDC).
package store

import (
	"fmt"
	"time"

	"github.com/crueber/loom-rebuild/internal/model"
)

// normalizeVisibility accepts "" (legacy), "public", "private".
func normalizeVisibility(v string) (string, error) {
	switch v {
	case "", model.VisibilityPublic:
		return model.VisibilityPublic, nil
	case model.VisibilityPrivate:
		return model.VisibilityPrivate, nil
	default:
		return "", fmt.Errorf("visibility must be public or private")
	}
}

func (s *FileStore) GetAuthSettings() (model.AuthSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snap.Auth, nil
}

func (s *FileStore) UpdateAuthSettings(a model.AuthSettings) (model.AuthSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snap.Auth = a
	return s.snap.Auth, s.persistLocked()
}

func (s *FileStore) UpsertUserBySubject(issuer, subject, email, name string) (model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, u := range s.snap.Users {
		if u.Issuer == issuer && u.Subject == subject {
			if email != "" {
				s.snap.Users[i].Email = email
			}
			if name != "" {
				s.snap.Users[i].Name = name
			}
			return s.snap.Users[i], s.persistLocked()
		}
	}
	u := model.NewUser(issuer, subject, email, name)
	s.snap.Users = append(s.snap.Users, u)
	return u, s.persistLocked()
}

func (s *FileStore) GetUser(id string) (model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.snap.Users {
		if u.ID == id {
			return u, nil
		}
	}
	return model.User{}, fmt.Errorf("user not found")
}

func (s *FileStore) GetUserByEmail(email string) (model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.snap.Users {
		if u.Email != "" && u.Email == email {
			return u, nil
		}
	}
	return model.User{}, fmt.Errorf("user not found")
}

// CreateBoardWithOwner stores owner/visibility alongside the board.
func (s *FileStore) CreateBoardWithOwner(title, ownerID, visibility string) (model.Board, error) {
	vis, err := normalizeVisibility(visibility)
	if err != nil {
		return model.Board{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	b := model.NewBoard(title, len(s.snap.Boards))
	b.OwnerID = ownerID
	b.Visibility = vis
	s.snap.Boards = append(s.snap.Boards, b)
	return b, s.persistLocked()
}

func (s *FileStore) SetBoardVisibility(id, visibility string) (model.Board, error) {
	vis, err := normalizeVisibility(visibility)
	if err != nil {
		return model.Board{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, b := range s.snap.Boards {
		if b.ID == id {
			s.snap.Boards[i].Visibility = vis
			return s.snap.Boards[i], s.persistLocked()
		}
	}
	return model.Board{}, fmt.Errorf("board not found")
}

func (s *FileStore) SetBoardOwner(id, ownerID string) (model.Board, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, b := range s.snap.Boards {
		if b.ID == id {
			s.snap.Boards[i].OwnerID = ownerID
			return s.snap.Boards[i], s.persistLocked()
		}
	}
	return model.Board{}, fmt.Errorf("board not found")
}

func (s *FileStore) ListBoardMembers(boardID string) ([]model.BoardMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []model.BoardMember{}
	for _, m := range s.snap.Members {
		if m.BoardID == boardID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (s *FileStore) SetBoardMember(boardID, userID, role string) (model.BoardMember, error) {
	if role != model.RoleViewer && role != model.RoleEditor {
		return model.BoardMember{}, fmt.Errorf("role must be viewer or editor")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, m := range s.snap.Members {
		if m.BoardID == boardID && m.UserID == userID {
			s.snap.Members[i].Role = role
			return s.snap.Members[i], s.persistLocked()
		}
	}
	m := model.BoardMember{BoardID: boardID, UserID: userID, Role: role}
	s.snap.Members = append(s.snap.Members, m)
	return m, s.persistLocked()
}

func (s *FileStore) RemoveBoardMember(boardID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, m := range s.snap.Members {
		if m.BoardID == boardID && m.UserID == userID {
			s.snap.Members = append(s.snap.Members[:i], s.snap.Members[i+1:]...)
			return s.persistLocked()
		}
	}
	return fmt.Errorf("member not found")
}

func (s *FileStore) GetBoardMember(boardID, userID string) (model.BoardMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.snap.Members {
		if m.BoardID == boardID && m.UserID == userID {
			return m, nil
		}
	}
	return model.BoardMember{}, fmt.Errorf("member not found")
}

func (s *FileStore) CreateSession(userID, tokenHash string, expires time.Time) (model.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	se := model.Session{TokenHash: tokenHash, UserID: userID, CreatedAt: time.Now().UTC(), ExpiresAt: expires}
	s.snap.Sessions = append(s.snap.Sessions, se)
	return se, s.persistLocked()
}

func (s *FileStore) GetSession(tokenHash string) (model.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, se := range s.snap.Sessions {
		if se.TokenHash == tokenHash {
			if !se.ExpiresAt.IsZero() && time.Now().UTC().After(se.ExpiresAt) {
				return model.Session{}, fmt.Errorf("session expired")
			}
			return se, nil
		}
	}
	return model.Session{}, fmt.Errorf("session not found")
}

func (s *FileStore) DeleteSession(tokenHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, se := range s.snap.Sessions {
		if se.TokenHash == tokenHash {
			s.snap.Sessions = append(s.snap.Sessions[:i], s.snap.Sessions[i+1:]...)
			return s.persistLocked()
		}
	}
	return nil
}

func (s *FileStore) BoardIDForColumn(columnID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.snap.Columns {
		if c.ID == columnID {
			return c.BoardID, nil
		}
	}
	return "", fmt.Errorf("column not found")
}

func (s *FileStore) BoardIDForCard(cardID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for colID, cards := range s.snap.Cards {
		for _, c := range cards {
			if c.ID == cardID {
				for _, col := range s.snap.Columns {
					if col.ID == colID {
						return col.BoardID, nil
					}
				}
				return "", fmt.Errorf("column not found")
			}
		}
	}
	return "", fmt.Errorf("card not found")
}

// FileStore satisfies the extended Store interface.
var _ Store = (*FileStore)(nil)
