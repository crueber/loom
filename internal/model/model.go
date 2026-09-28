package model

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Block types.
const (
	BlockLink  = "link"
	BlockNote  = "note"
	BlockImage = "image"
)

// Board is a top-level page of columns.
type Board struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Background string    `json:"background,omitempty"`
	Position   int       `json:"position"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Column is a vertical stack of cards. It carries over the v1
// list color/collapse/position semantics.
type Column struct {
	ID        string    `json:"id"`
	BoardID   string    `json:"board_id"`
	Title     string    `json:"title"`
	Color     string    `json:"color"`
	Position  int       `json:"position"`
	Collapsed bool      `json:"collapsed"`
	CreatedAt time.Time `json:"created_at"`
}

// Card is a roomy container of ordered blocks.
type Card struct {
	ID        string    `json:"id"`
	ColumnID  string    `json:"column_id"`
	Position  int       `json:"position"`
	Blocks    []Block   `json:"blocks"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Block is one ordered unit inside a card: a link, a markdown note,
// or an image.
type Block struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // link | note | image
	Position int    `json:"position"`

	// Link fields.
	URL   string `json:"url,omitempty"`
	Title string `json:"title,omitempty"`

	// Note field (markdown).
	Content string `json:"content,omitempty"`

	// Image fields. ImageURL is a remote URL or a /images/… path for
	// server-stored uploads; ThumbURL is the server thumbnail.
	ImageURL string `json:"image_url,omitempty"`
	ThumbURL string `json:"thumb_url,omitempty"`
	Alt      string `json:"alt,omitempty"`
}

// BoardTree is a board with its columns and cards, used for bootstrap
// payloads and export.
type BoardTree struct {
	Board   Board    `json:"board"`
	Columns []Column `json:"columns"`
	// Cards keyed by column id.
	Cards map[string][]Card `json:"cards"`
}

// NewID returns a random 128-bit hex id.
func NewID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func now() time.Time { return time.Now().UTC() }

// NewBoard returns a board with generated id and timestamps.
func NewBoard(title string, position int) Board {
	t := now()
	return Board{ID: NewID(), Title: title, Position: position, CreatedAt: t, UpdatedAt: t}
}

// NewColumn returns a column with generated id.
func NewColumn(boardID, title, color string, position int) Column {
	return Column{ID: NewID(), BoardID: boardID, Title: title, Color: color, Position: position, CreatedAt: now()}
}

// NewCard returns a card with generated id.
func NewCard(columnID string, position int, blocks []Block) Card {
	t := now()
	if blocks == nil {
		blocks = []Block{}
	}
	for i := range blocks {
		if blocks[i].ID == "" {
			blocks[i].ID = NewID()
		}
		blocks[i].Position = i
	}
	return Card{ID: NewID(), ColumnID: columnID, Position: position, Blocks: blocks, CreatedAt: t, UpdatedAt: t}
}

// NewLinkBlock builds a link block.
func NewLinkBlock(url, title string) Block {
	return Block{ID: NewID(), Type: BlockLink, URL: url, Title: title}
}

// NewNoteBlock builds a markdown note block.
func NewNoteBlock(content string) Block {
	return Block{ID: NewID(), Type: BlockNote, Content: content}
}

// NewImageBlock builds an image block.
func NewImageBlock(imageURL, alt string) Block {
	return Block{ID: NewID(), Type: BlockImage, ImageURL: imageURL, Alt: alt}
}
