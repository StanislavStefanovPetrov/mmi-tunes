// Package url provides YouTube URL validation and normalization.
package url

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

var (
	ErrInvalidURL     = errors.New("not a valid URL")
	ErrNotYouTube     = errors.New("not a YouTube URL")
	ErrNoVideoID      = errors.New("no video ID found in URL")
)

var videoIDRegex = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

var youtubeHosts = map[string]bool{
	"youtube.com":       true,
	"www.youtube.com":   true,
	"m.youtube.com":     true,
	"music.youtube.com": true,
	"youtu.be":          true,
}

// IsYouTubeURL reports whether s is a syntactically valid YouTube URL
// from which we can extract a video ID.
func IsYouTubeURL(s string) bool {
	_, err := ExtractVideoID(s)
	return err == nil
}

// ExtractVideoID returns the canonical 11-character YouTube video ID
// from any of the supported URL forms:
//
//	https://www.youtube.com/watch?v=ID&...
//	https://youtu.be/ID?t=10
//	https://www.youtube.com/embed/ID
//	https://www.youtube.com/shorts/ID
//	https://m.youtube.com/watch?v=ID
//	https://music.youtube.com/watch?v=ID
//
// A bare 11-character ID is also accepted.
func ExtractVideoID(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrInvalidURL
	}

	// Bare video ID
	if videoIDRegex.MatchString(raw) {
		return raw, nil
	}

	// Add scheme if missing so url.Parse accepts it
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", ErrInvalidURL
	}

	host := strings.ToLower(u.Host)
	if !youtubeHosts[host] {
		return "", ErrNotYouTube
	}

	// youtu.be/<id>
	if host == "youtu.be" {
		id := strings.TrimPrefix(u.Path, "/")
		id = strings.SplitN(id, "/", 2)[0]
		if videoIDRegex.MatchString(id) {
			return id, nil
		}
		return "", ErrNoVideoID
	}

	// /watch?v=<id>
	if strings.HasPrefix(u.Path, "/watch") {
		if id := u.Query().Get("v"); videoIDRegex.MatchString(id) {
			return id, nil
		}
		return "", ErrNoVideoID
	}

	// /embed/<id>, /shorts/<id>, /v/<id>, /live/<id>
	for _, prefix := range []string{"/embed/", "/shorts/", "/v/", "/live/"} {
		if strings.HasPrefix(u.Path, prefix) {
			id := strings.TrimPrefix(u.Path, prefix)
			id = strings.SplitN(id, "/", 2)[0]
			if videoIDRegex.MatchString(id) {
				return id, nil
			}
			return "", ErrNoVideoID
		}
	}

	return "", ErrNoVideoID
}
