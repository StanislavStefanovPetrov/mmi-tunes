package downloader

import "testing"

func TestParseProgressLine(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		want  Stage
		pct   float64
		ok    bool
	}{
		{"download with percent", "[download]   12.3% of   3.45MiB at 1.23MiB/s ETA 00:02", StageDownload, 12.3, true},
		{"download 100", "[download] 100% of 4.12MiB", StageDownload, 100, true},
		{"download starting", "[download] Destination: /tmp/x.mp4", StageDownload, 0, true},
		{"extract audio", "[ExtractAudio] Destination: /tmp/x.mp3", StageConvert, 0, true},
		{"metadata", "[Metadata] Adding metadata to \"/tmp/x.mp3\"", StageEmbedMeta, 0, true},
		{"thumbnail convert", "[ThumbnailsConvertor] Converting thumbnail \"x.webp\" to jpg", StageEmbedThumb, 0, true},
		{"embed thumb", "[EmbedThumbnail] Adding thumbnail to \"x.mp3\"", StageEmbedThumb, 0, true},
		{"info line", "[info] dQw4w9WgXcQ: Downloading 1 format(s)", StageMetadata, 0, true},
		{"youtube extractor", "[youtube] dQw4w9WgXcQ: Downloading webpage", StageMetadata, 0, true},
		{"empty", "", "", 0, false},
		{"random noise", "Some unrelated stdout", "", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseProgressLine(tc.in)
			if ok != tc.ok {
				t.Fatalf("ok=%v, want %v (got=%+v)", ok, tc.ok, got)
			}
			if !ok {
				return
			}
			if got.Stage != tc.want {
				t.Errorf("Stage = %q, want %q", got.Stage, tc.want)
			}
			if tc.pct > 0 && got.Percent != tc.pct {
				t.Errorf("Percent = %v, want %v", got.Percent, tc.pct)
			}
		})
	}
}
