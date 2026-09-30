package api

import (
	"path/filepath"
	"testing"

	"github.com/crueber/loom-rebuild/internal/store"
)

// The four OSS-136 login cases, on both backends via syncAdminOnLogin.
func TestSyncAdminOnLogin(t *testing.T) {
	newStores := func(t *testing.T) map[string]store.Store {
		fs, err := store.OpenFile("")
		if err != nil {
			t.Fatal(err)
		}
		sq, err := store.OpenSQLite(filepath.Join(t.TempDir(), "loom.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { sq.Close() })
		return map[string]store.Store{"file": fs, "sqlite": sq}
	}

	t.Run("first user without group becomes admin", func(t *testing.T) {
		for name, st := range newStores(t) {
			u, err := st.UpsertUserBySubject("iss", "first-"+name, "f@test", "F")
			if err != nil {
				t.Fatal(err)
			}
			u, err = syncAdminOnLogin(st, u, nil)
			if err != nil || !u.IsAdmin {
				t.Fatalf("%s: first user must become admin: %+v %v", name, u, err)
			}
		}
	})

	t.Run("user with admin group becomes admin", func(t *testing.T) {
		for name, st := range newStores(t) {
			// Seed an existing admin so bootstrap does not apply.
			seed, err := st.UpsertUserBySubject("iss", "seed-"+name, "s@test", "S")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := st.SetUserAdmin(seed.ID, true); err != nil {
				t.Fatal(err)
			}
			u, err := st.UpsertUserBySubject("iss", "grp-"+name, "g@test", "G")
			if err != nil {
				t.Fatal(err)
			}
			u, err = syncAdminOnLogin(st, u, []string{"dev", "admin"})
			if err != nil || !u.IsAdmin {
				t.Fatalf("%s: grouped user must become admin: %+v %v", name, u, err)
			}
		}
	})

	t.Run("admin without group demoted when another admin exists", func(t *testing.T) {
		for name, st := range newStores(t) {
			a, err := st.UpsertUserBySubject("iss", "a-"+name, "a@test", "A")
			if err != nil {
				t.Fatal(err)
			}
			b, err := st.UpsertUserBySubject("iss", "b-"+name, "b@test", "B")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := st.SetUserAdmin(a.ID, true); err != nil {
				t.Fatal(err)
			}
			if b, err = st.SetUserAdmin(b.ID, true); err != nil {
				t.Fatal(err)
			}
			b, err = syncAdminOnLogin(st, b, nil)
			if err != nil || b.IsAdmin {
				t.Fatalf("%s: second admin without group must demote: %+v %v", name, b, err)
			}
		}
	})

	t.Run("sole admin without group keeps admin", func(t *testing.T) {
		for name, st := range newStores(t) {
			u, err := st.UpsertUserBySubject("iss", "sole-"+name, "s@test", "S")
			if err != nil {
				t.Fatal(err)
			}
			if u, err = st.SetUserAdmin(u.ID, true); err != nil {
				t.Fatal(err)
			}
			u, err = syncAdminOnLogin(st, u, nil)
			if err != nil || !u.IsAdmin {
				t.Fatalf("%s: sole admin must keep admin: %+v %v", name, u, err)
			}
		}
	})
}
