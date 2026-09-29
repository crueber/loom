package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/crueber/loom-rebuild/internal/model"
)

// Todo checked state survives create/update/reload on both backends.
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
		done.Checked = true
		card, err := st.CreateCard(col.ID, []model.Block{model.NewTodoBlock("open"), done})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		tree, err := st.GetTree(b.ID)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		blocks := tree.Cards[col.ID][0].Blocks
		if len(blocks) != 2 || blocks[0].Type != model.BlockTodo || blocks[0].Checked ||
			blocks[1].Type != model.BlockTodo || !blocks[1].Checked || blocks[1].Content != "done" {
			t.Fatalf("%s: bad reload: %+v", name, blocks)
		}
		// Toggle via full-card persist, then reload.
		card.Blocks[0].Checked = true
		if _, err := st.UpdateCard(card); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		tree2, err := st.GetTree(b.ID)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := tree2.Cards[col.ID][0].Blocks; !got[0].Checked || !got[1].Checked {
			t.Fatalf("%s: toggle lost: %+v", name, got)
		}
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
	card.Blocks = append(card.Blocks, model.NewTodoBlock("migrated"))
	card.Blocks[1].Checked = true
	updated, err := st.UpdateCard(card)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Blocks) != 2 || !updated.Blocks[1].Checked {
		t.Fatalf("checked lost after migration: %+v", updated.Blocks)
	}
}
