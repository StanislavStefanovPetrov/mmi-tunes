package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/StanislavStefanovPetrov/mmi-tunes/internal/downloader"
	"github.com/StanislavStefanovPetrov/mmi-tunes/internal/history"
	"github.com/StanislavStefanovPetrov/mmi-tunes/internal/paths"
	"github.com/StanislavStefanovPetrov/mmi-tunes/internal/queue"
	"github.com/StanislavStefanovPetrov/mmi-tunes/internal/settings"
	"github.com/StanislavStefanovPetrov/mmi-tunes/internal/tools"
	urlpkg "github.com/StanislavStefanovPetrov/mmi-tunes/internal/url"
)

// App is the Go-side struct exposed to the React frontend through Wails.
// All exported (capitalized) methods become async functions in TypeScript.
type App struct {
	ctx context.Context

	settings *settings.Store
	history  *history.Store
	queue    *queue.Queue

	jobsPath string
}

func NewApp() *App { return &App{} }

// startup wires up persistence, the queue, and the event-emitting goroutine.
// Called once by Wails after the WebView has initialised.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	defaultDownload, err := paths.DefaultDownloadDir()
	if err != nil {
		log.Printf("default download dir: %v", err)
		defaultDownload = "."
	}
	settingsPath, _ := paths.SettingsPath()
	historyPath, _ := paths.HistoryPath()
	a.jobsPath, _ = paths.JobsPath()

	store, err := settings.NewStore(settingsPath, settings.Defaults(defaultDownload))
	if err != nil {
		log.Fatalf("settings: %v", err)
	}
	a.settings = store

	hist, err := history.NewStore(historyPath)
	if err != nil {
		log.Fatalf("history: %v", err)
	}
	a.history = hist

	a.queue = queue.New(
		store.Get().Concurrency,
		downloader.Download,
		func() downloader.Settings {
			s := store.Get()
			return downloader.Settings{
				DownloadFolder: s.DownloadFolder,
				Bitrate:        s.Bitrate,
				SampleRate:     s.SampleRate,
				Channels:       s.Channels,
				EmbedMetadata:  s.EmbedMetadata,
				EmbedThumbnail: s.EmbedThumbnail,
				ThumbnailMaxPx: s.ThumbnailMaxPx,
				Transliterate:  s.Transliterate,
			}
		},
	)
	if err := a.queue.Load(a.jobsPath); err != nil {
		log.Printf("queue load: %v", err)
	}

	go a.eventLoop()
}

// shutdown is called by Wails right before the app exits — last chance
// to persist state.
func (a *App) shutdown(ctx context.Context) {
	if a.queue != nil {
		_ = a.queue.Save(a.jobsPath)
		a.queue.Stop()
	}
}

// eventLoop bridges queue events to the Wails event system. The frontend
// subscribes via EventsOn("job:added"|"job:status"|...).
func (a *App) eventLoop() {
	for ev := range a.queue.Events() {
		var name string
		switch ev.Kind {
		case queue.EventAdded:
			name = "job:added"
		case queue.EventStatus:
			name = "job:status"
		case queue.EventProgress:
			name = "job:progress"
		case queue.EventDone:
			a.recordHistory(ev.Job)
			name = "job:done"
		case queue.EventError:
			name = "job:error"
		case queue.EventRemoved:
			name = "job:removed"
		default:
			continue
		}
		wruntime.EventsEmit(a.ctx, name, ev.Job)

		// Persist on every state change. Cheap (small JSON, debounce TBD).
		if a.jobsPath != "" {
			_ = a.queue.Save(a.jobsPath)
		}
	}
}

func (a *App) recordHistory(j queue.Job) {
	if j.VideoID == "" {
		return
	}
	_ = a.history.Add(history.Record{
		VideoID:    j.VideoID,
		Title:      j.Title,
		OutputPath: j.OutputPath,
	})
}

// --------------------------------------------------------------------
// Methods exposed to the React frontend.
// --------------------------------------------------------------------

// AddURL validates, dedups (per settings), and queues a URL.
func (a *App) AddURL(rawURL string) (queue.Job, error) {
	canonical, err := urlpkg.Canonical(rawURL)
	if err != nil {
		return queue.Job{}, fmt.Errorf("invalid URL: %w", err)
	}
	id, _ := urlpkg.ExtractVideoID(rawURL)

	if a.settings.Get().DedupHistory && a.history.Has(id) {
		// Caller still gets a Job back so the UI can show "already downloaded".
		// Returning a typed error lets the frontend offer a "download anyway" option.
		return queue.Job{}, fmt.Errorf("already downloaded: %s", id)
	}
	return a.queue.Add(canonical), nil
}

// AddURLForce queues even if the video is already in history (used when
// the user clicks "download anyway" past the dedup warning).
func (a *App) AddURLForce(rawURL string) (queue.Job, error) {
	canonical, err := urlpkg.Canonical(rawURL)
	if err != nil {
		return queue.Job{}, fmt.Errorf("invalid URL: %w", err)
	}
	return a.queue.Add(canonical), nil
}

// IsAlreadyDownloaded probes history without adding.
func (a *App) IsAlreadyDownloaded(rawURL string) (bool, error) {
	id, err := urlpkg.ExtractVideoID(rawURL)
	if err != nil {
		return false, err
	}
	return a.history.Has(id), nil
}

// RemoveJob deletes a job from the queue (cancels first if running).
func (a *App) RemoveJob(id string) bool { return a.queue.Remove(id) }

// ListJobs returns all current jobs in display order.
func (a *App) ListJobs() []queue.Job { return a.queue.List() }

// ClearCompleted removes all done/cancelled jobs from the visible list.
func (a *App) ClearCompleted() int { return a.queue.ClearCompleted() }

// StartAll queues every queued/error/cancelled job for execution.
func (a *App) StartAll() int { return a.queue.StartAll() }

// CancelJob cancels a single job.
func (a *App) CancelJob(id string) bool { return a.queue.Cancel(id) }

// CancelAll cancels every running/queued job.
func (a *App) CancelAll() int { return a.queue.CancelAll() }

// GetSettings returns the current persisted settings.
func (a *App) GetSettings() settings.Settings { return a.settings.Get() }

// SaveSettings validates and persists new settings.
func (a *App) SaveSettings(s settings.Settings) error { return a.settings.Save(s) }

// PickFolder opens a native macOS directory chooser.
func (a *App) PickFolder() (string, error) {
	return wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "Choose download folder",
	})
}

// RevealInFinder opens the parent folder of path in Finder, selecting
// the file if possible.
func (a *App) RevealInFinder(path string) error {
	if path == "" {
		return errors.New("empty path")
	}
	if runtime.GOOS != "darwin" {
		return errors.New("RevealInFinder only supported on macOS")
	}
	return exec.Command("open", "-R", path).Run()
}

// CountFilesInFolder returns the number of regular files directly inside
// folder (non-recursive). Used to flag the Audi MMI 5000-files-per-dir limit.
func (a *App) CountFilesInFolder(folder string) (int, error) {
	if folder == "" {
		return 0, nil
	}
	abs, _ := filepath.Abs(folder)
	entries, err := readDir(abs)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			n++
		}
	}
	return n, nil
}

// CheckTools probes yt-dlp and ffmpeg.
func (a *App) CheckTools() tools.AllStatus { return tools.CheckAll() }

// UpdateYtDlp runs `yt-dlp -U`. Returns combined stdout+stderr.
func (a *App) UpdateYtDlp() (string, error) {
	out, err := exec.CommandContext(a.ctx, "yt-dlp", "-U").CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), err
	}
	return strings.TrimSpace(string(out)), nil
}

// GetClipboardURL returns the clipboard contents if and only if it parses
// as a YouTube URL. Empty string means "no, but no error".
func (a *App) GetClipboardURL() (string, error) {
	text, err := wruntime.ClipboardGetText(a.ctx)
	if err != nil {
		return "", err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	if !urlpkg.IsYouTubeURL(text) {
		return "", nil
	}
	canonical, _ := urlpkg.Canonical(text)
	return canonical, nil
}
