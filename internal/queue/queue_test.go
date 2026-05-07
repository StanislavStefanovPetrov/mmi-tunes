package queue

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/StanislavStefanovPetrov/mmi-tunes/internal/downloader"
)

func defaultSettings() downloader.Settings {
	return downloader.MMIDefaults("/tmp/test")
}

func newWithStub(t *testing.T, concurrency int, dl Downloader) *Queue {
	t.Helper()
	q := New(concurrency, dl, defaultSettings)
	t.Cleanup(q.Stop)
	return q
}

// drainEvents collects events from q.Events() into a slice while the
// caller holds a wg; stops when ctx is cancelled.
func drainEvents(ctx context.Context, q *Queue) *eventSink {
	s := &eventSink{}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-q.Events():
				if !ok {
					return
				}
				s.add(ev)
			}
		}
	}()
	return s
}

type eventSink struct {
	mu     sync.Mutex
	events []Event
}

func (s *eventSink) add(ev Event) { s.mu.Lock(); s.events = append(s.events, ev); s.mu.Unlock() }
func (s *eventSink) snapshot() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Event, len(s.events))
	copy(out, s.events)
	return out
}

func TestQueue_RunsAllJobs(t *testing.T) {
	var ran int32
	dl := func(ctx context.Context, url string, _ downloader.Settings, cb func(downloader.Progress)) (*downloader.Result, error) {
		atomic.AddInt32(&ran, 1)
		cb(downloader.Progress{Stage: downloader.StageDownload, Percent: 50})
		return &downloader.Result{VideoID: "vid", Title: url, OutputPath: "/tmp/" + url + ".mp3"}, nil
	}
	q := newWithStub(t, 2, dl)
	for i := 0; i < 5; i++ {
		q.Add("url-" + string(rune('A'+i)))
	}
	if got := q.StartAll(); got != 5 {
		t.Fatalf("StartAll started %d, want 5", got)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&ran) == 5 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&ran); got != 5 {
		t.Fatalf("ran=%d, want 5", got)
	}
	for _, j := range q.List() {
		if j.Status != StatusDone {
			t.Errorf("job %s status=%s, want done", j.ID, j.Status)
		}
		if j.OutputPath == "" {
			t.Errorf("job %s missing OutputPath", j.ID)
		}
	}
}

func TestQueue_RespectsConcurrency(t *testing.T) {
	var inFlight, peakInFlight int32
	gate := make(chan struct{})
	dl := func(ctx context.Context, url string, _ downloader.Settings, _ func(downloader.Progress)) (*downloader.Result, error) {
		cur := atomic.AddInt32(&inFlight, 1)
		for {
			old := atomic.LoadInt32(&peakInFlight)
			if cur <= old || atomic.CompareAndSwapInt32(&peakInFlight, old, cur) {
				break
			}
		}
		<-gate
		atomic.AddInt32(&inFlight, -1)
		return &downloader.Result{VideoID: "v", Title: url, OutputPath: "/tmp/x.mp3"}, nil
	}
	q := newWithStub(t, 2, dl)
	for i := 0; i < 6; i++ {
		q.Add("u")
	}
	q.StartAll()

	// Wait for some workers to be running.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&peakInFlight) >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(gate)
	// Let things finish.
	time.Sleep(300 * time.Millisecond)
	if got := atomic.LoadInt32(&peakInFlight); got != 2 {
		t.Errorf("peak in flight = %d, want exactly 2 (concurrency)", got)
	}
}

func TestQueue_CancelMidFlight(t *testing.T) {
	dl := func(ctx context.Context, url string, _ downloader.Settings, _ func(downloader.Progress)) (*downloader.Result, error) {
		<-ctx.Done()
		return nil, &downloader.Error{Code: downloader.ErrCancelled, Message: "ctx cancelled"}
	}
	q := newWithStub(t, 1, dl)
	added := q.Add("u")
	q.StartAll()

	// Wait until job has started.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		jobs := q.List()
		if len(jobs) > 0 && jobs[0].Status == StatusRunning {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !q.Cancel(added.ID) {
		t.Fatal("Cancel returned false")
	}
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		jobs := q.List()
		if len(jobs) > 0 && jobs[0].Status == StatusCancelled {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("job did not transition to cancelled, last list: %+v", q.List())
}

func TestQueue_ErrorPropagates(t *testing.T) {
	dl := func(ctx context.Context, url string, _ downloader.Settings, _ func(downloader.Progress)) (*downloader.Result, error) {
		return nil, &downloader.Error{Code: downloader.ErrGeoBlocked, Message: "blocked"}
	}
	q := newWithStub(t, 1, dl)
	added := q.Add("u")
	q.StartAll()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		jobs := q.List()
		if len(jobs) > 0 && jobs[0].Status == StatusError {
			j := jobs[0]
			if j.ErrorCode != downloader.ErrGeoBlocked {
				t.Errorf("ErrorCode=%s, want %s", j.ErrorCode, downloader.ErrGeoBlocked)
			}
			if j.Error != "blocked" {
				t.Errorf("Error=%q, want %q", j.Error, "blocked")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("job did not enter error state, list=%+v", q.List())
	_ = added
}

func TestQueue_RemoveCancelsRunning(t *testing.T) {
	started := make(chan struct{})
	dl := func(ctx context.Context, url string, _ downloader.Settings, _ func(downloader.Progress)) (*downloader.Result, error) {
		close(started)
		<-ctx.Done()
		return nil, errors.New("cancelled")
	}
	q := newWithStub(t, 1, dl)
	j := q.Add("u")
	q.StartAll()
	<-started
	if !q.Remove(j.ID) {
		t.Fatal("Remove returned false")
	}
	if got := q.Len(); got != 0 {
		t.Errorf("Len=%d, want 0", got)
	}
}

func TestQueue_ClearCompleted(t *testing.T) {
	dl := func(ctx context.Context, url string, _ downloader.Settings, _ func(downloader.Progress)) (*downloader.Result, error) {
		return &downloader.Result{VideoID: "v", Title: url, OutputPath: "/tmp/x.mp3"}, nil
	}
	q := newWithStub(t, 2, dl)
	for i := 0; i < 3; i++ {
		q.Add("u")
	}
	q.StartAll()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		done := 0
		for _, j := range q.List() {
			if j.Status == StatusDone {
				done++
			}
		}
		if done == 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if removed := q.ClearCompleted(); removed != 3 {
		t.Errorf("ClearCompleted=%d, want 3", removed)
	}
	if got := q.Len(); got != 0 {
		t.Errorf("Len=%d, want 0", got)
	}
}
