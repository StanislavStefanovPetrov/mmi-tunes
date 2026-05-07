package tools

import (
	"os"
	"strings"
)

// commonGUIMissingPaths is the set of directories that GUI macOS apps
// don't get on PATH by default but where yt-dlp / ffmpeg are commonly
// installed: Apple-Silicon Homebrew, Intel Homebrew, MacPorts, and
// pipx user installs.
var commonGUIMissingPaths = []string{
	"/opt/homebrew/bin",
	"/opt/homebrew/sbin",
	"/usr/local/bin",
	"/usr/local/sbin",
	"/opt/local/bin", // MacPorts
}

// AugmentPATH prepends the common Homebrew/MacPorts directories to the
// process's PATH if they are missing. macOS .app bundles launched from
// Finder/Spotlight start with a minimal PATH (/usr/bin:/bin:/usr/sbin:/sbin)
// that excludes Homebrew, so without this fix `exec.LookPath("yt-dlp")`
// fails even though the tool is installed.
//
// Idempotent — directories already present in PATH are skipped.
// Only adds directories that actually exist.
func AugmentPATH() {
	cur := os.Getenv("PATH")
	parts := strings.Split(cur, string(os.PathListSeparator))
	have := make(map[string]bool, len(parts))
	for _, p := range parts {
		have[p] = true
	}

	prepend := []string{}
	for _, p := range commonGUIMissingPaths {
		if have[p] {
			continue
		}
		if info, err := os.Stat(p); err != nil || !info.IsDir() {
			continue
		}
		prepend = append(prepend, p)
	}
	if len(prepend) == 0 {
		return
	}
	newPath := strings.Join(prepend, string(os.PathListSeparator))
	if cur != "" {
		newPath = newPath + string(os.PathListSeparator) + cur
	}
	_ = os.Setenv("PATH", newPath)
}
