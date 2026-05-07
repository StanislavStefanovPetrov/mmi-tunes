package queue

import (
	"context"
	"errors"
	"fmt"
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

	downloader Downloader
	settings   func() downloader.Settings // re-read each run, not snapshotted
	events     chan Event

	idCounter int
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
		downloader:  dl,
		settings:    settings,
		events:      make(chan Event, 256),
	}
	for i := 0; i < concurrency; i++ {
		q.wg.Add(1)
		go q.worker()
	}
	return q
}

// Events returns the read end of the event channel. Consumers should drain
// it promptly; if the channel buffer (256) fills up, events are dropped.
func (q *Queue) Events() <-chan Event { return q.events }

// Stop signals all workers to finish current jobs and exit. Blocks until
// done. Future Add() / StartAll() calls become no-ops after Stop.
func (q *Queue) Stop() {
	close(q.stop)
	close(q.work)
	q.wg.Wait()
	close(q.events)
}

// Add inserts a new job in StatusQueued without starting it. Returns the
// created Job snapshot. URL must already be canonicalized by the caller.
func (q *Queue) Add(url string) Job {
	q.mu.Lock()
	q.idCounter++
	id := fmt.Sprintf("job-%d-%d", time.Now().UnixNano(), q.idCounter)
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
	if j.cancel != nil {
		j.cancel()
	}
	delete(q.jobs, id)
	for i, oid := range q.order {
		if oid == id {
			q.order = append(q.order[:i], q.order[i+1:]...)
			break
		}
	}
	q.mu.Unlock()
	q.emit(Event{Kind: EventRemoved, Job: Job{ID: id}})
	return true
}

// StartAll queues every job currently in StatusQueued for execution.
func (q *Queue) StartAll() int {
	q.mu.Lock()
	ids := make([]string, 0, len(q.order))
	for _, id := range q.order {
		j := q.jobs[id]
		if j != nil {
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

// Cancel a single in-flight job.
func (q *Queue) Cancel(id string) bool {
	q.mu.Lock()
	j, ok := q.jobs[id]
	q.mu.Unlock()
	if !ok {
		return false
	}
	j.mu.Lock()
	if j.cancel != nil {
		j.cancel()
		j.mu.Unlock()
		return true
	}
	// Not running yet — flip to cancelled directly.
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
	keep := q.order[:0]
	removed := []string{}
	for _, id := range q.order {
		j := q.jobs[id]
		s := j.Status
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
		return q.jobs[q.order[i]].AddedAt.Before(q.jobs[q.order[k]].AddedAt)
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
		q.runJob(id)
	}
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
	if j.Status == StatusCancelled {
		j.mu.RUnlock()
		return
	}
	j.mu.RUnlock()

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

// emit pushes an event without blocking. If the buffer is full, the event
// is dropped — UI will resync on next user action.
func (q *Queue) emit(ev Event) {
	select {
	case q.events <- ev:
	default:
	}
}
