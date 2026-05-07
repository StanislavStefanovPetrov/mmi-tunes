package downloader

import "strings"

// ErrorCode classifies known yt-dlp/ffmpeg failure modes for friendlier UI.
type ErrorCode string

const (
	ErrUnknown        ErrorCode = "UNKNOWN"
	ErrGeoBlocked     ErrorCode = "GEO_BLOCKED"
	ErrAgeRestricted  ErrorCode = "AGE_RESTRICTED"
	ErrUnavailable    ErrorCode = "UNAVAILABLE"
	ErrPrivate        ErrorCode = "PRIVATE"
	ErrCopyright      ErrorCode = "COPYRIGHT_REMOVED"
	ErrNetwork        ErrorCode = "NETWORK"
	ErrFFmpegMissing  ErrorCode = "FFMPEG_MISSING"
	ErrYtDlpOutdated  ErrorCode = "YTDLP_OUTDATED"
	ErrCancelled      ErrorCode = "CANCELLED"
)

// CategorizeStderr maps yt-dlp stderr text to a typed ErrorCode + a clean
// short message suitable for display in the UI.
func CategorizeStderr(stderr string) (ErrorCode, string) {
	s := strings.ToLower(stderr)

	switch {
	case strings.Contains(s, "not available in your country"),
		strings.Contains(s, "geo restricted"),
		strings.Contains(s, "is geo restricted"):
		return ErrGeoBlocked, "Video is geo-blocked in your region."
	case strings.Contains(s, "sign in to confirm your age"),
		strings.Contains(s, "age-restricted"),
		strings.Contains(s, "inappropriate for some users"):
		return ErrAgeRestricted, "Age-restricted video — sign-in required."
	case strings.Contains(s, "video unavailable"),
		strings.Contains(s, "this video has been removed"):
		return ErrUnavailable, "Video is unavailable or removed."
	case strings.Contains(s, "private video"):
		return ErrPrivate, "Private video — owner has restricted access."
	case strings.Contains(s, "removed for copyright"),
		strings.Contains(s, "copyright claim"):
		return ErrCopyright, "Removed for copyright reasons."
	case strings.Contains(s, "ffmpeg"),
		strings.Contains(s, "ffprobe and ffmpeg not found"):
		return ErrFFmpegMissing, "ffmpeg is missing — install with `brew install ffmpeg`."
	case strings.Contains(s, "yt-dlp is out of date"),
		strings.Contains(s, "please update"):
		return ErrYtDlpOutdated, "yt-dlp is out of date — try Update."
	case strings.Contains(s, "name resolution"),
		strings.Contains(s, "network is unreachable"),
		strings.Contains(s, "connection refused"),
		strings.Contains(s, "could not resolve host"):
		return ErrNetwork, "Network error — check your connection."
	}

	if stderr == "" {
		return ErrUnknown, "Unknown error."
	}
	// Trim noisy prefixes; keep just the first non-empty stderr line.
	for _, line := range strings.Split(strings.TrimSpace(stderr), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return ErrUnknown, line
	}
	return ErrUnknown, "Unknown error."
}
