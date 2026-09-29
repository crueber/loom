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
	BlockTodo  = "todo"
)

// Board is a top-level page of columns.
type Board struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Background string `json:"background,omitempty"`
	Position   int    `json:"position"`
	// OwnerID is the user that owns the board ("" = legacy/unowned,
	// treated as public). Visibility is "public" or "private"
	// ("" = legacy, treated as public).
	OwnerID    string    `json:"owner_id,omitempty"`
	Visibility string    `json:"visibility,omitempty"`
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

// TodoItem is one checklist row inside a single todo block.
// One todo block = one checklist; the Items array is the list.
type TodoItem struct {
	ID      string `json:"id"`
	Content string `json:"content,omitempty"`
	Checked bool   `json:"checked,omitempty"`
}

// Block is one ordered unit inside a card: a link, a markdown note,
// an image, or a todo-list checklist (one block = one list via Items).
type Block struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // link | note | image | todo
	Position int    `json:"position"`

	// Link fields.
	URL   string `json:"url,omitempty"`
	Title string `json:"title,omitempty"`

	// Note field (markdown).
	Content string `json:"content,omitempty"`

	// Todo fields. Content/Checked are a deprecated fallback read path
	// for old single-row todo blocks; Items is the source of truth.
	Checked bool       `json:"checked,omitempty"`
	Items   []TodoItem `json:"items,omitempty"`

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

// NewTodoBlock builds a todo-list checklist block holding one item.
func NewTodoBlock(content string) Block {
	return Block{ID: NewID(), Type: BlockTodo, Items: []TodoItem{{ID: NewID(), Content: content}}}
}

// NewTodoList builds an empty todo-list checklist block (one item draft).
func NewTodoList() Block {
	return Block{ID: NewID(), Type: BlockTodo, Items: []TodoItem{{ID: NewID()}}}
}

// TodoItems returns the checklist rows for a todo block, falling back to
// the deprecated Content/Checked read path for old single-row blocks.
func TodoItems(b Block) []TodoItem {
	if len(b.Items) > 0 {
		return b.Items
	}
	if b.Type != BlockTodo {
		return nil
	}
	id := b.ID
	if id == "" {
		id = NewID()
	}
	return []TodoItem{{ID: id, Content: b.Content, Checked: b.Checked}}
}

// NormalizeTodoBlocks upgrades legacy todo rows (blocks with no Items)
// to one-item checklist blocks and merges consecutive legacy rows into a
// single list. Blocks that already carry Items are modern explicit lists
// and NEVER merge, so adjacent lists from + todo stay separate. Positions
// are renumbered and empty ids backfilled.
func NormalizeTodoBlocks(blocks []Block) []Block {
	if len(blocks) == 0 {
		return blocks
	}
	out := make([]Block, 0, len(blocks))
	var run []Block
	flush := func() {
		if len(run) == 0 {
			return
		}
		if len(run) == 1 {
			b := run[0]
			items := TodoItems(b)
			for i := range items {
				if items[i].ID == "" {
					items[i].ID = NewID()
				}
			}
			b.Items = items
			b.Content = ""
			b.Checked = false
			out = append(out, b)
		} else {
			merged := Block{ID: run[0].ID, Type: BlockTodo}
			if merged.ID == "" {
				merged.ID = NewID()
			}
			for _, b := range run {
				for _, it := range TodoItems(b) {
					if it.ID == "" {
						it.ID = NewID()
					}
					merged.Items = append(merged.Items, it)
				}
			}
			out = append(out, merged)
		}
		run = nil
	}
	for _, b := range blocks {
		if b.Type == BlockTodo && len(b.Items) == 0 {
			run = append(run, b)
			continue
		}
		flush()
		out = append(out, b)
	}
	flush()
	for i := range out {
		out[i].Position = i
		if out[i].ID == "" {
			out[i].ID = NewID()
		}
		if out[i].Type == BlockTodo {
			for j := range out[i].Items {
				if out[i].Items[j].ID == "" {
					out[i].Items[j].ID = NewID()
				}
			}
		}
	}
	return out
}
