// Smoke tester for the downloader package — bypasses the GUI entirely.
//
// Usage:
//
//	go run ./cmd/cli <youtube-url>
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/StanislavStefanovPetrov/mmi-tunes/internal/downloader"
	"github.com/StanislavStefanovPetrov/mmi-tunes/internal/tools"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: cli <youtube-url> [output-folder]")
		os.Exit(2)
	}
	url := os.Args[1]

	folder := "."
	if len(os.Args) >= 3 {
		folder = os.Args[2]
	}
	abs, _ := filepath.Abs(folder)
	if err := os.MkdirAll(abs, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir:", err)
		os.Exit(1)
	}

	status := tools.CheckAll()
	if !status.YtDlp.Found {
		fmt.Fprintln(os.Stderr, "yt-dlp not found in PATH —", status.YtDlp.Error)
		os.Exit(1)
	}
	if !status.FFmpeg.Found {
		fmt.Fprintln(os.Stderr, "ffmpeg not found in PATH —", status.FFmpeg.Error)
		os.Exit(1)
	}
	// Not fatal: without a JS runtime the download still runs and fails
	// with a diagnosable message, which is worth exercising here.
	if !status.JSRuntime.Found {
		fmt.Fprintln(os.Stderr, "WARNING: qjs not found — YouTube will return no audio formats")
	}
	fmt.Printf("yt-dlp %s\n", status.YtDlp.Version)
	fmt.Printf("ffmpeg %s\n", status.FFmpeg.Version)
	fmt.Printf("qjs    %s\n", status.JSRuntime.Version)
	fmt.Printf("output: %s\n\n", abs)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	settings := downloader.MMIDefaults(abs)

	res, err := downloader.Download(ctx, url, settings, func(p downloader.Progress) {
		if p.Percent > 0 {
			fmt.Printf("\r[%s] %5.1f%%   ", p.Stage, p.Percent)
		} else if p.Message != "" {
			fmt.Printf("\n[%s] %s", p.Stage, p.Message)
		}
	})
	fmt.Println()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
	fmt.Printf("\nDONE: %s\n  → %s\n", res.Title, res.OutputPath)
}
