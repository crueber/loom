package store

import (
	"path/filepath"
	"testing"
)

// IsAdmin roundtrips on both backends; UpsertUserBySubject preserves it.
func TestAdminBitRoundtrip(t *testing.T) {
	fs, err := OpenFile("")
	if err != nil {
		t.Fatal(err)
	}
	sq, err := OpenSQLite(filepath.Join(t.TempDir(), "loom.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sq.Close()
	stores := map[string]Store{"file": fs, "sqlite": sq}
	for name, st := range stores {
		u, err := st.UpsertUserBySubject("iss", "sub-"+name, "e@test", "N")
		if err != nil {
			t.Fatalf("%s upsert: %v", name, err)
		}
		if u.IsAdmin {
			t.Fatalf("%s new user must not be admin", name)
		}
		u, err = st.SetUserAdmin(u.ID, true)
		if err != nil || !u.IsAdmin {
			t.Fatalf("%s set admin: %+v %v", name, u, err)
		}
		if n, err := st.CountAdmins(); err != nil || n != 1 {
			t.Fatalf("%s count admins: %d %v", name, n, err)
		}
		// Email/name update must preserve the admin bit.
		u2, err := st.UpsertUserBySubject("iss", "sub-"+name, "new@test", "New")
		if err != nil || !u2.IsAdmin {
			t.Fatalf("%s upsert must preserve admin: %+v %v", name, u2, err)
		}
		u, err = st.SetUserAdmin(u.ID, false)
		if err != nil || u.IsAdmin {
			t.Fatalf("%s demote: %+v %v", name, u, err)
		}
	}
}
