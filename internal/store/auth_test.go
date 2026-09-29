package store

import (
	"path/filepath"
	"testing"

	"github.com/crueber/loom-rebuild/internal/model"
)

// Settings persist across restarts on both backends (OSS-50: the admin
// enables OIDC via the UI and it must survive a reboot).
func TestAuthSettingsPersist(t *testing.T) {
	want := model.AuthSettings{Issuer: "https://issuer.test", ClientID: "cid", ClientSecret: "shh", Enabled: true, RequireAuth: true}

	fs, err := OpenFile(filepath.Join(t.TempDir(), "loom.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.UpdateAuthSettings(want); err != nil {
		t.Fatal(err)
	}
	fs2, err := OpenFile(fs.path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := fs2.GetAuthSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("file settings roundtrip: got %+v want %+v", got, want)
	}

	sq, err := OpenSQLite(filepath.Join(t.TempDir(), "loom.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sq.Close()
	if _, err := sq.UpdateAuthSettings(want); err != nil {
		t.Fatal(err)
	}
	got, err = sq.GetAuthSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("sqlite settings roundtrip: got %+v want %+v", got, want)
	}
}

// Legacy boards (created before auth existed) stay public with no owner,
// and board permission primitives roundtrip on both backends.
func TestBoardPermissionsRoundtrip(t *testing.T) {
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
		b, err := st.CreateBoard("Legacy")
		if err != nil {
			t.Fatalf("%s create: %v", name, err)
		}
		if b.EffectiveVisibility() != model.VisibilityPublic || b.OwnerID != "" {
			t.Fatalf("%s legacy board must be public/unowned: %+v", name, b)
		}
		u, err := st.UpsertUserBySubject("iss", "sub", "e@test", "N")
		if err != nil {
			t.Fatalf("%s upsert user: %v", name, err)
		}
		if _, err := st.SetBoardVisibility(b.ID, model.VisibilityPrivate); err != nil {
			t.Fatalf("%s visibility: %v", name, err)
		}
		if _, err := st.SetBoardOwner(b.ID, u.ID); err != nil {
			t.Fatalf("%s owner: %v", name, err)
		}
		if _, err := st.SetBoardMember(b.ID, u.ID, model.RoleEditor); err != nil {
			t.Fatalf("%s member: %v", name, err)
		}
		m, err := st.GetBoardMember(b.ID, u.ID)
		if err != nil || m.Role != model.RoleEditor {
			t.Fatalf("%s get member: %+v %v", name, m, err)
		}
		col, err := st.CreateColumn(b.ID, "C", "")
		if err != nil {
			t.Fatalf("%s column: %v", name, err)
		}
		if bid, err := st.BoardIDForColumn(col.ID); err != nil || bid != b.ID {
			t.Fatalf("%s board-for-column: %q %v", name, bid, err)
		}
		card, err := st.CreateCard(col.ID, nil)
		if err != nil {
			t.Fatalf("%s card: %v", name, err)
		}
		if bid, err := st.BoardIDForCard(card.ID); err != nil || bid != b.ID {
			t.Fatalf("%s board-for-card: %q %v", name, bid, err)
		}
	}
}
