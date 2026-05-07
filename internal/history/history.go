// Package history persists a list of completed downloads keyed by
// YouTube video ID for dedup.
package history

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Record is one completed download.
type Record struct {
	VideoID      string    `json:"video_id"`
	Title        string    `json:"title"`
	OutputPath   string    `json:"output_path"`
	DownloadedAt time.Time `json:"downloaded_at"`
}

// Store provides thread-safe lookup and persistence of records.
type Store struct {
	mu      sync.RWMutex
	path    string
	records map[string]Record
}

// NewStore opens (or creates) history.json at path.
func NewStore(path string) (*Store, error) {
	s := &Store{
		path:    path,
		records: map[string]Record{},
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return s, nil
	case err != nil:
		return nil, err
	}
	var list []Record
	if err := json.Unmarshal(b, &list); err != nil {
		// Corrupt — start fresh, keep .bak.
		_ = os.Rename(path, path+".bak")
		return s, nil
	}
	for _, r := range list {
		if r.VideoID != "" {
			s.records[r.VideoID] = r
		}
	}
	return s, nil
}

// Has reports whether videoID has been downloaded already.
func (s *Store) Has(videoID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.records[videoID]
	return ok
}

// Get returns the persisted record for videoID, if any.
func (s *Store) Get(videoID string) (Record, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.records[videoID]
	return r, ok
}

// Add inserts or updates a record and persists to disk.
func (s *Store) Add(r Record) error {
	if r.VideoID == "" {
		return errors.New("history: VideoID required")
	}
	if r.DownloadedAt.IsZero() {
		r.DownloadedAt = time.Now()
	}
	s.mu.Lock()
	s.records[r.VideoID] = r
	s.mu.Unlock()
	return s.flush()
}

// Remove forgets a record. Useful when the user deletes the file.
func (s *Store) Remove(videoID string) error {
	s.mu.Lock()
	delete(s.records, videoID)
	s.mu.Unlock()
	return s.flush()
}

// List returns all records.
func (s *Store) List() []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Record, 0, len(s.records))
	for _, r := range s.records {
		out = append(out, r)
	}
	return out
}

func (s *Store) flush() error {
	s.mu.RLock()
	list := make([]Record, 0, len(s.records))
	for _, r := range s.records {
		list = append(list, r)
	}
	s.mu.RUnlock()
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
