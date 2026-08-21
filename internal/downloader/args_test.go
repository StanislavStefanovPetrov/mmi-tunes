package downloader

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func testSettings() Settings {
	return MMIDefaults("/tmp/out")
}

// --no-warnings used to be passed unconditionally, which discarded the
// "No supported JavaScript runtime could be found" warning — the single
// line that explained why every download failed. Suppressing warnings
// means suppressing diagnosis, so it must stay gone.
func TestBuildYtDlpArgs_DoesNotSuppressWarnings(t *testing.T) {
	args := buildYtDlpArgs("https://youtu.be/x", "/tmp/out/%(ext)s", testSettings())
	if slices.Contains(args, "--no-warnings") {
		t.Error("--no-warnings is back; yt-dlp reports a missing JS runtime as a warning, so this hides the cause of failure")
	}
}

func TestBuildYtDlpArgs_VerboseOnlyWhenEnabled(t *testing.T) {
	s := testSettings()

	if args := buildYtDlpArgs("u", "o", s); slices.Contains(args, "-v") {
		t.Error("-v present with VerboseLogging off; verbose output belongs behind the setting")
	}

	s.Verbose = true
	args := buildYtDlpArgs("u", "o", s)
	if !slices.Contains(args, "-v") {
		t.Error("-v missing with Verbose on — the setting would do nothing")
	}
	if args[len(args)-1] != "u" {
		t.Errorf("URL must stay last, got %q", args[len(args)-1])
	}
}

// The Audi MMI output spec is the whole point of the app; assert the flags
// that enforce it survive any future arg reshuffling.
func TestBuildYtDlpArgs_KeepsMMISpec(t *testing.T) {
	args := buildYtDlpArgs("u", "o", testSettings())
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--audio-format mp3",
		"--audio-quality 320K",         // CBR, not yt-dlp's default VBR
		"ExtractAudio:-ac 2 -ar 48000", // stereo / 48 kHz
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q from args: %s", want, joined)
		}
	}
}

// jsRuntimeArgs is what makes YouTube hand back audio formats at all. It
// returns nil rather than erroring when qjs is absent, so the download
// still runs (and fails with a diagnosable message) instead of being
// blocked before it starts.
func TestJSRuntimeArgs_ShapeIsQuickJSPrefixed(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "qjs")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	args := jsRuntimeArgs()
	if len(args) != 2 || args[0] != "--js-runtimes" {
		t.Fatalf("args = %v, want [--js-runtimes quickjs:<path>]", args)
	}
	// The prefix is not cosmetic: yt-dlp parses RUNTIME[:PATH], so a bare
	// path would be read as a runtime name and silently ignored.
	if args[1] != "quickjs:"+fake {
		t.Errorf("args[1] = %q, want %q", args[1], "quickjs:"+fake)
	}
}

// A missing qjs must degrade to "no flags" rather than an error: the
// download then runs and fails with a diagnosable message instead of being
// blocked before it starts.
func TestJSRuntimeArgs_NilWhenQjsAbsent(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if args := jsRuntimeArgs(); args != nil {
		t.Errorf("args = %v, want nil when qjs cannot be found", args)
	}
}
