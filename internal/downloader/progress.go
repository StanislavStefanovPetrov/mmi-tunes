package downloader

import (
	"strconv"
	"strings"
)

// Stage identifies what yt-dlp is currently doing for a single video.
type Stage string

const (
	StageQueued      Stage = "queued"
	StageMetadata    Stage = "metadata"
	StageDownload    Stage = "download"
	StageConvert     Stage = "convert"
	StageEmbedMeta   Stage = "embed_metadata"
	StageEmbedThumb  Stage = "embed_thumbnail"
	StagePostProcess Stage = "post_process"
	StageDone        Stage = "done"
	StageError       Stage = "error"
)

// Progress reports a single tick of activity from yt-dlp's stdout.
type Progress struct {
	Stage   Stage   `json:"stage"`
	Percent float64 `json:"percent,omitempty"` // 0..100, only set during download
	Message string  `json:"message,omitempty"` // human-readable status line
}

// parseProgressLine inspects one line of yt-dlp --newline stdout and
// returns a Progress if recognised, or false to skip.
//
// Recognised formats (yt-dlp 2023+):
//
//	[download]   12.3% of   3.45MiB at 1.23MiB/s ETA 00:02
//	[ExtractAudio] Destination: /tmp/foo.mp3
//	[Metadata] Adding metadata to "/tmp/foo.mp3"
//	[ThumbnailsConvertor] Converting thumbnail "..." from webp to jpg
//	[EmbedThumbnail] Adding thumbnail to "..."
func parseProgressLine(line string) (Progress, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Progress{}, false
	}

	switch {
	case strings.HasPrefix(line, "[download]"):
		rest := strings.TrimSpace(strings.TrimPrefix(line, "[download]"))
		// "12.3% of  3.45MiB at  1.23MiB/s ETA 00:02"
		if pctIdx := strings.Index(rest, "%"); pctIdx > 0 {
			head := rest[:pctIdx]
			head = strings.TrimSpace(head)
			if v, err := strconv.ParseFloat(head, 64); err == nil {
				return Progress{Stage: StageDownload, Percent: v, Message: line}, true
			}
		}
		return Progress{Stage: StageDownload, Message: line}, true

	case strings.HasPrefix(line, "[ExtractAudio]"):
		return Progress{Stage: StageConvert, Message: line}, true

	case strings.HasPrefix(line, "[Metadata]"):
		return Progress{Stage: StageEmbedMeta, Message: line}, true

	case strings.HasPrefix(line, "[ThumbnailsConvertor]"),
		strings.HasPrefix(line, "[EmbedThumbnail]"):
		return Progress{Stage: StageEmbedThumb, Message: line}, true

	case strings.HasPrefix(line, "[info]"),
		strings.HasPrefix(line, "[youtube]"),
		strings.HasPrefix(line, "[generic]"):
		return Progress{Stage: StageMetadata, Message: line}, true
	}

	return Progress{}, false
}
