package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/crueber/loom-rebuild/internal/model"
)

// Todo checked state survives create/update/reload on both backends.
// One todo block = one checklist: a single list block roundtrips with
// its items intact.
func TestTodoCheckedRoundTrip(t *testing.T) {
	stores := map[string]Store{}
	sqlite, err := OpenSQLite(filepath.Join(t.TempDir(), "todo.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlite.Close()
	stores["sqlite"] = sqlite
	file, err := OpenFile(filepath.Join(t.TempDir(), "todo.json"))
	if err != nil {
		t.Fatal(err)
	}
	stores["file"] = file

	for name, st := range stores {
		b, err := st.CreateBoard("T")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		col, err := st.CreateColumn(b.ID, "C", "")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		done := model.NewTodoBlock("done")
		done.Items[0].Checked = true
		open := model.NewTodoBlock("open")
		list := model.Block{ID: "list1", Type: model.BlockTodo,
			Items: append(open.Items, done.Items...)}
		card, err := st.CreateCard(col.ID, []model.Block{list})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		tree, err := st.GetTree(b.ID)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		blocks := tree.Cards[col.ID][0].Blocks
		if len(blocks) != 1 || blocks[0].Type != model.BlockTodo || len(blocks[0].Items) != 2 {
			t.Fatalf("%s: want one checklist block with 2 items, got %+v", name, blocks)
		}
		if blocks[0].Items[0].Content != "open" || blocks[0].Items[0].Checked ||
			blocks[0].Items[1].Content != "done" || !blocks[0].Items[1].Checked {
			t.Fatalf("%s: bad reload: %+v", name, blocks)
		}
		// Toggle via full-card persist, then reload.
		card = tree.Cards[col.ID][0]
		card.Blocks[0].Items[0].Checked = true
		if _, err := st.UpdateCard(card); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		tree2, err := st.GetTree(b.ID)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := tree2.Cards[col.ID][0].Blocks; len(got) != 1 || !got[0].Items[0].Checked || !got[0].Items[1].Checked {
			t.Fatalf("%s: toggle lost: %+v", name, got)
		}
	}
}

// Consecutive legacy single-row todo blocks (Content/Checked, no Items)
// migrate into one checklist block on PATCH/read.
func TestLegacyTodoRowsMerge(t *testing.T) {
	legacy := []model.Block{
		{ID: "r1", Type: model.BlockTodo, Content: "one"},
		{ID: "r2", Type: model.BlockTodo, Content: "two", Checked: true},
		{ID: "n", Type: model.BlockNote, Content: "sep"},
		{ID: "r3", Type: model.BlockTodo, Content: "three"},
	}
	got := model.NormalizeTodoBlocks(legacy)
	if len(got) != 3 {
		t.Fatalf("want 3 blocks (list, note, list), got %+v", got)
	}
	if got[0].Type != model.BlockTodo || len(got[0].Items) != 2 ||
		got[0].Items[0].ID != "r1" || got[0].Items[0].Content != "one" ||
		got[0].Items[1].ID != "r2" || !got[0].Items[1].Checked {
		t.Fatalf("first run not merged: %+v", got[0])
	}
	if got[1].Type != model.BlockNote || got[2].Type != model.BlockTodo || len(got[2].Items) != 1 ||
		got[2].Items[0].Content != "three" {
		t.Fatalf("separator run wrong: %+v", got)
	}
	// Adjacent modern lists (with Items) never merge: + todo stays separate.
	modern := []model.Block{
		{ID: "m1", Type: model.BlockTodo, Items: []model.TodoItem{{ID: "i1", Content: "l1"}}},
		{ID: "m2", Type: model.BlockTodo, Items: []model.TodoItem{{ID: "i2", Content: "l2"}}},
	}
	if got := model.NormalizeTodoBlocks(modern); len(got) != 2 || got[0].ID != "m1" || got[1].ID != "m2" {
		t.Fatalf("modern lists merged: %+v", got)
	}
	// Deprecated fallback: ItemsOf exposes Content/Checked rows.
	fb := model.TodoItems(model.Block{ID: "x", Type: model.BlockTodo, Content: "hi", Checked: true})
	if len(fb) != 1 || fb[0].Content != "hi" || !fb[0].Checked {
		t.Fatalf("fallback read path broken: %+v", fb)
	}
}

// Pre-todo SQLite files gain the checked column via migration.
func TestSQLiteTodoMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, ddl := range []string{
		`CREATE TABLE boards (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', background TEXT NOT NULL DEFAULT '', position INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE columns (id TEXT PRIMARY KEY, board_id TEXT NOT NULL, title TEXT NOT NULL DEFAULT '', color TEXT NOT NULL DEFAULT '', position INTEGER NOT NULL DEFAULT 0, collapsed INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL)`,
		`CREATE TABLE cards (id TEXT PRIMARY KEY, column_id TEXT NOT NULL, position INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE blocks (id TEXT PRIMARY KEY, card_id TEXT NOT NULL, type TEXT NOT NULL, position INTEGER NOT NULL DEFAULT 0, url TEXT NOT NULL DEFAULT '', title TEXT NOT NULL DEFAULT '', content TEXT NOT NULL DEFAULT '', image_url TEXT NOT NULL DEFAULT '', thumb_url TEXT NOT NULL DEFAULT '', alt TEXT NOT NULL DEFAULT '')`,
		`INSERT INTO boards(id,title,background,position,created_at,updated_at) VALUES('b','T','',0,'t','t')`,
		`INSERT INTO columns(id,board_id,title,color,position,collapsed,created_at) VALUES('c','b','C','',0,0,'t')`,
		`INSERT INTO cards(id,column_id,position,created_at,updated_at) VALUES('k','c',0,'t','t')`,
		`INSERT INTO blocks(id,card_id,type,position,content) VALUES('n','k','note',0,'hello')`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()

	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	tree, err := st.GetTree("b")
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Cards["c"][0].Blocks) != 1 {
		t.Fatalf("old rows unreadable: %+v", tree)
	}
	card := tree.Cards["c"][0]
	migrated := model.NewTodoBlock("migrated")
	migrated.Items[0].Checked = true
	card.Blocks = append(card.Blocks, migrated)
	updated, err := st.UpdateCard(card)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Blocks) != 2 || len(updated.Blocks[1].Items) != 1 || !updated.Blocks[1].Items[0].Checked {
		t.Fatalf("checked lost after migration: %+v", updated.Blocks)
	}
}
