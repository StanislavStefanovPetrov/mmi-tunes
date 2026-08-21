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
		Bitrate:        9999,   // → 320
		SampleRate:     192000, // → 48000
		Channels:       7,      // → 2
		Concurrency:    99,     // → 5
		ThumbnailMaxPx: 5000,   // → 1000
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

// Every already-installed user has a settings.json written before
// verbose_logging existed. Loading one must not error and must leave
// verbose logging off — an upgrade should not silently start writing a log
// file, and must not disturb the fields the user did set.
func TestStore_UpgradeFromFileWithoutVerboseLogging(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	// A realistic v1.0.1 settings.json: every key of that release, no
	// verbose_logging.
	const old = `{"download_folder":"/Users/x/Music/MMI Tunes","bitrate":320,` +
		`"sample_rate":48000,"channels":2,"concurrency":3,"embed_metadata":true,` +
		`"embed_thumbnail":true,"thumbnail_max_px":800,"transliterate":true,` +
		`"generate_m3u":false,"dedup_history":true,"auto_detect_clipboard":true}`
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(path, Defaults("/tmp/default"))
	if err != nil {
		t.Fatalf("loading a pre-upgrade settings.json must not fail: %v", err)
	}
	got := store.Get()
	if got.VerboseLogging {
		t.Error("VerboseLogging defaulted to true on upgrade; a quiet app must stay quiet unless asked")
	}
	if got.DownloadFolder != "/Users/x/Music/MMI Tunes" {
		t.Errorf("upgrade clobbered the user's download folder: %q", got.DownloadFolder)
	}
}

func TestStore_VerboseLoggingRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	store, err := NewStore(path, Defaults("/tmp/default"))
	if err != nil {
		t.Fatal(err)
	}
	s := store.Get()
	s.VerboseLogging = true
	if err := store.Save(s); err != nil {
		t.Fatal(err)
	}

	// Re-open from disk: the toggle has to survive a restart or the user
	// silently loses verbose mode mid-investigation.
	reopened, err := NewStore(path, Defaults("/tmp/default"))
	if err != nil {
		t.Fatal(err)
	}
	if !reopened.Get().VerboseLogging {
		t.Error("VerboseLogging did not persist across reopen")
	}
}
