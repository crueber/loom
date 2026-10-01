package store

import (
	"path/filepath"
	"testing"

	"github.com/crueber/loom-rebuild/internal/model"
)

// Both backends return defaults for unknown users and persist normalized
// prefs (OSS-83).
func TestUserPrefsBothBackends(t *testing.T) {
	file, err := OpenFile("")
	if err != nil {
		t.Fatal(err)
	}
	sqlite, err := OpenSQLite(filepath.Join(t.TempDir(), "prefs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlite.Close()
	for name, st := range map[string]Store{"file": file, "sqlite": sqlite} {
		def, err := st.GetUserPrefs("ghost")
		if err != nil {
			t.Fatalf("%s get defaults: %v", name, err)
		}
		if def != model.DefaultUserPrefs() {
			t.Fatalf("%s defaults: got %+v", name, def)
		}
		// Prefs belong to real users (the API only writes prefs for the
		// authenticated user). With SQLite FK enforcement on (OSS-96),
		// upserting prefs for a dangling user id fails, so create the
		// user first on both backends alike.
		u, err := st.UpsertUserBySubject("test", "u1", "", "")
		if err != nil {
			t.Fatalf("%s create user: %v", name, err)
		}
		got, err := st.UpdateUserPrefs(u.ID, model.UserPrefs{Theme: "neon", Language: "xx", Animations: true})
		if err != nil {
			t.Fatalf("%s update: %v", name, err)
		}
		if got.Theme != "paper" || got.Language != "en" || got.Animations != true {
			t.Fatalf("%s normalized: got %+v", name, got)
		}
		back, err := st.GetUserPrefs(u.ID)
		if err != nil || back != got {
			t.Fatalf("%s roundtrip: wrote %+v, read %+v (%v)", name, got, back, err)
		}
		// OSS-158: explicit false persists.
		off, err := st.UpdateUserPrefs(u.ID, model.UserPrefs{Theme: "sky", Language: "en", Animations: false})
		if err != nil || off.Animations != false {
			t.Fatalf("%s animations off: wrote %+v (%v)", name, off, err)
		}
		backOff, err := st.GetUserPrefs(u.ID)
		if err != nil || backOff.Animations != false {
			t.Fatalf("%s animations roundtrip: got %+v (%v)", name, backOff, err)
		}
	}
}
