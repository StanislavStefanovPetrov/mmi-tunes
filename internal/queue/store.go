package queue

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// PersistableState is the queue snapshot written to disk.
type PersistableState struct {
	Jobs []Job `json:"jobs"`
}

// Save writes the current job list to path. Running jobs are downgraded to
// queued — workers will pick them up again on next launch.
func (q *Queue) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	jobs := q.List()
	for i := range jobs {
		if jobs[i].Status == StatusRunning {
			jobs[i].Status = StatusQueued
			jobs[i].Progress.Stage = ""
			jobs[i].Progress.Percent = 0
		}
	}
	b, err := json.MarshalIndent(PersistableState{Jobs: jobs}, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Load restores jobs from path into q. Missing file is not an error.
func (q *Queue) Load(path string) error {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var state PersistableState
	if err := json.Unmarshal(b, &state); err != nil {
		_ = os.Rename(path, path+".bak")
		return nil
	}
	for _, j := range state.Jobs {
		q.AddCompleted(j)
	}
	q.Sort()
	return nil
}
