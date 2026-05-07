package url

import "testing"

func TestExtractVideoID(t *testing.T) {
	const id = "dQw4w9WgXcQ"
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"watch", "https://www.youtube.com/watch?v=" + id, id, true},
		{"watch+params", "https://www.youtube.com/watch?v=" + id + "&t=42&list=PL123", id, true},
		{"http scheme", "http://www.youtube.com/watch?v=" + id, id, true},
		{"no scheme", "www.youtube.com/watch?v=" + id, id, true},
		{"mobile", "https://m.youtube.com/watch?v=" + id, id, true},
		{"music", "https://music.youtube.com/watch?v=" + id, id, true},
		{"short youtu.be", "https://youtu.be/" + id, id, true},
		{"short youtu.be+param", "https://youtu.be/" + id + "?t=10", id, true},
		{"embed", "https://www.youtube.com/embed/" + id, id, true},
		{"shorts", "https://www.youtube.com/shorts/" + id, id, true},
		{"v path", "https://www.youtube.com/v/" + id, id, true},
		{"live path", "https://www.youtube.com/live/" + id, id, true},
		{"bare id", id, id, true},
		{"trailing whitespace", "  https://youtu.be/" + id + "  ", id, true},

		{"empty", "", "", false},
		{"non-youtube domain", "https://vimeo.com/12345", "", false},
		{"watch without v param", "https://www.youtube.com/watch", "", false},
		{"watch with bad id", "https://www.youtube.com/watch?v=tooshort", "", false},
		{"junk", "not a url at all", "", false},
		{"random domain looking like yt", "https://example.com/watch?v=" + id, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExtractVideoID(tc.in)
			if tc.ok {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.want {
					t.Fatalf("got %q, want %q", got, tc.want)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
			}
		})
	}
}

func TestCanonical(t *testing.T) {
	const id = "dQw4w9WgXcQ"
	want := "https://www.youtube.com/watch?v=" + id
	inputs := []string{
		"https://youtu.be/" + id + "?t=42",
		"https://www.youtube.com/watch?v=" + id + "&list=PL",
		"https://m.youtube.com/watch?v=" + id,
		id,
	}
	for _, in := range inputs {
		got, err := Canonical(in)
		if err != nil {
			t.Fatalf("Canonical(%q) error: %v", in, err)
		}
		if got != want {
			t.Errorf("Canonical(%q) = %q, want %q", in, got, want)
		}
	}
}
