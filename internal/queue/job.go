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

// Job is one URL waiting to be (or being / having been) processed.
// Mutated only under its own lock; the queue holds a pointer.
type Job struct {
	mu sync.RWMutex

	ID         string                 `json:"id"`
	URL        string                 `json:"url"`
	VideoID    string                 `json:"video_id,omitempty"`
	Title      string                 `json:"title,omitempty"`
	Status     Status                 `json:"status"`
	Progress   downloader.Progress    `json:"progress"`
	Error      string                 `json:"error,omitempty"`
	ErrorCode  downloader.ErrorCode   `json:"error_code,omitempty"`
	OutputPath string                 `json:"output_path,omitempty"`
	AddedAt    time.Time              `json:"added_at"`
	StartedAt  *time.Time             `json:"started_at,omitempty"`
	FinishedAt *time.Time             `json:"finished_at,omitempty"`

	cancel func()
}

// Snapshot returns a copy safe to ship over the Wails event bus or persist.
func (j *Job) Snapshot() Job {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return Job{
		ID: j.ID, URL: j.URL, VideoID: j.VideoID, Title: j.Title,
		Status: j.Status, Progress: j.Progress, Error: j.Error,
		ErrorCode: j.ErrorCode, OutputPath: j.OutputPath,
		AddedAt: j.AddedAt, StartedAt: j.StartedAt, FinishedAt: j.FinishedAt,
	}
}

func (j *Job) setStatus(s Status) {
	j.mu.Lock()
	j.Status = s
	now := time.Now()
	switch s {
	case StatusRunning:
		if j.StartedAt == nil {
			j.StartedAt = &now
		}
	case StatusDone, StatusError, StatusCancelled:
		j.FinishedAt = &now
	}
	j.mu.Unlock()
}

func (j *Job) setProgress(p downloader.Progress) {
	j.mu.Lock()
	j.Progress = p
	j.mu.Unlock()
}

func (j *Job) setError(code downloader.ErrorCode, msg string) {
	j.mu.Lock()
	j.ErrorCode = code
	j.Error = msg
	j.mu.Unlock()
}

func (j *Job) setResult(videoID, title, output string) {
	j.mu.Lock()
	j.VideoID = videoID
	if title != "" {
		j.Title = title
	}
	j.OutputPath = output
	j.mu.Unlock()
}
