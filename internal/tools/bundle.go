package tools

import (
	"os"
	"path/filepath"
)

// BundleResourcePath returns the absolute path to a file inside the
// running .app bundle's Contents/Resources/ directory, or "" if we are
// not running from a bundle (e.g. `wails dev`, `go test`, smoke CLI).
//
// Layout assumed:
//
//	MMI Tunes.app/
//	└── Contents/
//	    ├── MacOS/MMI Tunes      ← os.Executable()
//	    └── Resources/<name>     ← what we return
func BundleResourcePath(name string) string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err == nil {
		exe = resolved
	}
	macOSDir := filepath.Dir(exe)             // .../Contents/MacOS
	contentsDir := filepath.Dir(macOSDir)     // .../Contents
	resourcesDir := filepath.Join(contentsDir, "Resources")
	candidate := filepath.Join(resourcesDir, name)
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate
	}
	return ""
}
