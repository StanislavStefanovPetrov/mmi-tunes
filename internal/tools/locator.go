// Package tools locates and inspects external CLI dependencies (yt-dlp, ffmpeg).
package tools

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// Status reports whether a tool is available and at what version.
type Status struct {
	Name    string `json:"name"`
	Found   bool   `json:"found"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

// AllStatus bundles per-tool status for the diagnostics banner.
type AllStatus struct {
	YtDlp  Status `json:"ytdlp"`
	FFmpeg Status `json:"ffmpeg"`
}

// Locate returns the absolute path to a tool, preferring a binary bundled
// inside the .app's Resources/ directory over whatever is on PATH.
// Bundled binaries make the app self-contained — no `brew install` needed.
func Locate(name string) (string, error) {
	if bundled := BundleResourcePath(name); bundled != "" {
		return bundled, nil
	}
	p, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	return p, nil
}

// Probe locates `name` on PATH and runs `name <versionArg>` to capture
// the first line of output as the version. Times out after 5 seconds.
func Probe(name string, versionArg string) Status {
	s := Status{Name: name}
	path, err := Locate(name)
	if err != nil {
		s.Error = err.Error()
		return s
	}
	s.Found = true
	s.Path = path

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, versionArg).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			s.Error = strings.TrimSpace(string(ee.Stderr))
		} else {
			s.Error = err.Error()
		}
		return s
	}
	if line, _, ok := strings.Cut(strings.TrimSpace(string(out)), "\n"); ok {
		s.Version = line
	} else {
		s.Version = strings.TrimSpace(string(out))
	}
	return s
}

// CheckAll probes both yt-dlp and ffmpeg.
func CheckAll() AllStatus {
	return AllStatus{
		YtDlp:  Probe("yt-dlp", "--version"),
		FFmpeg: probeFFmpeg(),
	}
}

// probeFFmpeg parses ffmpeg's noisy version output.
// `ffmpeg -version` prints "ffmpeg version N.N.N Copyright ...".
func probeFFmpeg() Status {
	s := Probe("ffmpeg", "-version")
	if s.Found && s.Version != "" {
		// Strip the "ffmpeg version " prefix and trailing copyright.
		v := strings.TrimPrefix(s.Version, "ffmpeg version ")
		if i := strings.Index(v, " Copyright"); i > 0 {
			v = v[:i]
		}
		s.Version = v
	}
	return s
}
