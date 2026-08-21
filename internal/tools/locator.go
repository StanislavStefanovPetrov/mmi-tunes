// Package tools locates and inspects external CLI dependencies
// (yt-dlp, ffmpeg, qjs).
package tools

import (
	"context"
	"errors"
	"fmt"
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
	YtDlp     Status `json:"ytdlp"`
	FFmpeg    Status `json:"ffmpeg"`
	JSRuntime Status `json:"jsruntime"`
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

// probeTimeout is generous because the bundled yt-dlp is a PyInstaller
// bundle that unpacks itself on every run — `yt-dlp --version` alone takes
// ~9s cold. At the previous 5s the probe was always killed, and since a
// killed process leaves no stderr the Status came back Found-but-blank:
// the diagnostics panel silently showed no yt-dlp version at all, hiding
// exactly the staleness it exists to surface.
const probeTimeout = 20 * time.Second

// Probe locates `name` on PATH and runs `name <versionArg>` to capture
// the first line of output as the version.
func Probe(name string, versionArg string) Status {
	s := Status{Name: name}
	path, err := Locate(name)
	if err != nil {
		s.Error = err.Error()
		return s
	}
	s.Found = true
	s.Path = path

	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, versionArg).Output()
	if err != nil {
		// A timeout kill yields an ExitError with empty stderr, which would
		// otherwise report success-with-no-version. Name it explicitly.
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			s.Error = fmt.Sprintf("%s %s timed out after %s", name, versionArg, probeTimeout)
			return s
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			s.Error = strings.TrimSpace(string(ee.Stderr))
			if s.Error == "" {
				s.Error = err.Error()
			}
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

// CheckAll probes yt-dlp, ffmpeg, and the QuickJS runtime.
//
// qjs is not optional: without a JS runtime YouTube returns no audio
// formats at all, so a missing one must be visible rather than silent.
func CheckAll() AllStatus {
	return AllStatus{
		YtDlp:     Probe("yt-dlp", "--version"),
		FFmpeg:    probeFFmpeg(),
		JSRuntime: Probe("qjs", "--version"),
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
