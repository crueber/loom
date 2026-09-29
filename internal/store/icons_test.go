package store

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"
)

func TestIconRoundTripSQLite(t *testing.T) {
	s, err := OpenSQLite(filepath.Join(t.TempDir(), "icons.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	blob := []byte{0x89, 'P', 'N', 'G'}
	if _, err := s.SaveIcon("example.com", "image/png", blob); err != nil {
		t.Fatal(err)
	}
	rec, err := s.GetIcon("example.com")
	if err != nil {
		t.Fatal(err)
	}
	if rec.ContentType != "image/png" || !bytes.Equal(rec.Blob, blob) {
		t.Fatalf("round trip mismatch: %+v", rec)
	}
	if IconStale(rec, time.Now()) {
		t.Fatal("fresh entry reports stale")
	}
	if !IconStale(rec, rec.FetchedAt.Add(IconTTL+time.Second)) {
		t.Fatal("old entry should be stale")
	}
}

func TestIconFilePersistsAndCaps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "loom.json")
	s, err := OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveIcon("example.com", "image/png", []byte{1, 2}); err != nil {
		t.Fatal(err)
	}
	// Reload from disk: entry survives.
	s2, err := OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := s2.GetIcon("example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rec.Blob, []byte{1, 2}) {
		t.Fatal("file persistence mismatch")
	}
	// Blob cap enforced on both backends.
	big := make([]byte, IconMaxBytes+1)
	if _, err := s.SaveIcon("big.com", "image/png", big); err == nil {
		t.Fatal("expected too-large error (file)")
	}
	sq, err := OpenSQLite(filepath.Join(t.TempDir(), "cap.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sq.Close()
	if _, err := sq.SaveIcon("big.com", "image/png", big); err == nil {
		t.Fatal("expected too-large error (sqlite)")
	}
}
