package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore_DefaultsOnFreshFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	defaults := Defaults("/tmp/x")

	store, err := NewStore(path, defaults)
	if err != nil {
		t.Fatal(err)
	}
	got := store.Get()
	if got.DownloadFolder != "/tmp/x" {
		t.Errorf("DownloadFolder = %q, want /tmp/x", got.DownloadFolder)
	}
	if got.Bitrate != 320 || got.SampleRate != 48000 || got.Channels != 2 {
		t.Errorf("MMI defaults wrong: %+v", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("settings.json should exist: %v", err)
	}
}

func TestStore_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	store, err := NewStore(path, Defaults("/tmp/x"))
	if err != nil {
		t.Fatal(err)
	}
	custom := Settings{
		DownloadFolder: "/tmp/custom",
		Bitrate:        256,
		SampleRate:     44100,
		Channels:       1,
		Concurrency:    2,
		EmbedMetadata:  true,
		EmbedThumbnail: false,
		ThumbnailMaxPx: 600,
		Transliterate:  false,
		DedupHistory:   false,
	}
	if err := store.Save(custom); err != nil {
		t.Fatal(err)
	}

	// Re-open and confirm everything round-tripped.
	store2, err := NewStore(path, Defaults("/tmp/different-default"))
	if err != nil {
		t.Fatal(err)
	}
	got := store2.Get()
	if got.DownloadFolder != "/tmp/custom" || got.Bitrate != 256 || got.SampleRate != 44100 ||
		got.Channels != 1 || got.Concurrency != 2 || got.ThumbnailMaxPx != 600 {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestStore_ClampOnSave(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(filepath.Join(dir, "s.json"), Defaults("/tmp/x"))
	insane := Settings{
		DownloadFolder: "/tmp/x",
		Bitrate:        9999, // → 320
		SampleRate:     192000, // → 48000
		Channels:       7,    // → 2
		Concurrency:    99,   // → 5
		ThumbnailMaxPx: 5000, // → 1000
	}
	if err := store.Save(insane); err != nil {
		t.Fatal(err)
	}
	got := store.Get()
	if got.Bitrate != 320 || got.SampleRate != 48000 || got.Channels != 2 ||
		got.Concurrency != 5 || got.ThumbnailMaxPx != 1000 {
		t.Errorf("clamp failed: %+v", got)
	}
}

func TestStore_CorruptFileFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte("not json {"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(path, Defaults("/tmp/x"))
	if err != nil {
		t.Fatal(err)
	}
	if got := store.Get(); got.Bitrate != 320 {
		t.Errorf("expected defaults after corruption, got Bitrate=%d", got.Bitrate)
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Errorf("expected .bak to be created after recovering from corrupt file: %v", err)
	}
}

func TestStore_MergesPartialFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	// Write only a few fields; the rest should fall back to defaults.
	if err := os.WriteFile(path, []byte(`{"download_folder":"/tmp/partial","bitrate":192}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(path, Defaults("/tmp/default"))
	if err != nil {
		t.Fatal(err)
	}
	got := store.Get()
	if got.DownloadFolder != "/tmp/partial" || got.Bitrate != 192 {
		t.Errorf("partial-load preserve failed: %+v", got)
	}
	if got.SampleRate != 48000 || got.Channels != 2 || got.Concurrency != 3 {
		t.Errorf("missing fields not filled from defaults: %+v", got)
	}
}
