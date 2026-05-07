// Package paths centralises macOS-conventional file locations.
package paths

import (
	"errors"
	"os"
	"path/filepath"
)

const appName = "MMI Tunes"

// AppSupportDir returns ~/Library/Application Support/MMI Tunes,
// creating it if missing.
func AppSupportDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Library", "Application Support", appName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// SettingsPath returns ~/Library/Application Support/MMI Tunes/settings.json.
func SettingsPath() (string, error) {
	d, err := AppSupportDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "settings.json"), nil
}

// HistoryPath returns ~/Library/Application Support/MMI Tunes/history.json.
func HistoryPath() (string, error) {
	d, err := AppSupportDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "history.json"), nil
}

// JobsPath returns ~/Library/Application Support/MMI Tunes/jobs.json.
func JobsPath() (string, error) {
	d, err := AppSupportDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "jobs.json"), nil
}

// DefaultDownloadDir returns ~/Music/MMI Tunes, creating it if missing.
func DefaultDownloadDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Music", appName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// EnsureDir creates path (and parents) if missing. Returns an error if
// path exists but is not a directory.
func EnsureDir(path string) error {
	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return errors.New("path exists and is not a directory: " + path)
		}
		return nil
	}
	return os.MkdirAll(path, 0o755)
}
