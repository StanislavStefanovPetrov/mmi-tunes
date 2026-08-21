package tools

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFakeTool puts an executable script named `name` in its own dir and
// returns that dir, for use as a one-entry PATH.
func writeFakeTool(t *testing.T, name, script string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestProbe_ReportsVersion(t *testing.T) {
	t.Setenv("PATH", writeFakeTool(t, "faketool", "#!/bin/sh\necho 1.2.3\n"))

	s := Probe("faketool", "--version")
	if !s.Found || s.Version != "1.2.3" {
		t.Fatalf("got %+v, want found with version 1.2.3", s)
	}
}

// A tool that fails without writing to stderr must never come back as
// "found, no version, no error". That combination is indistinguishable
// from success in the UI, and it is what hid the fact that the yt-dlp
// version probe was being killed by its own timeout — leaving the
// diagnostics panel blank instead of showing a 5-month-old yt-dlp.
func TestProbe_FailureIsNeverSilent(t *testing.T) {
	t.Setenv("PATH", writeFakeTool(t, "faketool", "#!/bin/sh\nexit 3\n"))

	s := Probe("faketool", "--version")
	if s.Version == "" && s.Error == "" {
		t.Fatalf("got %+v — a failed probe with no version and no error reads as success", s)
	}
}

func TestProbe_NotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	s := Probe("faketool", "--version")
	if s.Found {
		t.Errorf("got %+v, want found=false", s)
	}
	if s.Error == "" {
		t.Error("a missing tool must say why")
	}
}

// The bundled yt-dlp is a PyInstaller bundle that needs ~9s just to print
// its version. The timeout has to stay comfortably above that or the probe
// silently reports nothing for the single most important tool.
func TestProbeTimeout_LeavesRoomForPyInstallerColdStart(t *testing.T) {
	const observedYtDlpColdStart = 10 // seconds, measured 2026-08-21
	if probeTimeout.Seconds() <= observedYtDlpColdStart {
		t.Errorf("probeTimeout is %s, too tight for yt-dlp's ~%ds cold start", probeTimeout, observedYtDlpColdStart)
	}
}
