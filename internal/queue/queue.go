package queue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"sort"
	"sync"
	"time"

	"github.com/StanislavStefanovPetrov/mmi-tunes/internal/downloader"
)

// Event is a state-change notification.
type Event struct {
	Kind EventKind `json:"kind"`
	Job  Job       `json:"job"`
}

type EventKind string

const (
	EventAdded    EventKind = "added"
	EventStatus   EventKind = "status"
	EventProgress EventKind = "progress"
	EventDone     EventKind = "done"
	EventError    EventKind = "error"
	EventRemoved  EventKind = "removed"
)

// Downloader is the function used to actually fetch a URL. Pluggable so
// the queue can be unit-tested with a stub.
type Downloader func(ctx context.Context, url string, settings downloader.Settings, onProgress func(downloader.Progress)) (*downloader.Result, error)

// Queue manages a concurrency-limited worker pool over a bounded set of
// Jobs. All public methods are safe for concurrent use.
type Queue struct {
	mu          sync.Mutex
	jobs        map[string]*jobState // by ID
	order       []string             // FIFO display order
	concurrency int

	work chan string // job IDs ready to run
	stop chan struct{}
	wg   sync.WaitGroup

	stopOnce sync.Once
	stopped  chan struct{} // closed once events channel is fully drained

	downloader Downloader
	settings   func() downloader.Settings // re-read each run, not snapshotted
	events     chan Event
}

// New constructs a Queue. settings is a closure so the caller can change
// global settings without restarting the queue.
func New(concurrency int, dl Downloader, settings func() downloader.Settings) *Queue {
	if concurrency < 1 {
		concurrency = 1
	}
	q := &Queue{
		jobs:        make(map[string]*jobState),
		concurrency: concurrency,
		work:        make(chan string, 1024),
		stop:        make(chan struct{}),
		stopped:     make(chan struct{}),
		downloader:  dl,
		settings:    settings,
		events:      make(chan Event, 256),
	}
	for i := 0; i < concurrency; i++ {
		q.wg.Add(1)
		go q.worker()
	}
	go q.eventCloser()
	return q
}

// Events returns the read end of the event channel. Consumers should drain
// it promptly; if the channel buffer (256) fills up, events are dropped.
func (q *Queue) Events() <-chan Event { return q.events }

// Stop signals all workers to finish current jobs and exit. Idempotent —
// calling twice is safe. Blocks until all workers exit and the events
// channel is fully drained/closed. emit() becomes a no-op after Stop.
//
// We deliberately do NOT close q.work here. StartAll() writes to q.work
// inside a `select { case <-q.stop: ; case q.work <- id: }`; if work were
// closed, both cases could be ready at once and Go's pseudo-random pick
// would panic on send-to-closed-channel. Instead, workers exit when
// q.stop closes (see worker()).
func (q *Queue) Stop() {
	q.stopOnce.Do(func() {
		close(q.stop)
		q.wg.Wait()
	})
	<-q.stopped
}

// eventCloser waits for all workers to finish, then closes events.
// Running this from a dedicated goroutine means emit() can race with Stop()
// safely: emit checks stop before sending, and the events channel is only
// closed after every worker has returned.
func (q *Queue) eventCloser() {
	q.wg.Wait()
	close(q.events)
	close(q.stopped)
}

// Add inserts a new job in StatusQueued without starting it. Returns the
// created Job snapshot. URL must already be canonicalized by the caller.
func (q *Queue) Add(url string) Job {
	id := newJobID()
	js := newJobState(Job{
		ID: id, URL: url, Status: StatusQueued, AddedAt: time.Now(),
	})

	q.mu.Lock()
	q.jobs[id] = js
	q.order = append(q.order, id)
	q.mu.Unlock()

	snap := js.snapshot()
	q.emit(Event{Kind: EventAdded, Job: snap})
	return snap
}

// AddCompleted inserts a job that's already finished — used when restoring
// persisted state from disk on app launch.
func (q *Queue) AddCompleted(j Job) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.jobs[j.ID] = newJobState(j)
	q.order = append(q.order, j.ID)
}

// Remove deletes a job. Cancels first if it's still running.
func (q *Queue) Remove(id string) bool {
	q.mu.Lock()
	js, ok := q.jobs[id]
	if !ok {
		q.mu.Unlock()
		return false
	}
	delete(q.jobs, id)
	for i, oid := range q.order {
		if oid == id {
			q.order = append(q.order[:i], q.order[i+1:]...)
			break
		}
	}
	q.mu.Unlock()

	// Capture cancel under js.mu, release before invoking — the cancel func
	// may synchronously trigger callbacks that re-take js.mu.
	js.mu.Lock()
	cancelFn := js.cancel
	js.cancel = nil
	js.mu.Unlock()
	if cancelFn != nil {
		cancelFn()
	}
	q.emit(Event{Kind: EventRemoved, Job: Job{ID: id}})
	return true
}

// StartJob queues a single job for execution. If the job is in error
// or cancelled status it is reset to queued first. Returns true if the
// job exists and was queued; false if the job doesn't exist or is
// already running/done.
func (q *Queue) StartJob(id string) bool {
	q.mu.Lock()
	js, ok := q.jobs[id]
	q.mu.Unlock()
	if !ok || js == nil {
		return false
	}
	s := js.status()
	if s == StatusRunning || s == StatusDone {
		return false
	}
	if s != StatusQueued {
		js.setStatus(StatusQueued)
		q.emit(Event{Kind: EventStatus, Job: js.snapshot()})
	}
	select {
	case <-q.stop:
		return false
	case q.work <- id:
		return true
	}
}

// StartAll queues every job currently in StatusQueued for execution.
func (q *Queue) StartAll() int {
	q.mu.Lock()
	ids := make([]string, 0, len(q.order))
	for _, id := range q.order {
		js := q.jobs[id]
		if js == nil {
			continue
		}
		s := js.status()
		if s == StatusQueued || s == StatusError || s == StatusCancelled {
			if s != StatusQueued {
				js.setStatus(StatusQueued)
				q.emit(Event{Kind: EventStatus, Job: js.snapshot()})
			}
			ids = append(ids, id)
		}
	}
	q.mu.Unlock()

	for _, id := range ids {
		select {
		case <-q.stop:
			return len(ids)
		case q.work <- id:
		}
	}
	return len(ids)
}

// Cancel a single in-flight job. Captures the cancel func under lock then
// invokes it after releasing the lock — the downloader's progress callback
// also takes js.mu, so calling cancel() while holding it would deadlock.
func (q *Queue) Cancel(id string) bool {
	q.mu.Lock()
	js, ok := q.jobs[id]
	q.mu.Unlock()
	if !ok {
		return false
	}

	js.mu.Lock()
	cancelFn := js.cancel
	if cancelFn != nil {
		// Running: invoke cancel after unlocking.
		js.cancel = nil
		js.mu.Unlock()
		cancelFn()
		return true
	}
	if js.data.Status == StatusQueued {
		js.data.Status = StatusCancelled
		now := time.Now()
		js.data.FinishedAt = &now
		js.mu.Unlock()
		q.emit(Event{Kind: EventStatus, Job: js.snapshot()})
		return true
	}
	js.mu.Unlock()
	return false
}

// CancelAll cancels everything. Returns count cancelled.
func (q *Queue) CancelAll() int {
	q.mu.Lock()
	ids := make([]string, 0, len(q.order))
	for _, id := range q.order {
		ids = append(ids, id)
	}
	q.mu.Unlock()
	n := 0
	for _, id := range ids {
		if q.Cancel(id) {
			n++
		}
	}
	return n
}

// List returns a snapshot of every job in display order.
func (q *Queue) List() []Job {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]Job, 0, len(q.order))
	for _, id := range q.order {
		if js, ok := q.jobs[id]; ok {
			out = append(out, js.snapshot())
		}
	}
	return out
}

// ClearAll cancels every running job and removes every job from the queue.
// Returns the number of jobs removed. Settings/history are left untouched —
// this is a "wipe the visible list" action, not a factory reset.
func (q *Queue) ClearAll() int {
	q.mu.Lock()
	ids := make([]string, len(q.order))
	copy(ids, q.order)
	q.mu.Unlock()
	for _, id := range ids {
		q.Remove(id)
	}
	return len(ids)
}

// ClearCompleted removes every Done/Cancelled job. Returns count removed.
func (q *Queue) ClearCompleted() int {
	q.mu.Lock()
	keep := make([]string, 0, len(q.order))
	removed := []string{}
	for _, id := range q.order {
		js := q.jobs[id]
		if js == nil {
			// Drift between order and jobs map — skip ghosts.
			continue
		}
		s := js.status()
		if s == StatusDone || s == StatusCancelled {
			delete(q.jobs, id)
			removed = append(removed, id)
			continue
		}
		keep = append(keep, id)
	}
	q.order = keep
	q.mu.Unlock()
	for _, id := range removed {
		q.emit(Event{Kind: EventRemoved, Job: Job{ID: id}})
	}
	return len(removed)
}

// Len reports current job count.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.order)
}

// Sort orders jobs by AddedAt; useful for stable display after restoration.
func (q *Queue) Sort() {
	q.mu.Lock()
	defer q.mu.Unlock()
	sort.SliceStable(q.order, func(i, k int) bool {
		ji, jk := q.jobs[q.order[i]], q.jobs[q.order[k]]
		if ji == nil || jk == nil {
			return false
		}
		return ji.addedAt().Before(jk.addedAt())
	})
}

// worker is one of `concurrency` goroutines pulling job IDs off q.work.
// Exits when q.stop is closed; q.work is never closed (see Stop()).
func (q *Queue) worker() {
	defer q.wg.Done()
	for {
		select {
		case <-q.stop:
			return
		case id := <-q.work:
			q.runJobSafe(id)
		}
	}
}

// runJobSafe wraps runJob with panic recovery. A bug in the downloader
// (or one of its dependencies) should kill the job, not the worker.
func (q *Queue) runJobSafe(id string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("queue: worker panic on job %s: %v\n%s", id, r, debug.Stack())
			q.mu.Lock()
			js := q.jobs[id]
			q.mu.Unlock()
			if js != nil {
				js.setError(downloader.ErrUnknown, fmt.Sprintf("internal panic: %v", r))
				js.setStatus(StatusError)
				q.emit(Event{Kind: EventError, Job: js.snapshot()})
			}
		}
	}()
	q.runJob(id)
}

func (q *Queue) runJob(id string) {
	q.mu.Lock()
	js, ok := q.jobs[id]
	q.mu.Unlock()
	if !ok {
		return
	}

	// Skip if cancelled while sitting in the queue.
	if js.status() == StatusCancelled {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	js.mu.Lock()
	js.cancel = cancel
	js.mu.Unlock()

	// Defer cancel-cleanup so a panic inside the downloader (caught by
	// runJobSafe's recover) still clears js.cancel. Without this a stale
	// CancelFunc would linger and Cancel() would return success on a
	// dead job.
	defer func() {
		js.mu.Lock()
		js.cancel = nil
		js.mu.Unlock()
	}()

	js.setStatus(StatusRunning)
	q.emit(Event{Kind: EventStatus, Job: js.snapshot()})

	res, err := q.downloader(ctx, js.data.URL, q.settings(), func(p downloader.Progress) {
		js.setProgress(p)
		q.emit(Event{Kind: EventProgress, Job: js.snapshot()})
	})

	if err != nil {
		var dlErr *downloader.Error
		if errors.As(err, &dlErr) && dlErr.Code == downloader.ErrCancelled {
			js.setStatus(StatusCancelled)
			q.emit(Event{Kind: EventStatus, Job: js.snapshot()})
			return
		}
		if errors.As(err, &dlErr) {
			js.setError(dlErr.Code, dlErr.Message)
		} else {
			js.setError(downloader.ErrUnknown, err.Error())
		}
		js.setStatus(StatusError)
		q.emit(Event{Kind: EventError, Job: js.snapshot()})
		return
	}
	js.setResult(res.VideoID, res.Title, res.OutputPath)
	js.setStatus(StatusDone)
	q.emit(Event{Kind: EventDone, Job: js.snapshot()})
}

// emit pushes an event without blocking. If Stop has been called or the
// channel buffer (256) is full, the event is dropped — UI will resync
// on next user action.
func (q *Queue) emit(ev Event) {
	select {
	case <-q.stop:
		return
	default:
	}
	select {
	case q.events <- ev:
	case <-q.stop:
	default:
	}
}

// newJobID returns a collision-resistant ID. UUIDv4-style, 16 random bytes
// hex-encoded — survives clock skew, NTP jumps, and process restarts.
func newJobID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure on modern macOS is impossibly rare; fall
		// back to time-based to keep the API total.
		return fmt.Sprintf("job-%d", time.Now().UnixNano())
	}
	return "job-" + hex.EncodeToString(b[:])
}
