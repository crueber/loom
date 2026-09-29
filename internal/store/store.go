// Package store implements persistence behind the Store interface.
//
// The file-backed implementation below lets the API and UI land first
// with zero external dependencies (stdlib only). It honors the exact
// same interface the SQLite store will implement against schema.sql,
// so the driver swap is mechanical.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/crueber/loom-rebuild/internal/model"
)

// Store is the persistence contract. The SQLite implementation
// (schema.sql + database/sql driver) must satisfy this interface.
type Store interface {
	ListBoards() ([]model.Board, error)
	CreateBoard(title string) (model.Board, error)
	GetTree(boardID string) (model.BoardTree, error)
	UpdateBoard(id, title, background string) (model.Board, error)
	DeleteBoard(id string) error

	CreateColumn(boardID, title, color string) (model.Column, error)
	UpdateColumn(col model.Column) (model.Column, error)
	DeleteColumn(id string) error

	CreateCard(columnID string, blocks []model.Block) (model.Card, error)
	UpdateCard(card model.Card) (model.Card, error)
	DeleteCard(id string) error
	MoveCard(id, toColumnID string, position int) (model.Card, error)

	// ImportTree stores a full board tree (used by v1 import).
	ImportTree(tree model.BoardTree) (model.BoardTree, error)

	// Auth + board permissions (OSS-50, UI-configured OIDC). Additive:
	// when auth is disabled the board methods below behave like their
	// legacy counterparts.
	GetAuthSettings() (model.AuthSettings, error)
	UpdateAuthSettings(s model.AuthSettings) (model.AuthSettings, error)
	UpsertUserBySubject(issuer, subject, email, name string) (model.User, error)
	GetUser(id string) (model.User, error)
	GetUserByEmail(email string) (model.User, error)
	CreateBoardWithOwner(title, ownerID, visibility string) (model.Board, error)
	SetBoardVisibility(id, visibility string) (model.Board, error)
	SetBoardOwner(id, ownerID string) (model.Board, error)
	ListBoardMembers(boardID string) ([]model.BoardMember, error)
	SetBoardMember(boardID, userID, role string) (model.BoardMember, error)
	RemoveBoardMember(boardID, userID string) error
	GetBoardMember(boardID, userID string) (model.BoardMember, error)
	CreateSession(userID, tokenHash string, expires time.Time) (model.Session, error)
	GetSession(tokenHash string) (model.Session, error)
	DeleteSession(tokenHash string) error
	BoardIDForColumn(columnID string) (string, error)
	BoardIDForCard(cardID string) (string, error)

	// ClaimUnownedBoards assigns every board with an empty owner to
	// userID on the first call and records the backfill flag; later
	// calls are a no-op. Boards that already have an owner are never
	// touched. The future OIDC callback calls this once per sign-in
	// (OSS-71); with auth disabled there is no caller, so no behavior
	// change. Returns the number of boards claimed by this call.
	ClaimUnownedBoards(userID string) (int, error)
}

type snapshot struct {
	Boards  []model.Board           `json:"boards"`
	Columns []model.Column          `json:"columns"`
	Cards   map[string][]model.Card `json:"cards"`
	// Auth state (OSS-50). Absent in legacy files -> zero values:
	// auth disabled, boards public/unowned.
	Auth     model.AuthSettings  `json:"auth"`
	Users    []model.User        `json:"users"`
	Members  []model.BoardMember `json:"members"`
	Sessions []model.Session     `json:"sessions"`
}

// FileStore is a JSON-file-backed Store. Suitable for single-user
// local use; the SQLite swap keeps the same semantics.
type FileStore struct {
	mu   sync.Mutex
	path string
	snap snapshot
}

func OpenFile(path string) (*FileStore, error) {
	fs := &FileStore{path: path, snap: snapshot{Cards: map[string][]model.Card{}}}
	if path == "" {
		return fs, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return fs, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return fs, nil
	}
	if err := json.Unmarshal(data, &fs.snap); err != nil {
		return nil, fmt.Errorf("corrupt store file: %w", err)
	}
	if fs.snap.Cards == nil {
		fs.snap.Cards = map[string][]model.Card{}
	}
	return fs, nil
}

func (s *FileStore) persistLocked() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	data, err := json.Marshal(s.snap)
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *FileStore) ListBoards() ([]model.Board, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]model.Board{}, s.snap.Boards...)
	sort.Slice(out, func(i, j int) bool { return out[i].Position < out[j].Position })
	return out, nil
}

func (s *FileStore) CreateBoard(title string) (model.Board, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := model.NewBoard(title, len(s.snap.Boards))
	b.Visibility = model.VisibilityPublic
	s.snap.Boards = append(s.snap.Boards, b)
	return b, s.persistLocked()
}

func (s *FileStore) GetTree(boardID string) (model.BoardTree, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range s.snap.Boards {
		if b.ID != boardID {
			continue
		}
		tree := model.BoardTree{Board: b, Columns: []model.Column{}, Cards: map[string][]model.Card{}}
		for _, c := range s.snap.Columns {
			if c.BoardID == boardID {
				tree.Columns = append(tree.Columns, c)
			}
		}
		sort.Slice(tree.Columns, func(i, j int) bool { return tree.Columns[i].Position < tree.Columns[j].Position })
		for _, c := range tree.Columns {
			cards := append([]model.Card{}, s.snap.Cards[c.ID]...)
			sort.Slice(cards, func(i, j int) bool { return cards[i].Position < cards[j].Position })
			if cards == nil {
				cards = []model.Card{}
			}
			for i := range cards {
				cards[i].Blocks = model.NormalizeTodoBlocks(cards[i].Blocks)
			}
			tree.Cards[c.ID] = cards
		}
		return tree, nil
	}
	return model.BoardTree{}, fmt.Errorf("board not found")
}

func (s *FileStore) UpdateBoard(id, title, background string) (model.Board, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, b := range s.snap.Boards {
		if b.ID == id {
			s.snap.Boards[i].Title = title
			s.snap.Boards[i].Background = background
			return s.snap.Boards[i], s.persistLocked()
		}
	}
	return model.Board{}, fmt.Errorf("board not found")
}

func (s *FileStore) DeleteBoard(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, b := range s.snap.Boards {
		if b.ID == id {
			s.snap.Boards = append(s.snap.Boards[:i], s.snap.Boards[i+1:]...)
			var keepCols []model.Column
			deadCols := map[string]bool{}
			for _, c := range s.snap.Columns {
				if c.BoardID == id {
					deadCols[c.ID] = true
					continue
				}
				keepCols = append(keepCols, c)
			}
			s.snap.Columns = keepCols
			for cid := range deadCols {
				delete(s.snap.Cards, cid)
			}
			return s.persistLocked()
		}
	}
	return fmt.Errorf("board not found")
}

func (s *FileStore) CreateColumn(boardID, title, color string) (model.Column, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, c := range s.snap.Columns {
		if c.BoardID == boardID {
			n++
		}
	}
	col := model.NewColumn(boardID, title, color, n)
	s.snap.Columns = append(s.snap.Columns, col)
	return col, s.persistLocked()
}

func (s *FileStore) UpdateColumn(col model.Column) (model.Column, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.snap.Columns {
		if c.ID == col.ID {
			col.BoardID = c.BoardID
			col.CreatedAt = c.CreatedAt
			s.snap.Columns[i] = col
			return col, s.persistLocked()
		}
	}
	return model.Column{}, fmt.Errorf("column not found")
}

func (s *FileStore) DeleteColumn(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.snap.Columns {
		if c.ID == id {
			s.snap.Columns = append(s.snap.Columns[:i], s.snap.Columns[i+1:]...)
			delete(s.snap.Cards, id)
			return s.persistLocked()
		}
	}
	return fmt.Errorf("column not found")
}

func (s *FileStore) CreateCard(columnID string, blocks []model.Block) (model.Card, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	blocks = model.NormalizeTodoBlocks(blocks)
	card := model.NewCard(columnID, len(s.snap.Cards[columnID]), blocks)
	s.snap.Cards[columnID] = append(s.snap.Cards[columnID], card)
	return card, s.persistLocked()
}

func (s *FileStore) UpdateCard(card model.Card) (model.Card, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cards := s.snap.Cards[card.ColumnID]
	for i, c := range cards {
		if c.ID == card.ID {
			card.CreatedAt = c.CreatedAt
			card.Blocks = model.NormalizeTodoBlocks(card.Blocks)
			for j := range card.Blocks {
				if card.Blocks[j].ID == "" {
					card.Blocks[j].ID = model.NewID()
				}
				card.Blocks[j].Position = j
			}
			cards[i] = card
			s.snap.Cards[card.ColumnID] = cards
			return card, s.persistLocked()
		}
	}
	return model.Card{}, fmt.Errorf("card not found")
}

func (s *FileStore) DeleteCard(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for colID, cards := range s.snap.Cards {
		for i, c := range cards {
			if c.ID == id {
				s.snap.Cards[colID] = append(cards[:i], cards[i+1:]...)
				return s.persistLocked()
			}
		}
	}
	return fmt.Errorf("card not found")
}

func (s *FileStore) MoveCard(id, toColumnID string, position int) (model.Card, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for colID, cards := range s.snap.Cards {
		for i, c := range cards {
			if c.ID != id {
				continue
			}
			moved := c
			s.snap.Cards[colID] = append(cards[:i], cards[i+1:]...)
			for j := range s.snap.Cards[colID] {
				s.snap.Cards[colID][j].Position = j
			}
			moved.ColumnID = toColumnID
			dst := s.snap.Cards[toColumnID]
			if position < 0 || position > len(dst) {
				position = len(dst)
			}
			dst = append(dst, model.Card{})
			copy(dst[position+1:], dst[position:])
			moved.Position = position
			dst[position] = moved
			for j := range dst {
				dst[j].Position = j
			}
			s.snap.Cards[toColumnID] = dst
			return moved, s.persistLocked()
		}
	}
	return model.Card{}, fmt.Errorf("card not found")
}

func (s *FileStore) ImportTree(tree model.BoardTree) (model.BoardTree, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tree.Board.Position = len(s.snap.Boards)
	s.snap.Boards = append(s.snap.Boards, tree.Board)
	for _, c := range tree.Columns {
		s.snap.Columns = append(s.snap.Columns, c)
	}
	for colID, cards := range tree.Cards {
		s.snap.Cards[colID] = append(s.snap.Cards[colID], cards...)
	}
	return tree, s.persistLocked()
}
