package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHistory_AddHasGet(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "h.json"))
	if err != nil {
		t.Fatal(err)
	}
	if store.Has("abc") {
		t.Fatal("Has on empty store should be false")
	}
	if err := store.Add(Record{VideoID: "abc", Title: "T", OutputPath: "/tmp/t.mp3"}); err != nil {
		t.Fatal(err)
	}
	if !store.Has("abc") {
		t.Error("Has after Add should be true")
	}
	r, ok := store.Get("abc")
	if !ok {
		t.Fatal("Get after Add should succeed")
	}
	if r.Title != "T" || r.OutputPath != "/tmp/t.mp3" {
		t.Errorf("Get returned %+v", r)
	}
	if r.DownloadedAt.IsZero() {
		t.Error("DownloadedAt should be auto-populated")
	}
}

func TestHistory_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "h.json")

	s1, _ := NewStore(path)
	for _, id := range []string{"a", "b", "c"} {
		_ = s1.Add(Record{VideoID: id, Title: "T-" + id, OutputPath: "/tmp/" + id + ".mp3"})
	}

	s2, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b", "c"} {
		if !s2.Has(id) {
			t.Errorf("missing %q after reload", id)
		}
	}
	if len(s2.List()) != 3 {
		t.Errorf("expected 3 records, got %d", len(s2.List()))
	}
}

func TestHistory_Remove(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(filepath.Join(dir, "h.json"))
	_ = store.Add(Record{VideoID: "x"})
	if !store.Has("x") {
		t.Fatal()
	}
	if err := store.Remove("x"); err != nil {
		t.Fatal(err)
	}
	if store.Has("x") {
		t.Error("Has after Remove should be false")
	}
}

func TestHistory_AddRequiresVideoID(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(filepath.Join(dir, "h.json"))
	if err := store.Add(Record{Title: "no id"}); err == nil {
		t.Error("expected error for empty VideoID")
	}
}

func TestHistory_CorruptFileStartsFresh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "h.json")
	if err := os.WriteFile(path, []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.List()) != 0 {
		t.Errorf("expected fresh store after corruption, got %d records", len(store.List()))
	}
}

func TestHistory_DownloadedAtPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "h.json")
	store, _ := NewStore(path)
	want := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	_ = store.Add(Record{VideoID: "x", DownloadedAt: want})

	store2, _ := NewStore(path)
	r, _ := store2.Get("x")
	if !r.DownloadedAt.Equal(want) {
		t.Errorf("DownloadedAt = %v, want %v", r.DownloadedAt, want)
	}
}
