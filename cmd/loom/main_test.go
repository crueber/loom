package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionedShell(t *testing.T) {
	tmpl := `<link rel="stylesheet" href="/styles.css"><script src="/app.js" defer></script>`
	got := versionedShell(tmpl, "abc123def456")
	if !strings.Contains(got, `src="/app.js?v=abc123def456"`) {
		t.Fatalf("script not versioned: %s", got)
	}
	if !strings.Contains(got, `href="/styles.css?v=abc123def456"`) {
		t.Fatalf("stylesheet not versioned: %s", got)
	}
	if versionedShell(tmpl, "") != tmpl {
		t.Fatalf("empty version must leave template untouched")
	}
}

func TestAssetVersionTracksBytes(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("app.js", "v1")
	write("styles.css", "s")
	v1 := assetVersion(dir)
	if len(v1) != 12 {
		t.Fatalf("want 12-hex-char version, got %q", v1)
	}
	write("app.js", "v2")
	if v2 := assetVersion(dir); v2 == v1 {
		t.Fatalf("version must change when bytes change")
	}
	if err := os.Remove(filepath.Join(dir, "styles.css")); err != nil {
		t.Fatal(err)
	}
	if v := assetVersion(dir); v != "" {
		t.Fatalf("missing asset must yield empty version, got %q", v)
	}
}
