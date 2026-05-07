package downloader

import "strings"

// ErrorCode classifies known yt-dlp/ffmpeg failure modes for friendlier UI.
type ErrorCode string

const (
	ErrUnknown       ErrorCode = "UNKNOWN"
	ErrGeoBlocked    ErrorCode = "GEO_BLOCKED"
	ErrAgeRestricted ErrorCode = "AGE_RESTRICTED"
	ErrBotCheck      ErrorCode = "BOT_CHECK"
	ErrPremium       ErrorCode = "PREMIUM_REQUIRED"
	ErrUnavailable   ErrorCode = "UNAVAILABLE"
	ErrPrivate       ErrorCode = "PRIVATE"
	ErrCopyright     ErrorCode = "COPYRIGHT_REMOVED"
	ErrNetwork       ErrorCode = "NETWORK"
	ErrFFmpegMissing ErrorCode = "FFMPEG_MISSING"
	ErrYtDlpOutdated ErrorCode = "YTDLP_OUTDATED"
	ErrFolderMissing ErrorCode = "FOLDER_MISSING"
	ErrCancelled     ErrorCode = "CANCELLED"
)

// CategorizeStderr maps yt-dlp stderr text to a typed ErrorCode + a clean
// short message suitable for display in the UI. Patterns are sourced from
// real yt-dlp 2024-2025 output and known-broken videos. Order matters —
// more specific matches come first.
func CategorizeStderr(stderr string) (ErrorCode, string) {
	s := strings.ToLower(stderr)

	switch {
	// --- Bot / sign-in challenges (most common 2024-25 failure mode) ---
	case strings.Contains(s, "sign in to confirm you're not a bot"),
		strings.Contains(s, "sign in to confirm you’re not a bot"),
		strings.Contains(s, "confirm you're not a bot"),
		strings.Contains(s, "use --cookies"):
		return ErrBotCheck, "YouTube wants a bot check. Sign in to YouTube in a browser, then export cookies."

	// --- Age gate ---
	case strings.Contains(s, "sign in to confirm your age"),
		strings.Contains(s, "age-restricted"),
		strings.Contains(s, "inappropriate for some users"),
		strings.Contains(s, "confirm your age"):
		return ErrAgeRestricted, "Age-restricted video — sign-in required."

	// --- Premium / membership ---
	case strings.Contains(s, "requires premium"),
		strings.Contains(s, "join this channel to get access"),
		strings.Contains(s, "members-only"):
		return ErrPremium, "Video requires YouTube Premium or a channel membership."

	// --- Copyright (checked before "video is restricted" so a copyright
	//     strike with that phrasing isn't misclassified as geo) ---
	case strings.Contains(s, "removed for copyright"),
		strings.Contains(s, "copyright claim"),
		strings.Contains(s, "blocked it on copyright"):
		return ErrCopyright, "Removed for copyright reasons."

	// --- Geo blocking ---
	case strings.Contains(s, "not available in your country"),
		strings.Contains(s, "available from your country"),
		strings.Contains(s, "is geo restricted"),
		strings.Contains(s, "video is restricted"),
		strings.Contains(s, "uploader has not made this video available in your country"):
		return ErrGeoBlocked, "Video is geo-blocked in your region."

	// --- Removed / unavailable ---
	case strings.Contains(s, "this video has been removed"),
		strings.Contains(s, "this video is no longer available"),
		strings.Contains(s, "video unavailable"):
		return ErrUnavailable, "Video is unavailable or removed."

	// --- Private ---
	case strings.Contains(s, "private video"),
		strings.Contains(s, "this video is private"):
		return ErrPrivate, "Private video — owner has restricted access."

	// --- Folder / filesystem problems ---
	case strings.Contains(s, "no such file or directory"),
		strings.Contains(s, "permission denied"),
		strings.Contains(s, "read-only file system"),
		strings.Contains(s, "no space left on device"),
		strings.Contains(s, "disk quota exceeded"):
		return ErrFolderMissing, "Cannot write to download folder — check the path and free space."

	// --- ffmpeg missing ---
	case strings.Contains(s, "ffprobe and ffmpeg not found"),
		strings.Contains(s, "ffmpeg not found"),
		strings.Contains(s, "ffmpeg or avconv could not be found"):
		return ErrFFmpegMissing, "ffmpeg is missing — install with `brew install ffmpeg`."

	// --- yt-dlp outdated ---
	case strings.Contains(s, "yt-dlp is out of date"),
		strings.Contains(s, "please report this issue"),
		strings.Contains(s, "requested format is not available"),
		strings.Contains(s, "unable to extract"),
		strings.Contains(s, "no video formats found"):
		return ErrYtDlpOutdated, "yt-dlp may be outdated — try Update."

	// --- Network ---
	case strings.Contains(s, "name resolution"),
		strings.Contains(s, "network is unreachable"),
		strings.Contains(s, "connection refused"),
		strings.Contains(s, "could not resolve host"),
		strings.Contains(s, "unable to download webpage"),
		strings.Contains(s, "http error 403"),
		strings.Contains(s, "http error 429"),
		strings.Contains(s, "http error 5"),
		strings.Contains(s, "ssl: certificate"),
		strings.Contains(s, "timeout"):
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
