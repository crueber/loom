package store

import (
	"path/filepath"
	"testing"

	"github.com/crueber/loom-rebuild/internal/model"
)

// SQLite must satisfy the same Store contract as the file backend:
// full board/column/card lifecycle plus a v1-imported tree round-trip.
func TestSQLiteStoreConformance(t *testing.T) {
	st, err := OpenSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	b, err := st.CreateBoard("Home")
	if err != nil {
		t.Fatal(err)
	}
	col, err := st.CreateColumn(b.ID, "Links", "blue")
	if err != nil {
		t.Fatal(err)
	}
	card, err := st.CreateCard(col.ID, []model.Block{
		model.NewLinkBlock("https://example.com", "Example"),
		model.NewNoteBlock("hello"),
	})
	if err != nil {
		t.Fatal(err)
	}
	col2, err := st.CreateColumn(b.ID, "Notes", "")
	if err != nil {
		t.Fatal(err)
	}
	moved, err := st.MoveCard(card.ID, col2.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if moved.ColumnID != col2.ID || moved.Position != 0 {
		t.Fatalf("bad move: %+v", moved)
	}

	tree, err := st.GetTree(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Columns) != 2 || len(tree.Cards[col2.ID]) != 1 || len(tree.Cards[col2.ID][0].Blocks) != 2 {
		t.Fatalf("unexpected tree: %+v", tree)
	}

	v1tree, err := model.ImportV1([]byte(`{"version":1,"lists":[{"id":1,"title":"T","color":"red","position":0,"collapsed":true,"items":[{"id":1,"type":"note","content":"n","position":0}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ImportTree(v1tree); err != nil {
		t.Fatal(err)
	}
	boards, err := st.ListBoards()
	if err != nil {
		t.Fatal(err)
	}
	if len(boards) != 2 {
		t.Fatalf("expected 2 boards, got %d", len(boards))
	}
	rt, err := st.GetTree(v1tree.Board.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !rt.Columns[0].Collapsed || rt.Columns[0].Color != "red" {
		t.Fatalf("v1 semantics lost: %+v", rt.Columns[0])
	}
}

// OSS-96: deleting a column must remove its cards and blocks too (no
// orphans). The schema declares ON DELETE CASCADE; OpenSQLite enables
// FK enforcement so the clauses actually apply.
func TestSQLiteDeleteColumnCascades(t *testing.T) {
	st, err := OpenSQLite(filepath.Join(t.TempDir(), "delcol.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	b, err := st.CreateBoard("Home")
	if err != nil {
		t.Fatal(err)
	}
	col, err := st.CreateColumn(b.ID, "Links", "blue")
	if err != nil {
		t.Fatal(err)
	}
	card, err := st.CreateCard(col.ID, []model.Block{
		model.NewLinkBlock("https://example.com", "Example"),
		model.NewNoteBlock("hello"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteColumn(col.ID); err != nil {
		t.Fatal(err)
	}
	var nCards, nBlocks int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM cards WHERE column_id=?`, col.ID).Scan(&nCards); err != nil {
		t.Fatal(err)
	}
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM blocks WHERE card_id=?`, card.ID).Scan(&nBlocks); err != nil {
		t.Fatal(err)
	}
	if nCards != 0 || nBlocks != 0 {
		t.Fatalf("orphans after column delete: cards=%d blocks=%d", nCards, nBlocks)
	}
	tree, err := st.GetTree(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Columns) != 0 {
		t.Fatalf("column still in tree: %+v", tree.Columns)
	}
	if err := st.DeleteColumn(col.ID); err == nil {
		t.Fatal("second delete should fail with column not found")
	}
}
