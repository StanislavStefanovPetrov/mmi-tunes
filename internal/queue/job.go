// Package queue is a worker-pool download queue. Each Job represents one
// URL → MP3 conversion managed by the downloader package.
package queue

import (
	"sync"
	"time"

	"github.com/StanislavStefanovPetrov/mmi-tunes/internal/downloader"
)

// Status is the lifecycle stage of a Job.
type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusDone      Status = "done"
	StatusError     Status = "error"
	StatusCancelled Status = "cancelled"
)

// Job is the wire/persistence representation of a single URL → MP3 job.
// Pure data, no mutex — safe to copy, marshal, ship over Wails events,
// or write to jobs.json. The live, mutating instance lives inside the
// queue as *jobState; Snapshot() converts state → Job.
type Job struct {
	ID          string               `json:"id"`
	URL         string               `json:"url"`
	VideoID     string               `json:"video_id,omitempty"`
	Title       string               `json:"title,omitempty"`
	Status      Status               `json:"status"`
	Progress    downloader.Progress  `json:"progress"`
	Error       string               `json:"error,omitempty"`
	ErrorCode   downloader.ErrorCode `json:"error_code,omitempty"`
	ErrorDetail string               `json:"error_detail,omitempty"`
	OutputPath  string               `json:"output_path,omitempty"`
	AddedAt     time.Time            `json:"added_at"`
	StartedAt   *time.Time           `json:"started_at,omitempty"`
	FinishedAt  *time.Time           `json:"finished_at,omitempty"`
}

// jobState wraps a Job with the mutex and cancel func used by the queue
// internally. It is never returned by value and never marshaled.
type jobState struct {
	mu     sync.RWMutex
	data   Job
	cancel func()
}

func newJobState(j Job) *jobState { return &jobState{data: j} }

// snapshot returns a value copy of the data fields, safe to ship over
// the wire or persist. Does not include the mutex or cancel func.
func (s *jobState) snapshot() Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

func (s *jobState) setStatus(st Status) {
	s.mu.Lock()
	s.data.Status = st
	now := time.Now()
	switch st {
	case StatusRunning:
		if s.data.StartedAt == nil {
			s.data.StartedAt = &now
		}
	case StatusDone, StatusError, StatusCancelled:
		s.data.FinishedAt = &now
	}
	s.mu.Unlock()
}

func (s *jobState) setProgress(p downloader.Progress) {
	s.mu.Lock()
	s.data.Progress = p
	s.mu.Unlock()
}

// maxErrorDetailBytes caps the raw stderr we keep per job. Jobs are
// persisted to jobs.json on every change, and verbose yt-dlp output would
// otherwise bloat the file without bound.
const maxErrorDetailBytes = 8 << 10 // 8 KiB

func (s *jobState) setError(code downloader.ErrorCode, msg, detail string) {
	s.mu.Lock()
	s.data.ErrorCode = code
	s.data.Error = msg
	s.data.ErrorDetail = truncateHead(detail, maxErrorDetailBytes)
	s.mu.Unlock()
}

// truncateHead keeps the LAST max bytes of v — yt-dlp reports the actual
// failure at the end of stderr, so the tail is the useful part.
func truncateHead(v string, max int) string {
	if len(v) <= max {
		return v
	}
	return "…(truncated)\n" + v[len(v)-max:]
}

func (s *jobState) setResult(videoID, title, output string) {
	s.mu.Lock()
	s.data.VideoID = videoID
	if title != "" {
		s.data.Title = title
	}
	s.data.OutputPath = output
	s.mu.Unlock()
}

func (s *jobState) status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Status
}

func (s *jobState) addedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.AddedAt
}
