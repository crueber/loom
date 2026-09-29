package model

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// V1 export format (from the reference implementation in
// ~/dev/github.com/crueber/loom, internal/models ExportData).
// Kept here so the scratch rebuild can import v1 JSON cleanly.
type v1Export struct {
	Version    int       `json:"version"`
	ExportedAt time.Time `json:"exported_at"`
	Lists      []v1List  `json:"lists"`
}

type v1List struct {
	ID        int          `json:"id"`
	Title     string       `json:"title"`
	Color     string       `json:"color"`
	Position  int          `json:"position"`
	Collapsed bool         `json:"collapsed"`
	Bookmarks []v1Bookmark `json:"bookmarks"`
	Notes     []v1Note     `json:"notes"`
	Items     []v1Item     `json:"items"`
}

type v1Bookmark struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Position int    `json:"position"`
}

type v1Note struct {
	ID       int    `json:"id"`
	Content  string `json:"content"`
	Position int    `json:"position"`
}

type v1Item struct {
	ID       int     `json:"id"`
	Type     string  `json:"type"` // bookmark | note
	Title    *string `json:"title"`
	URL      *string `json:"url"`
	Content  *string `json:"content"`
	Position int     `json:"position"`
}

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// normalizeLinkURL canonicalizes a link URL for import dedupe: trim
// surrounding space, case-fold the host, drop the fragment, and strip a
// single trailing slash (a bare "/" path counts as empty). The query is
// preserved. Unparseable input falls back to the trimmed raw string so
// the normalizer never drops a link by itself.
func normalizeLinkURL(raw string) string {
	s := strings.TrimSpace(raw)
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return s
	}
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	u.Path = strings.TrimSuffix(u.Path, "/")
	return u.String()
}

// ImportV1 migrates a v1 export payload into a single board tree.
// Each v1 list becomes a column (color/collapse/position preserved);
// each v1 item becomes a card holding one block.
// Links dedupe per list on normalized URL (v1 exports carry the same
// link in both "bookmarks" and "items"): first occurrence wins, order
// preserved. Empty URLs are never deduped.
func ImportV1(data []byte) (BoardTree, error) {
	var v1 v1Export
	if err := jsonUnmarshal(data, &v1); err != nil {
		return BoardTree{}, fmt.Errorf("invalid v1 export: %w", err)
	}
	t := time.Now().UTC()
	tree := BoardTree{
		Board: Board{ID: NewID(), Title: "Imported from Loom v1", CreatedAt: t, UpdatedAt: t},
		Cards: map[string][]Card{},
	}
	for _, l := range v1.Lists {
		col := Column{
			ID: NewID(), BoardID: tree.Board.ID,
			Title: l.Title, Color: l.Color,
			Position: l.Position, Collapsed: l.Collapsed, CreatedAt: t,
		}
		tree.Columns = append(tree.Columns, col)
		var cards []Card
		seenLink := map[string]bool{}
		add := func(pos int, b Block) {
			if b.Type == BlockLink {
				if key := normalizeLinkURL(b.URL); key == "" {
					// Degenerate: keep, never dedupe empties.
				} else if seenLink[key] {
					return // dual-imported link: keep first, preserve order
				} else {
					seenLink[key] = true
				}
			}
			b.Position = 0
			cards = append(cards, NewCard(col.ID, pos, []Block{b}))
		}
		for _, bm := range l.Bookmarks {
			add(bm.Position, NewLinkBlock(bm.URL, bm.Title))
		}
		for _, n := range l.Notes {
			add(n.Position, NewNoteBlock(n.Content))
		}
		for _, it := range l.Items {
			switch it.Type {
			case "note":
				add(it.Position, NewNoteBlock(strVal(it.Content)))
			default: // bookmark and unknown default to link
				add(it.Position, NewLinkBlock(strVal(it.URL), strVal(it.Title)))
			}
		}
		// Stable order by position.
		for i := 0; i < len(cards); i++ {
			for j := i + 1; j < len(cards); j++ {
				if cards[j].Position < cards[i].Position {
					cards[i], cards[j] = cards[j], cards[i]
				}
			}
		}
		for i := range cards {
			cards[i].Position = i
		}
		tree.Cards[col.ID] = cards
	}
	return tree, nil
}
