package downloader

import "testing"

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

		{"private", "ERROR: Private video", ErrPrivate},

		{"copyright", "removed for copyright reasons", ErrCopyright},

		{"folder missing", "no such file or directory: /tmp/x.mp3", ErrFolderMissing},
		{"perm denied", "permission denied", ErrFolderMissing},
		{"disk full", "No space left on device", ErrFolderMissing},

		{"ffmpeg missing", "ffprobe and ffmpeg not found", ErrFFmpegMissing},

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
