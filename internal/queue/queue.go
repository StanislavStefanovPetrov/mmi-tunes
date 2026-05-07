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
	jobs        map[string]*Job // by ID
	order       []string        // FIFO display order
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
		jobs:        make(map[string]*Job),
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
func (q *Queue) Stop() {
	q.stopOnce.Do(func() {
		close(q.stop)
		close(q.work)
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
	q.mu.Lock()
	id := newJobID()
	j := &Job{
		ID: id, URL: url, Status: StatusQueued, AddedAt: time.Now(),
	}
	q.jobs[id] = j
	q.order = append(q.order, id)
	q.mu.Unlock()

	q.emit(Event{Kind: EventAdded, Job: j.Snapshot()})
	return j.Snapshot()
}

// AddCompleted inserts a job that's already finished — used when restoring
// persisted state from disk on app launch.
func (q *Queue) AddCompleted(j Job) {
	q.mu.Lock()
	defer q.mu.Unlock()
	jp := &Job{
		ID: j.ID, URL: j.URL, VideoID: j.VideoID, Title: j.Title,
		Status: j.Status, Progress: j.Progress, Error: j.Error,
		ErrorCode: j.ErrorCode, OutputPath: j.OutputPath,
		AddedAt: j.AddedAt, StartedAt: j.StartedAt, FinishedAt: j.FinishedAt,
	}
	q.jobs[j.ID] = jp
	q.order = append(q.order, j.ID)
}

// Remove deletes a job. Cancels first if it's still running.
func (q *Queue) Remove(id string) bool {
	q.mu.Lock()
	j, ok := q.jobs[id]
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

	// Capture cancel under j.mu, release before invoking — the cancel func
	// may synchronously trigger callbacks that re-take j.mu.
	j.mu.Lock()
	cancelFn := j.cancel
	j.cancel = nil
	j.mu.Unlock()
	if cancelFn != nil {
		cancelFn()
	}
	q.emit(Event{Kind: EventRemoved, Job: Job{ID: id}})
	return true
}

// StartAll queues every job currently in StatusQueued for execution.
func (q *Queue) StartAll() int {
	q.mu.Lock()
	ids := make([]string, 0, len(q.order))
	for _, id := range q.order {
		j := q.jobs[id]
		if j == nil {
			continue
		}
		j.mu.RLock()
		s := j.Status
		j.mu.RUnlock()
		if s == StatusQueued || s == StatusError || s == StatusCancelled {
			if s != StatusQueued {
				j.setStatus(StatusQueued)
				q.emit(Event{Kind: EventStatus, Job: j.Snapshot()})
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
// also takes j.mu, so calling cancel() while holding it would deadlock.
func (q *Queue) Cancel(id string) bool {
	q.mu.Lock()
	j, ok := q.jobs[id]
	q.mu.Unlock()
	if !ok {
		return false
	}

	j.mu.Lock()
	cancelFn := j.cancel
	if cancelFn != nil {
		// Running: invoke cancel after unlocking.
		j.cancel = nil
		j.mu.Unlock()
		cancelFn()
		return true
	}
	if j.Status == StatusQueued {
		j.Status = StatusCancelled
		now := time.Now()
		j.FinishedAt = &now
		j.mu.Unlock()
		q.emit(Event{Kind: EventStatus, Job: j.Snapshot()})
		return true
	}
	j.mu.Unlock()
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
		if j, ok := q.jobs[id]; ok {
			out = append(out, j.Snapshot())
		}
	}
	return out
}

// ClearCompleted removes every Done/Cancelled job. Returns count removed.
func (q *Queue) ClearCompleted() int {
	q.mu.Lock()
	keep := make([]string, 0, len(q.order))
	removed := []string{}
	for _, id := range q.order {
		j := q.jobs[id]
		if j == nil {
			// Drift between order and jobs map — skip ghosts.
			continue
		}
		j.mu.RLock()
		s := j.Status
		j.mu.RUnlock()
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
		return ji.AddedAt.Before(jk.AddedAt)
	})
}

// worker is one of `concurrency` goroutines pulling job IDs off q.work.
func (q *Queue) worker() {
	defer q.wg.Done()
	for id := range q.work {
		select {
		case <-q.stop:
			return
		default:
		}
		q.runJobSafe(id)
	}
}

// runJobSafe wraps runJob with panic recovery. A bug in the downloader
// (or one of its dependencies) should kill the job, not the worker.
func (q *Queue) runJobSafe(id string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("queue: worker panic on job %s: %v\n%s", id, r, debug.Stack())
			q.mu.Lock()
			j := q.jobs[id]
			q.mu.Unlock()
			if j != nil {
				j.setError(downloader.ErrUnknown, fmt.Sprintf("internal panic: %v", r))
				j.setStatus(StatusError)
				q.emit(Event{Kind: EventError, Job: j.Snapshot()})
			}
		}
	}()
	q.runJob(id)
}

func (q *Queue) runJob(id string) {
	q.mu.Lock()
	j, ok := q.jobs[id]
	q.mu.Unlock()
	if !ok {
		return
	}

	// Skip if cancelled while sitting in the queue.
	j.mu.RLock()
	cancelledEarly := j.Status == StatusCancelled
	j.mu.RUnlock()
	if cancelledEarly {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	j.mu.Lock()
	j.cancel = cancel
	j.mu.Unlock()

	j.setStatus(StatusRunning)
	q.emit(Event{Kind: EventStatus, Job: j.Snapshot()})

	res, err := q.downloader(ctx, j.URL, q.settings(), func(p downloader.Progress) {
		j.setProgress(p)
		q.emit(Event{Kind: EventProgress, Job: j.Snapshot()})
	})

	j.mu.Lock()
	j.cancel = nil
	j.mu.Unlock()

	if err != nil {
		var dlErr *downloader.Error
		if errors.As(err, &dlErr) && dlErr.Code == downloader.ErrCancelled {
			j.setStatus(StatusCancelled)
			q.emit(Event{Kind: EventStatus, Job: j.Snapshot()})
			return
		}
		if errors.As(err, &dlErr) {
			j.setError(dlErr.Code, dlErr.Message)
		} else {
			j.setError(downloader.ErrUnknown, err.Error())
		}
		j.setStatus(StatusError)
		q.emit(Event{Kind: EventError, Job: j.Snapshot()})
		return
	}
	j.setResult(res.VideoID, res.Title, res.OutputPath)
	j.setStatus(StatusDone)
	q.emit(Event{Kind: EventDone, Job: j.Snapshot()})
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
