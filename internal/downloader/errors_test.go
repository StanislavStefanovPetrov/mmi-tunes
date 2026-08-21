package downloader

import (
	"strings"
	"testing"
)

func TestCategorizeStderr(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want ErrorCode
	}{
		{"empty", "", ErrUnknown},
		{"unknown junk", "something random went wrong", ErrUnknown},

		{"bot check ASCII", "Sign in to confirm you're not a bot", ErrBotCheck},
		{"bot check curly", "Sign in to confirm you’re not a bot", ErrBotCheck},
		{"bot check cookies hint", "WARNING: use --cookies-from-browser or --cookies", ErrBotCheck},

		{"age", "Sign in to confirm your age", ErrAgeRestricted},
		{"age2", "This video may be inappropriate for some users", ErrAgeRestricted},

		{"premium", "This video requires Premium", ErrPremium},
		{"members only", "Join this channel to get access to members-only content", ErrPremium},

		{"geo us", "This video is not available in your country", ErrGeoBlocked},
		{"geo restricted", "ERROR: This video is restricted, blocked it on copyright grounds", ErrCopyright}, // copyright wins by order
		{"geo from your country", "available from your country", ErrGeoBlocked},

		{"removed", "Video unavailable\nThis video has been removed by the uploader", ErrUnavailable},
		{"unavailable", "Video unavailable", ErrUnavailable},
		// Once a JS runtime is present, this phrasing means the video really
		// is gone — it must not be blamed on the runtime.
		{"not available, no js evidence", "ERROR: [youtube] abc: This video is not available", ErrUnavailable},

		{"private", "ERROR: Private video", ErrPrivate},

		{"copyright", "removed for copyright reasons", ErrCopyright},

		{"folder missing", "no such file or directory: /tmp/x.mp3", ErrFolderMissing},
		{"perm denied", "permission denied", ErrFolderMissing},
		{"disk full", "No space left on device", ErrFolderMissing},

		{"ffmpeg missing", "ffprobe and ffmpeg not found", ErrFFmpegMissing},

		{"js runtime missing", "WARNING: [youtube] No supported JavaScript runtime could be found.", ErrJSRuntime},
		{"n challenge failed", "WARNING: [youtube] abc: n challenge solving failed: Some formats may be missing.", ErrJSRuntime},
		{"n challenge solver error", `WARNING: [youtube] [jsc] Error solving n challenge request using "node" provider`, ErrJSRuntime},

		{"format not available", "ERROR: Requested format is not available", ErrYtDlpOutdated},
		{"unable to extract", "Unable to extract player response", ErrYtDlpOutdated},

		{"http 403", "ERROR: unable to download video data: HTTP Error 403: Forbidden", ErrNetwork},
		{"http 429", "HTTP Error 429: Too Many Requests", ErrNetwork},
		{"http 503", "HTTP Error 503: Service Unavailable", ErrNetwork},
		{"dns", "could not resolve host: youtube.com", ErrNetwork},
		{"timeout", "operation timeout", ErrNetwork},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, msg := CategorizeStderr(tc.in)
			if code != tc.want {
				t.Errorf("code = %s, want %s; msg = %q", code, tc.want, msg)
			}
		})
	}
}

// realNoJSRuntimeStderr is the verbatim stderr captured from the shipped
// v1.0.1 bundled yt-dlp (2026.03.17) for video hBKJ5oR3Ciw on 2026-08-21 —
// a public video that plays fine in a browser.
//
// This is the failure the user actually hit. It is a three-way trap: the
// stderr simultaneously suggests "video is gone" ("This video is not
// available"), "yt-dlp is stale" (the 90-days warning), and carries the one
// line that names the real cause. Classifying it as anything other than
// ErrJSRuntime sends the user chasing the wrong fix — which is exactly what
// happened before this change.
const realNoJSRuntimeStderr = `WARNING: Your yt-dlp version (2026.03.17) is older than 90 days!
         It is strongly recommended to always use the latest version.
         Run "yt-dlp --update" or "yt-dlp -U" to update.
         To suppress this warning, add --no-update to your command/config.
WARNING: [youtube] No supported JavaScript runtime could be found. Only deno is enabled by default; to use another runtime add  --js-runtimes RUNTIME[:PATH]  to your command/config. YouTube extraction without a JS runtime has been deprecated, and some formats may be missing. See  https://github.com/yt-dlp/yt-dlp/wiki/EJS  for details on installing one
ERROR: [youtube] hBKJ5oR3Ciw: This video is not available`

func TestCategorizeStderr_RealMissingJSRuntime(t *testing.T) {
	code, msg := CategorizeStderr(realNoJSRuntimeStderr)
	if code != ErrJSRuntime {
		t.Fatalf("code = %s, want %s — the actionable cause is the missing JS runtime, not %q", code, ErrJSRuntime, msg)
	}
	if !strings.Contains(msg, "JavaScript runtime") {
		t.Errorf("message %q should tell the user a JavaScript runtime is the problem", msg)
	}
}

// A 403 on a format URL is what a missing JS runtime looks like at download
// time (the URL was never properly signed). Previously this reported
// "Network error — check your connection", pointing at the user's wifi
// instead of the app's own missing dependency.
func TestCategorizeStderr_Unsigned403IsNotBlamedOnTheNetwork(t *testing.T) {
	stderr := `WARNING: [youtube] abc: n challenge solving failed: Some formats may be missing.
ERROR: unable to download video data: HTTP Error 403: Forbidden`
	if code, msg := CategorizeStderr(stderr); code != ErrJSRuntime {
		t.Errorf("code = %s (%q), want %s", code, msg, ErrJSRuntime)
	}
}
