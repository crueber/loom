package store

import (
	"path/filepath"
	"testing"
)

// First OIDC sign-in claims every legacy unowned board exactly once
// (OSS-71): first claim owns all, a second user claims nothing new,
// re-runs are a no-op, and pre-owned boards are never stolen. Runs on
// both backends through the shared Store interface.
func TestClaimUnownedBoardsBackfill(t *testing.T) {
	stores := map[string]Store{}
	fs, err := OpenFile("")
	if err != nil {
		t.Fatal(err)
	}
	stores["file"] = fs
	sq, err := OpenSQLite(filepath.Join(t.TempDir(), "loom.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sq.Close()
	stores["sqlite"] = sq

	for name, st := range stores {
		b1, err := st.CreateBoard("Legacy One")
		if err != nil {
			t.Fatalf("%s create b1: %v", name, err)
		}
		b2, err := st.CreateBoard("Legacy Two")
		if err != nil {
			t.Fatalf("%s create b2: %v", name, err)
		}
		owner, err := st.UpsertUserBySubject("iss", "sub-"+name, "owner@test", "Owner")
		if err != nil {
			t.Fatalf("%s upsert owner: %v", name, err)
		}
		kept, err := st.CreateBoardWithOwner("Owned", owner.ID, "private")
		if err != nil {
			t.Fatalf("%s create owned: %v", name, err)
		}

		first, err := st.UpsertUserBySubject("iss", "first-"+name, "first@test", "First")
		if err != nil {
			t.Fatalf("%s upsert first: %v", name, err)
		}
		n, err := st.ClaimUnownedBoards(first.ID)
		if err != nil {
			t.Fatalf("%s first claim: %v", name, err)
		}
		if n != 2 {
			t.Fatalf("%s first claim must own 2 boards, got %d", name, n)
		}
		for _, id := range []string{b1.ID, b2.ID} {
			tree, err := st.GetTree(id)
			if err != nil {
				t.Fatalf("%s tree %s: %v", name, id, err)
			}
			if tree.Board.OwnerID != first.ID {
				t.Fatalf("%s board %s owner = %q, want %q", name, id, tree.Board.OwnerID, first.ID)
			}
		}
		keptTree, err := st.GetTree(kept.ID)
		if err != nil {
			t.Fatalf("%s owned tree: %v", name, err)
		}
		if keptTree.Board.OwnerID != owner.ID {
			t.Fatalf("%s pre-owned board stolen: owner = %q, want %q", name, keptTree.Board.OwnerID, owner.ID)
		}

		second, err := st.UpsertUserBySubject("iss", "second-"+name, "second@test", "Second")
		if err != nil {
			t.Fatalf("%s upsert second: %v", name, err)
		}
		n, err = st.ClaimUnownedBoards(second.ID)
		if err != nil {
			t.Fatalf("%s second claim: %v", name, err)
		}
		if n != 0 {
			t.Fatalf("%s second claim must be a no-op, got %d", name, n)
		}
		n, err = st.ClaimUnownedBoards(first.ID)
		if err != nil {
			t.Fatalf("%s re-run claim: %v", name, err)
		}
		if n != 0 {
			t.Fatalf("%s re-run must be a no-op, got %d", name, n)
		}
		settings, err := st.GetAuthSettings()
		if err != nil {
			t.Fatalf("%s settings: %v", name, err)
		}
		if !settings.OIDCBackfillDone {
			t.Fatalf("%s backfill flag not recorded", name)
		}
	}

	if _, err := fs.ClaimUnownedBoards(""); err == nil {
		t.Fatal("file: empty user id must fail")
	}
	if _, err := sq.ClaimUnownedBoards(""); err == nil {
		t.Fatal("sqlite: empty user id must fail")
	}
}
