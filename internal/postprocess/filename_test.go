package postprocess

import (
	"strings"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		fallback string
		want     string
	}{
		{
			name: "plain ASCII",
			in:   "Rick Astley - Never Gonna Give You Up (Official Video)",
			want: "Rick Astley - Never Gonna Give You Up (Official Video)",
		},
		{
			name: "Bulgarian Cyrillic",
			in:   "Хитова Песен 2024",
			want: "Hitova Pesen 2024",
		},
		{
			name: "mixed Cyrillic + Latin",
			in:   "Live: Хитова Песен (Sofia)",
			want: "Live- Hitova Pesen (Sofia)",
		},
		{
			name: "forbidden chars replaced with dash",
			in:   `What\Is/This:Path*?"<>|Stuff`,
			want: "What-Is-This-Path------Stuff",
		},
		{
			name: "control chars dropped",
			in:   "Hello\x00World\x07Test",
			want: "HelloWorldTest",
		},
		{
			name: "whitespace collapsed",
			in:   "Hello    World\t\tFoo  \n  Bar",
			want: "Hello World Foo Bar",
		},
		{
			name: "trailing dots and spaces trimmed",
			in:   "  ...Hello World... ",
			want: "Hello World",
		},
		{
			name: "non-transliterable unicode dropped",
			in:   "Mix 中文 + Latin",
			want: "Mix + Latin",
		},
		{
			name:     "all unicode dropped → fallback",
			in:       "中文 한국어 العربية",
			fallback: "dQw4w9WgXcQ",
			want:     "dQw4w9WgXcQ",
		},
		{
			name:     "empty input → fallback",
			in:       "",
			fallback: "abc123XYZ_-",
			want:     "abc123XYZ_-",
		},
		{
			name:     "only forbidden chars → keeps dashes",
			in:       `\\\///:::***???`,
			fallback: "fallback",
			want:     "---------------",
		},
		{
			name: "Russian Cyrillic with ё ы э",
			in:   "Дождь Идёт По Улице",
			want: "Dozhd Idyot Po Ulitse",
		},
		{
			name: "long title truncated to 80 runes",
			in:   strings.Repeat("a", 100),
			want: strings.Repeat("a", 80),
		},
		{
			name: "long Cyrillic title truncated after transliteration",
			in:   strings.Repeat("щ", 30),
			want: strings.Repeat("sht", 30)[:80],
		},
		{
			name: "emoji dropped",
			in:   "Song Title 🎵🎶 Live",
			want: "Song Title Live",
		},
		{
			name: "trailing dots after truncation also trimmed",
			in:   strings.Repeat("a", 78) + " . . .",
			want: strings.Repeat("a", 78),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fallback := tc.fallback
			if fallback == "" {
				fallback = "FALLBACK"
			}
			got := SanitizeFilename(tc.in, fallback)
			if got != tc.want {
				t.Errorf("SanitizeFilename(%q) =\n  got  %q\n  want %q", tc.in, got, tc.want)
			}
			for _, r := range got {
				if forbiddenChars[r] {
					t.Errorf("output contains forbidden char %q", r)
				}
			}
			if len([]rune(got)) > MaxFilenameRunes {
				t.Errorf("output exceeds MaxFilenameRunes (%d > %d)", len([]rune(got)), MaxFilenameRunes)
			}
		})
	}
}
