// Icon cache shared types (OSS-82 child scope).
// Server-cached link icons with a 7-day TTL; stdlib only.
package store

import (
	"database/sql"
	"fmt"
	"time"
)

// IconTTL is the single source of truth for icon freshness.
const IconTTL = 7 * 24 * time.Hour

// IconMaxBytes caps a single cached icon blob (256KB).
const IconMaxBytes = 256 * 1024

// IconRecord is one cached host icon.
type IconRecord struct {
	Host        string    `json:"host"`
	ContentType string    `json:"content_type"`
	Blob        []byte    `json:"blob"`
	FetchedAt   time.Time `json:"fetched_at"`
}

// IconStale reports whether rec is older than IconTTL.
func IconStale(rec IconRecord, now time.Time) bool {
	return now.Sub(rec.FetchedAt) >= IconTTL
}

// GetIcon loads a cached icon by host (exact lowercase match).
// SQLite backend.
func (s *SQLiteStore) GetIcon(host string) (IconRecord, error) {
	var rec IconRecord
	var fetched string
	err := s.db.QueryRow(`SELECT host,content_type,blob,fetched_at FROM icons WHERE host=?`, host).
		Scan(&rec.Host, &rec.ContentType, &rec.Blob, &fetched)
	if err == sql.ErrNoRows {
		return rec, fmt.Errorf("icon not found")
	}
	if err != nil {
		return rec, err
	}
	rec.FetchedAt = parseTS(fetched)
	return rec, nil
}

// SaveIcon inserts or replaces a cached icon.
func (s *SQLiteStore) SaveIcon(host, contentType string, blob []byte) (IconRecord, error) {
	if len(blob) > IconMaxBytes {
		return IconRecord{}, fmt.Errorf("icon too large")
	}
	rec := IconRecord{Host: host, ContentType: contentType, Blob: blob, FetchedAt: time.Now().UTC()}
	_, err := s.db.Exec(`INSERT OR REPLACE INTO icons(host,content_type,blob,fetched_at) VALUES(?,?,?,?)`,
		rec.Host, rec.ContentType, rec.Blob, ts(rec.FetchedAt))
	return rec, err
}

// SetIconFetchedAt backdates/forwards an entry (tests/maintenance).
func (s *SQLiteStore) SetIconFetchedAt(host string, t time.Time) error {
	_, err := s.db.Exec(`UPDATE icons SET fetched_at=? WHERE host=?`, ts(t), host)
	return err
}

// SetIconFetchedAt backdates/forwards an entry (tests/maintenance).
func (s *FileStore) SetIconFetchedAt(host string, t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.snap.Icons[host]
	if !ok {
		return fmt.Errorf("icon not found")
	}
	rec.FetchedAt = t
	s.snap.Icons[host] = rec
	return s.persistLocked()
}

// GetIcon loads a cached icon (file backend).
func (s *FileStore) GetIcon(host string) (IconRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.snap.Icons[host]
	if !ok {
		return IconRecord{}, fmt.Errorf("icon not found")
	}
	return rec, nil
}

// SaveIcon inserts or replaces a cached icon (file backend, persisted).
func (s *FileStore) SaveIcon(host, contentType string, blob []byte) (IconRecord, error) {
	if len(blob) > IconMaxBytes {
		return IconRecord{}, fmt.Errorf("icon too large")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.snap.Icons == nil {
		s.snap.Icons = map[string]IconRecord{}
	}
	rec := IconRecord{Host: host, ContentType: contentType, Blob: blob, FetchedAt: time.Now().UTC()}
	s.snap.Icons[host] = rec
	return rec, s.persistLocked()
}
