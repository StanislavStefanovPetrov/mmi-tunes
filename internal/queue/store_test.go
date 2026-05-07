package queue

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/StanislavStefanovPetrov/mmi-tunes/internal/downloader"
)

func noopDownloader(ctx context.Context, url string, _ downloader.Settings, _ func(downloader.Progress)) (*downloader.Result, error) {
	return &downloader.Result{VideoID: "v", Title: url, OutputPath: "/tmp/x.mp3"}, nil
}

func TestQueue_SaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.json")

	q1 := New(2, noopDownloader, func() downloader.Settings { return downloader.MMIDefaults("/tmp") })
	defer q1.Stop()

	q1.Add("https://www.youtube.com/watch?v=aaaaaaaaaaa")
	q1.Add("https://www.youtube.com/watch?v=bbbbbbbbbbb")
	q1.StartAll()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		done := 0
		for _, j := range q1.List() {
			if j.Status == StatusDone {
				done++
			}
		}
		if done == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := q1.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("jobs.json not created: %v", err)
	}

	q2 := New(2, noopDownloader, func() downloader.Settings { return downloader.MMIDefaults("/tmp") })
	defer q2.Stop()
	if err := q2.Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	jobs := q2.List()
	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs after reload, got %d", len(jobs))
	}
	for _, j := range jobs {
		if j.Status != StatusDone {
			t.Errorf("status not preserved: %+v", j)
		}
		if j.OutputPath == "" || j.Title == "" {
			t.Errorf("result fields lost on reload: %+v", j)
		}
	}
}

func TestQueue_LoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	q := New(1, noopDownloader, func() downloader.Settings { return downloader.MMIDefaults("/tmp") })
	defer q.Stop()
	if err := q.Load(filepath.Join(dir, "does-not-exist.json")); err != nil {
		t.Errorf("Load on missing file should be no-op, got: %v", err)
	}
	if q.Len() != 0 {
		t.Errorf("expected empty queue, got %d", q.Len())
	}
}

func TestQueue_LoadCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.json")
	if err := os.WriteFile(path, []byte("{ totally not valid }"), 0o644); err != nil {
		t.Fatal(err)
	}

	q := New(1, noopDownloader, func() downloader.Settings { return downloader.MMIDefaults("/tmp") })
	defer q.Stop()
	if err := q.Load(path); err != nil {
		t.Errorf("Load on corrupt file should not return an error, got: %v", err)
	}
	if q.Len() != 0 {
		t.Error("corrupt file should yield empty queue")
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Errorf("expected .bak to be created from corrupt jobs.json: %v", err)
	}
}

func TestQueue_SaveDowngradesRunningToQueued(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.json")

	// Start a long-running job we never finish.
	gate := make(chan struct{})
	dl := func(ctx context.Context, url string, _ downloader.Settings, _ func(downloader.Progress)) (*downloader.Result, error) {
		<-gate
		return nil, nil
	}
	q := New(1, dl, func() downloader.Settings { return downloader.MMIDefaults("/tmp") })
	q.Add("u")
	q.StartAll()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		jobs := q.List()
		if len(jobs) > 0 && jobs[0].Status == StatusRunning {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := q.Save(path); err != nil {
		t.Fatal(err)
	}
	close(gate) // let the worker finish so Stop returns
	q.Stop()

	q2 := New(1, noopDownloader, func() downloader.Settings { return downloader.MMIDefaults("/tmp") })
	defer q2.Stop()
	if err := q2.Load(path); err != nil {
		t.Fatal(err)
	}
	jobs := q2.List()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Status != StatusQueued {
		t.Errorf("running → queued downgrade failed: %s", jobs[0].Status)
	}
}

func TestQueue_StopIdempotent(t *testing.T) {
	q := New(1, noopDownloader, func() downloader.Settings { return downloader.MMIDefaults("/tmp") })
	q.Stop()
	q.Stop() // second call must not panic
	q.Stop() // third either
}

func TestQueue_PanicInDownloaderRecovers(t *testing.T) {
	dl := func(ctx context.Context, url string, _ downloader.Settings, _ func(downloader.Progress)) (*downloader.Result, error) {
		panic("simulated downloader bug")
	}
	q := New(1, dl, func() downloader.Settings { return downloader.MMIDefaults("/tmp") })
	defer q.Stop()
	added := q.Add("u")
	q.StartAll()

	// Wait for the error event from the queue's event channel rather than
	// time-based polling — eliminates flakiness on a loaded CI box.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case ev, ok := <-q.Events():
			if !ok {
				t.Fatal("events channel closed before error event")
			}
			if ev.Kind == EventError && ev.Job.ID == added.ID {
				goto recovered
			}
		case <-deadline:
			t.Fatalf("worker did not recover from panic, jobs=%+v", q.List())
		}
	}
recovered:
	// And the cancel func must have been cleared by the deferred
	// cleanup, even though the downloader panicked.
	jobs := q.List()
	if len(jobs) != 1 || jobs[0].Status != StatusError {
		t.Errorf("expected exactly 1 job in StatusError, got %+v", jobs)
	}
	// Cancel on a dead job should now return false (no live cancel func).
	if q.Cancel(added.ID) {
		t.Error("Cancel on errored job returned true; stale cancel func leaked")
	}
}

// TestQueue_StartAllRacesStop reproduces the send-on-closed-channel scenario
// the second reviewer found. Without the worker-loop fix, this panics
// reliably under -race when StartAll fires concurrently with Stop.
func TestQueue_StartAllRacesStop(t *testing.T) {
	for trial := 0; trial < 50; trial++ {
		dl := func(ctx context.Context, url string, _ downloader.Settings, _ func(downloader.Progress)) (*downloader.Result, error) {
			return &downloader.Result{VideoID: "v", Title: url, OutputPath: "/tmp/x.mp3"}, nil
		}
		q := New(2, dl, func() downloader.Settings { return downloader.MMIDefaults("/tmp") })
		for i := 0; i < 20; i++ {
			q.Add("u")
		}
		// Race the producer against the stopper.
		done := make(chan struct{})
		go func() {
			q.StartAll()
			close(done)
		}()
		q.Stop()
		<-done
	}
}

// TestQueue_EmitDuringStop fires events from the same goroutine while
// Stop is winding down. With the eventCloser pattern emit() simply drops
// events after stop closes — it must not panic.
func TestQueue_EmitDuringStop(t *testing.T) {
	released := make(chan struct{})
	dl := func(ctx context.Context, url string, _ downloader.Settings, cb func(downloader.Progress)) (*downloader.Result, error) {
		<-released
		// One last event after stop has been signalled.
		cb(downloader.Progress{Stage: downloader.StageDownload, Percent: 99})
		return &downloader.Result{VideoID: "v", Title: url, OutputPath: "/tmp/x.mp3"}, nil
	}
	q := New(2, dl, func() downloader.Settings { return downloader.MMIDefaults("/tmp") })
	for i := 0; i < 5; i++ {
		q.Add("u")
	}
	q.StartAll()

	go func() { time.Sleep(50 * time.Millisecond); close(released) }()
	q.Stop()
	// If we got here without panicking, emit() correctly handled stop.
}
