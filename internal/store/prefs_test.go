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
		got, err := st.UpdateUserPrefs("u1", model.UserPrefs{Theme: "neon", Language: "xx"})
		if err != nil {
			t.Fatalf("%s update: %v", name, err)
		}
		if got.Theme != "paper" || got.Language != "en" {
			t.Fatalf("%s normalized: got %+v", name, got)
		}
		back, err := st.GetUserPrefs("u1")
		if err != nil || back != got {
			t.Fatalf("%s roundtrip: wrote %+v, read %+v (%v)", name, got, back, err)
		}
	}
}
