# MMI Tunes

macOS app for downloading YouTube audio as Audi MMI–compatible MP3 files.

Replaces the manual workflow of editing `youtube_urls.txt` and running a Go CLI with a polished desktop app. Output MP3s satisfy every Audi MMI 3G+ specification — bitrate ≤ 320 kbit/sec, sample rate ≤ 48 kHz, embedded metadata (artist/title/album), embedded album cover ≤ 800×800 px, FAT32-safe filenames.

## Status

🚧 In active development. See [`/Users/stanislavpetrov/.claude/plans/concurrent-prancing-brook.md`](file:///Users/stanislavpetrov/.claude/plans/concurrent-prancing-brook.md) for the implementation plan.

- [x] Phase 0 — Wails + React + TypeScript + Tailwind bootstrap
- [ ] Phase 1 — MMI-compliant downloader engine
- [ ] Phase 2 — Queue + worker pool
- [ ] Phase 3 — Wails backend API + persistence
- [ ] Phase 4 — React UI
- [ ] Phase 5 — Polish + build

## Tech stack

- Go 1.23 + [Wails v2](https://wails.io)
- React 18 + TypeScript + Vite + Tailwind CSS + Zustand
- External CLI tools: `yt-dlp`, `ffmpeg` (system PATH for now; bundled later)

## Development

```bash
# Prereqs (macOS):
brew install yt-dlp ffmpeg
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Live reload dev mode
wails dev

# Production build (.app)
wails build -platform darwin/arm64
# Output: build/bin/MMI Tunes.app
```

## License

MIT — see [LICENSE](LICENSE).
