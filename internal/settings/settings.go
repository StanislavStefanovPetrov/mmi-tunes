// Package settings persists user preferences to a JSON file in the
// macOS Application Support directory.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// Settings is the persisted user-tunable configuration. Renaming fields
// is a breaking change for already-installed users.
type Settings struct {
	DownloadFolder      string `json:"download_folder"`
	Bitrate             int    `json:"bitrate"`     // kbps
	SampleRate          int    `json:"sample_rate"` // Hz
	Channels            int    `json:"channels"`    // 1 or 2
	Concurrency         int    `json:"concurrency"` // parallel downloads
	EmbedMetadata       bool   `json:"embed_metadata"`
	EmbedThumbnail      bool   `json:"embed_thumbnail"`
	ThumbnailMaxPx      int    `json:"thumbnail_max_px"`
	Transliterate       bool   `json:"transliterate"`
	GenerateM3U         bool   `json:"generate_m3u"`
	DedupHistory        bool   `json:"dedup_history"`
	AutoDetectClipboard bool   `json:"auto_detect_clipboard"`
	VerboseLogging      bool   `json:"verbose_logging"` // pass -v to yt-dlp, tee stderr to the log
}

// Defaults returns Audi-MMI-tuned defaults for a fresh install.
// downloadFolder must already be expanded to an absolute path.
func Defaults(downloadFolder string) Settings {
	return Settings{
		DownloadFolder:      downloadFolder,
		Bitrate:             320,
		SampleRate:          48000,
		Channels:            2,
		Concurrency:         3,
		EmbedMetadata:       true,
		EmbedThumbnail:      true,
		ThumbnailMaxPx:      800,
		Transliterate:       true,
		GenerateM3U:         false,
		DedupHistory:        true,
		AutoDetectClipboard: true,
		VerboseLogging:      false,
	}
}

// Store reads and writes the settings JSON file under a mutex.
type Store struct {
	mu   sync.RWMutex
	path string
	cur  Settings
}

// NewStore opens (or creates) settings.json at path. If the file is
// missing or invalid, defaults are written.
func NewStore(path string, defaults Settings) (*Store, error) {
	s := &Store{path: path}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir settings dir: %w", err)
	}
	loaded, err := readFile(path)
	switch {
	case err == nil:
		// Merge: keep loaded values, fill in any zero-valued field with default.
		s.cur = mergeWithDefaults(loaded, defaults)
	case errors.Is(err, fs.ErrNotExist):
		s.cur = defaults
		if err := writeFile(path, s.cur); err != nil {
			return nil, err
		}
	default:
		// Corrupted file — fall back to defaults but keep the corrupt one as .bak.
		_ = os.Rename(path, path+".bak")
		s.cur = defaults
		if err := writeFile(path, s.cur); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Get returns a copy of the current settings.
func (s *Store) Get() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cur
}

// Save validates and writes new settings.
func (s *Store) Save(in Settings) error {
	clean := clamp(in)
	s.mu.Lock()
	s.cur = clean
	s.mu.Unlock()
	return writeFile(s.path, clean)
}

func clamp(s Settings) Settings {
	if s.Bitrate < 64 {
		s.Bitrate = 64
	}
	if s.Bitrate > 320 {
		s.Bitrate = 320
	}
	if s.SampleRate < 22050 {
		s.SampleRate = 22050
	}
	if s.SampleRate > 48000 {
		s.SampleRate = 48000
	}
	if s.Channels < 1 {
		s.Channels = 1
	}
	if s.Channels > 2 {
		s.Channels = 2
	}
	if s.Concurrency < 1 {
		s.Concurrency = 1
	}
	if s.Concurrency > 5 {
		s.Concurrency = 5
	}
	if s.ThumbnailMaxPx < 100 {
		s.ThumbnailMaxPx = 100
	}
	if s.ThumbnailMaxPx > 1000 {
		s.ThumbnailMaxPx = 1000
	}
	return s
}

func mergeWithDefaults(loaded, defaults Settings) Settings {
	if loaded.DownloadFolder == "" {
		loaded.DownloadFolder = defaults.DownloadFolder
	}
	if loaded.Bitrate == 0 {
		loaded.Bitrate = defaults.Bitrate
	}
	if loaded.SampleRate == 0 {
		loaded.SampleRate = defaults.SampleRate
	}
	if loaded.Channels == 0 {
		loaded.Channels = defaults.Channels
	}
	if loaded.Concurrency == 0 {
		loaded.Concurrency = defaults.Concurrency
	}
	if loaded.ThumbnailMaxPx == 0 {
		loaded.ThumbnailMaxPx = defaults.ThumbnailMaxPx
	}
	return clamp(loaded)
}

func readFile(path string) (Settings, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Settings{}, err
	}
	var s Settings
	if err := json.Unmarshal(b, &s); err != nil {
		return Settings{}, fmt.Errorf("decode settings.json: %w", err)
	}
	return s, nil
}

func writeFile(path string, s Settings) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
