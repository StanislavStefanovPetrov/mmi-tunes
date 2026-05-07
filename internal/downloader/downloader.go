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

	"github.com/StanislavStefanovPetrov/mmi-tunes/internal/postprocess"
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
	meta, err := fetchMetadata(ctx, canonical)
	if err != nil {
		return nil, err
	}

	sanitized := postprocess.SanitizeFilename(meta.Title, meta.ID)

	// yt-dlp expands %(ext)s to the final extension after post-processing.
	outputTemplate := filepath.Join(s.DownloadFolder, sanitized+".%(ext)s")
	expectedPath := filepath.Join(s.DownloadFolder, sanitized+".mp3")

	args := buildYtDlpArgs(canonical, outputTemplate, s)

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, &Error{Code: ErrUnknown, Message: err.Error()}
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

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
func fetchMetadata(ctx context.Context, url string) (*Metadata, error) {
	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--skip-download",
		"--print-json",
		"--no-warnings",
		url,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
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
func buildYtDlpArgs(url, outputTemplate string, s Settings) []string {
	bitrate := clamp(s.Bitrate, 64, 320)
	ppArgs := fmt.Sprintf("ExtractAudio:-ac %s -ar %s",
		strconv.Itoa(clamp(s.Channels, 1, 2)),
		strconv.Itoa(clamp(s.SampleRate, 8000, 48000)),
	)

	args := []string{
		"--newline",
		"--no-playlist",
		"--no-warnings",
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
