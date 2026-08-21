// Package downloader runs yt-dlp end-to-end for one URL and produces a
// fully Audi MMI–compliant MP3 (320 kbps / 48 kHz / stereo / embedded
// metadata + thumbnail resized to 800×800).
package downloader

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/StanislavStefanovPetrov/mmi-tunes/internal/postprocess"
	"github.com/StanislavStefanovPetrov/mmi-tunes/internal/tools"
	urlpkg "github.com/StanislavStefanovPetrov/mmi-tunes/internal/url"
)

// Settings controls a single Download() call.
type Settings struct {
	DownloadFolder string // e.g. ~/Music/MMI Tunes
	Bitrate        int    // kbps,  Audi MMI ≤ 320
	SampleRate     int    // Hz,    Audi MMI ≤ 48000
	Channels       int    // 1 or 2
	EmbedMetadata  bool
	EmbedThumbnail bool
	ThumbnailMaxPx int  // resize embedded thumbnail to this max dimension
	Transliterate  bool // (currently always on; reserved for future toggle)
	Verbose        bool // pass -v to yt-dlp and tee stderr to the log file
}

// MMIDefaults returns the recommended settings for Audi MMI 3G+ output.
func MMIDefaults(downloadFolder string) Settings {
	return Settings{
		DownloadFolder: downloadFolder,
		Bitrate:        320,
		SampleRate:     48000,
		Channels:       2,
		EmbedMetadata:  true,
		EmbedThumbnail: true,
		ThumbnailMaxPx: postprocess.MaxCoverPx,
		Transliterate:  true,
	}
}

// Result describes a successful download.
type Result struct {
	VideoID    string `json:"video_id"`
	Title      string `json:"title"`
	OutputPath string `json:"output_path"`
}

// Metadata is the subset of yt-dlp --print-json fields we read.
type Metadata struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Uploader string `json:"uploader"`
	Duration int    `json:"duration"`
}

// Error categorises a download failure.
type Error struct {
	Code    ErrorCode
	Message string
	Stderr  string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// jsRuntimeArgs returns the flags that point yt-dlp at our bundled
// QuickJS binary, or nil if it cannot be found.
//
// YouTube requires solving a JavaScript "n signature" challenge before it
// will return adaptive audio formats. yt-dlp delegates that to an external
// JS engine; with none available it falls back to the visionos client,
// which answers UNPLAYABLE — surfacing as "This video is not available"
// even for perfectly public videos.
//
// We do not pass --no-js-runtimes first. yt-dlp prefers deno > node >
// quickjs > bun and picks the highest-priority runtime that is both
// enabled and available, so leaving deno enabled lets a user who has one
// take the faster path while our bundled qjs remains the guaranteed floor.
func jsRuntimeArgs() []string {
	qjs, err := tools.Locate("qjs")
	if err != nil {
		return nil
	}
	return []string{"--js-runtimes", "quickjs:" + qjs}
}

// maxLogBytes caps the verbose log so a long session cannot fill the disk.
// The file is truncated rather than rotated — it is a debugging aid, not an
// audit trail.
const maxLogBytes = 5 << 20 // 5 MiB

// LogPath returns the verbose log location, creating its directory.
// Also used by App.OpenLog to reveal the file in Finder.
func LogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Library", "Logs", "MMI Tunes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "mmi-tunes.log"), nil
}

// openVerboseLog opens the log for appending, truncating it first if it has
// grown past maxLogBytes. Returns nil (not an error) when logging cannot be
// set up — verbose logging is a diagnostic nicety and must never fail a
// download.
func openVerboseLog(url string) *os.File {
	path, err := LogPath()
	if err != nil {
		log.Printf("downloader: verbose log path: %v", err)
		return nil
	}
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if info, err := os.Stat(path); err == nil && info.Size() > maxLogBytes {
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}
	f, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		log.Printf("downloader: open verbose log: %v", err)
		return nil
	}
	fmt.Fprintf(f, "\n===== %s  %s =====\n", time.Now().Format(time.RFC3339), url)
	return f
}

// Download runs yt-dlp end-to-end for url, streaming progress to
// onProgress (which may be nil). Returns an error of type *Error on
// known failure modes.
func Download(ctx context.Context, url string, s Settings, onProgress func(Progress)) (*Result, error) {
	if onProgress == nil {
		onProgress = func(Progress) {}
	}

	canonical, err := urlpkg.Canonical(url)
	if err != nil {
		return nil, &Error{Code: ErrUnknown, Message: err.Error()}
	}

	if s.DownloadFolder == "" {
		return nil, &Error{Code: ErrFolderMissing, Message: "Download folder is not set."}
	}
	if err := os.MkdirAll(s.DownloadFolder, 0o755); err != nil {
		return nil, &Error{Code: ErrFolderMissing, Message: "Cannot create download folder: " + err.Error()}
	}

	onProgress(Progress{Stage: StageMetadata, Message: "Fetching video metadata…"})
	meta, err := fetchMetadata(ctx, canonical, s.Verbose)
	if err != nil {
		return nil, err
	}

	sanitized := postprocess.SanitizeFilename(meta.Title, meta.ID)

	// yt-dlp expands %(ext)s to the final extension after post-processing.
	outputTemplate := filepath.Join(s.DownloadFolder, sanitized+".%(ext)s")
	expectedPath := filepath.Join(s.DownloadFolder, sanitized+".mp3")

	args := buildYtDlpArgs(canonical, outputTemplate, s)

	ytdlpPath, err := tools.Locate("yt-dlp")
	if err != nil {
		return nil, &Error{Code: ErrUnknown, Message: "yt-dlp not found: " + err.Error()}
	}
	// When the bundled yt-dlp invokes ffmpeg we want it to find our
	// bundled ffmpeg, not whatever (if anything) is on PATH. Pass the
	// directory via --ffmpeg-location so yt-dlp picks it up explicitly.
	if ffmpegPath, err := tools.Locate("ffmpeg"); err == nil {
		args = append([]string{"--ffmpeg-location", filepath.Dir(ffmpegPath)}, args...)
	}
	args = append(jsRuntimeArgs(), args...)

	cmd := exec.CommandContext(ctx, ytdlpPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, &Error{Code: ErrUnknown, Message: err.Error()}
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	if s.Verbose {
		if logFile := openVerboseLog(canonical); logFile != nil {
			defer logFile.Close()
			cmd.Stderr = io.MultiWriter(&stderrBuf, logFile)
		}
	}

	if err := cmd.Start(); err != nil {
		// cmd.Wait() never runs, so close the pipe ourselves to avoid
		// leaking the file descriptor.
		_ = stdout.Close()
		return nil, &Error{Code: ErrUnknown, Message: err.Error()}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if scanErr := streamProgress(stdout, onProgress); scanErr != nil {
			log.Printf("downloader: progress scanner: %v", scanErr)
		}
	}()

	waitErr := cmd.Wait()
	wg.Wait()

	if waitErr != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, &Error{Code: ErrCancelled, Message: "Cancelled."}
		}
		code, msg := CategorizeStderr(stderrBuf.String())
		return nil, &Error{Code: code, Message: msg, Stderr: stderrBuf.String()}
	}

	// Post-process: resize embedded cover art down to ≤ ThumbnailMaxPx.
	if s.EmbedThumbnail && s.ThumbnailMaxPx > 0 {
		onProgress(Progress{Stage: StagePostProcess, Message: "Resizing cover art…"})
		if err := postprocess.ResizeCoverArtInMP3(expectedPath, s.ThumbnailMaxPx); err != nil {
			return nil, &Error{Code: ErrUnknown, Message: "cover resize: " + err.Error()}
		}
	}

	onProgress(Progress{Stage: StageDone, Percent: 100, Message: "Done"})
	return &Result{
		VideoID:    meta.ID,
		Title:      meta.Title,
		OutputPath: expectedPath,
	}, nil
}

// fetchMetadata calls yt-dlp with --skip-download --print-json to grab
// the title/uploader/duration. Used for filename construction and UI.
//
// Warnings are deliberately NOT suppressed: yt-dlp reports a missing JS
// runtime as a warning, and swallowing it turns a diagnosable failure into
// an opaque "This video is not available".
func fetchMetadata(ctx context.Context, url string, verbose bool) (*Metadata, error) {
	ytdlpPath, err := tools.Locate("yt-dlp")
	if err != nil {
		return nil, &Error{Code: ErrUnknown, Message: "yt-dlp not found: " + err.Error()}
	}
	args := append(jsRuntimeArgs(),
		"--skip-download",
		"--print-json",
		url,
	)
	if verbose {
		args = append([]string{"-v"}, args...)
	}
	cmd := exec.CommandContext(ctx, ytdlpPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if verbose {
		if logFile := openVerboseLog(url); logFile != nil {
			defer logFile.Close()
			cmd.Stderr = io.MultiWriter(&stderr, logFile)
		}
	}
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, &Error{Code: ErrCancelled, Message: "Cancelled."}
		}
		code, msg := CategorizeStderr(stderr.String())
		return nil, &Error{Code: code, Message: msg, Stderr: stderr.String()}
	}
	var meta Metadata
	if err := json.Unmarshal(stdout.Bytes(), &meta); err != nil {
		return nil, &Error{Code: ErrUnknown, Message: "metadata json: " + err.Error()}
	}
	if meta.ID == "" {
		return nil, &Error{Code: ErrUnknown, Message: "no video ID returned"}
	}
	return &meta, nil
}

// buildYtDlpArgs assembles the all-in-one command for MMI-compliant MP3.
//
// Bitrate is forced to CBR via --audio-quality <bitrate>K. yt-dlp's
// default --audio-quality is VBR 5 (~130 kbps), and even --audio-quality 0
// produces VBR ~200 kbps; for Audi MMI we want predictable CBR.
//
// Sample rate and channels go through --postprocessor-args using the
// ExtractAudio: prefix so they only apply to the audio extraction step
// (not, say, thumbnail conversion).
//
// --no-warnings is intentionally absent — see fetchMetadata.
func buildYtDlpArgs(url, outputTemplate string, s Settings) []string {
	bitrate := clamp(s.Bitrate, 64, 320)
	ppArgs := fmt.Sprintf("ExtractAudio:-ac %s -ar %s",
		strconv.Itoa(clamp(s.Channels, 1, 2)),
		strconv.Itoa(clamp(s.SampleRate, 8000, 48000)),
	)

	args := []string{
		"--newline",
		"--no-playlist",
		"--no-colors",
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", strconv.Itoa(bitrate) + "K",
		"--postprocessor-args", ppArgs,
		"--output", outputTemplate,
	}
	if s.EmbedMetadata {
		args = append(args, "--embed-metadata")
	}
	if s.EmbedThumbnail {
		args = append(args, "--embed-thumbnail", "--convert-thumbnails", "jpg")
	}
	if s.Verbose {
		args = append(args, "-v")
	}
	args = append(args, url)
	return args
}

func streamProgress(r io.Reader, onProgress func(Progress)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if p, ok := parseProgressLine(line); ok {
			onProgress(p)
		}
	}
	return scanner.Err()
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// SanitizeForLog returns stderr with control chars stripped, suitable
// for displaying inline in a UI. Currently a thin wrapper.
func SanitizeForLog(s string) string {
	return strings.ReplaceAll(s, "\x00", "")
}
