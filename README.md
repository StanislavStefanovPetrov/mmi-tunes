# MMI Tunes

<p align="center">
  <img src="build/appicon.png" width="180" height="180" alt="MMI Tunes app icon">
</p>

<p align="center">
  <strong>YouTube → Audi MMI–compatible MP3, in two clicks.</strong>
</p>

A macOS app that downloads YouTube audio and converts it to MP3s that fully comply with the Audi MMI 3G+ specification — so the tracks play on the in-car system with title, artist, and cover art on the dashboard, no "unsupported format" surprises.

## What it does

<p align="center">
  <img src="docs/screenshots/main.png" width="640" alt="MMI Tunes main window with five jobs in different states">
</p>

- Paste a YouTube URL → click → MP3 file in `~/Music/MMI Tunes/`.
- Bitrate, sample rate, channels, and cover-art size are automatically tuned to Audi MMI's spec sheet.
- Multiple URLs download in parallel (configurable, default 3).
- Real-time progress bars per track, with stage labels (downloading → converting → embedding cover).
- "Already downloaded?" check stops you from grabbing the same video twice — and shows you where the existing file is so you can drop it on your SD card.
- Automatic Cyrillic → Latin transliteration in filenames so they survive on FAT32 SD cards.
- Survives bad URLs, geo-blocked videos, age-gated content, network drops, and YouTube's bot checks with friendly error messages.

<p align="center">
  <img src="docs/screenshots/dedup.png" width="640" alt="Dedup prompt — already-downloaded URL shows path with Reveal in Finder button">
</p>

## Audi MMI compliance, automatic

Every output MP3 hits the Audi MMI 3G+ spec sheet:

| Spec | Audi MMI limit | What MMI Tunes produces |
|---|---|---|
| Codec | MPEG-1/-2 Layer 3 | `.mp3` |
| Bitrate | ≤ 320 kbit/sec | **320 kbps CBR** |
| Sample rate | ≤ 48 kHz | **48 000 Hz** |
| Channels | Stereo | **2** |
| ID3 metadata | Album, track, artist, year, genre, comments | Pulled from YouTube + embedded |
| Cover art | ≤ 800×800 px (JPEG/PNG/GIF) | **Auto-resized** to fit |
| Filename | FAT32-safe (no `\ / : * ? " < > \|`) | **Sanitised + transliterated** |
| Files per directory | ≤ 5000 | Pre-flight warning |

The numbers in the table above come straight from Audi's MMI manual page **Controls ▸ Media drives/connections ▸ Supported media and file formats**. The full spec sheet is also documented in [this AudiWorld forum thread](https://www.audiworld.com/forums/q5-sq5-mki-8r-discussion-129/mmi-3g-largest-sd-card-size-2872958/).

Tested on **Audi MMI 3G+** (Audi Q7 4LB).

## Install

1. Download the latest **`MMI-Tunes-X.Y.Z.pkg`** from [Releases](https://github.com/StanislavStefanovPetrov/mmi-tunes/releases).

2. **Expect macOS to block it the first time.** These builds are unsigned — there is no Apple Developer account behind the project — so downloading tags the file with a quarantine flag and Gatekeeper refuses to open the installer:

   > **"MMI-Tunes-X.Y.Z.pkg" Not Opened**
   > Apple could not verify "MMI-Tunes-X.Y.Z.pkg" is free of malware that may harm your Mac or compromise your privacy.

   Press **Done** — *not* **Move to Bin**. Then open **System Settings ▸ Privacy & Security**, scroll to the bottom, and press **Open Anyway** beside the blocked file.

   On macOS 15 (Sequoia) and later this is the only route Apple supports; right-clicking the file and choosing **Open** no longer bypasses the check the way it did on older releases.

   Prefer the terminal? Clearing the quarantine flag has the same effect, and then a plain double-click works:

   ```bash
   xattr -d com.apple.quarantine ~/Downloads/MMI-Tunes-X.Y.Z.pkg
   ```

3. Apple installer wizard → **Install** → enter your password.

4. Launch from Spotlight (`⌘+Space → "MMI Tunes"`).

`yt-dlp`, `ffmpeg`, `ffprobe`, and `qjs` ship inside the bundle — **no `brew install` needed**, no Homebrew assumed. Works on a fresh Mac.

The app itself starts without any further warning: the installer writes it into `/Applications` as root, and the quarantine flag is not carried over to the installed bundle. The Gatekeeper prompt in step 2 is the only one you should see (verified on macOS 26.6).

## Usage

1. Paste a YouTube URL into the input field, press Enter.
2. The clip appears in the list — click ⬇ to download just that one, or **Download all** at the bottom for the whole batch.
3. When the row turns green and shows ✔ Done, click 📁 to reveal the file in Finder.
4. Drag onto an SD card formatted as FAT32, slide it into the Audi, drive.

### Settings (⚙ icon, top-right)

<p align="center">
  <img src="docs/screenshots/settings.png" width="640" alt="Settings drawer with Audi MMI Preset, audio quality, cover-art and behaviour options">
</p>

- **Download folder** — defaults to `~/Music/MMI Tunes`.
- **Audi MMI Preset** — one click sets bitrate to 320, sample rate to 48000, stereo, cover ≤ 800 px.
- **Concurrent downloads** (1–5).
- **Embed metadata / thumbnail**, **Cyrillic transliteration**, **Skip duplicates**, **Auto-detect clipboard URLs**.
- **Verbose logging** (Diagnostics) — runs yt-dlp with `-v` and writes its full output to
  `~/Library/Logs/MMI Tunes/mmi-tunes.log`. Off by default. Turn it on before reproducing a
  failure, then use **Open log**.

When a download fails, the row shows a short cause plus a **Details** toggle with the raw
yt-dlp output and a **Copy** button — the friendly message is a best guess, the raw output is
the ground truth.

## How it works

```
┌──────────────────────────────────────────────────────────────┐
│  React UI (Tailwind + Zustand)                               │
│  ↕ Wails event bus + auto-generated TS bindings              │
│  Go App struct                                               │
│  ├─ queue.Queue   worker pool, per-job context.CancelFunc    │
│  ├─ settings.Store / history.Store / queue.Save+Load         │
│  └─ downloader.Download                                      │
│      └─ yt-dlp (bundled in Resources/)                       │
│          ├─ ffmpeg + ffprobe (bundled, invoked by yt-dlp)    │
│          └─ qjs (bundled; solves YouTube's JS n-challenge)   │
│      └─ postprocess.ResizeCoverArtInMP3 (Go, bogem/id3v2)    │
└──────────────────────────────────────────────────────────────┘
```

Persistence:

```
~/Library/Application Support/MMI Tunes/
├── settings.json   user preferences
├── history.json    completed video IDs (for dedup + Reveal)
└── jobs.json       full job list (resumes across launches)
```

## Tech stack

- **Backend** — Go 1.23, [Wails v2](https://wails.io), [`bogem/id3v2`](https://github.com/bogem/id3v2), `golang.org/x/image/draw`.
- **Frontend** — React 18, TypeScript, Vite, Tailwind CSS, Zustand.
- **Bundled CLIs** — [`yt-dlp`](https://github.com/yt-dlp/yt-dlp) (universal binary from upstream releases), static [`ffmpeg`](https://ffmpeg.org/) + `ffprobe` from [evermeet.cx](https://evermeet.cx/ffmpeg/), and [`qjs`](https://github.com/quickjs-ng/quickjs) (QuickJS-NG, 1.2 MB). All four live in `MMI Tunes.app/Contents/Resources/`.

### Why a JavaScript runtime is bundled

YouTube requires solving a JavaScript "n signature" challenge before it will return adaptive
audio formats, and yt-dlp delegates that to an external JS engine. With none available it
falls back to the `visionos` client, which answers `UNPLAYABLE` — surfacing as the misleading
`This video is not available` on videos that play fine in a browser. `qjs` is bundled so the
app never depends on the user having deno or a recent-enough Node.

## Development

```bash
# Prereqs (one-time)
go install github.com/wailsapp/wails/v2/cmd/wails@latest
brew install yt-dlp ffmpeg quickjs  # only for `wails dev` and `go test`;
                                    # the .pkg build downloads bundled
                                    # binaries on its own (see below).

# Live-reload dev (Go + React both hot-reload)
wails dev

# Run tests (race detector on)
go test -race ./...
```

### Production build (.pkg installer)

```bash
./scripts/download-tools.sh         # pulls yt-dlp, static ffmpeg/ffprobe,
                                    # and qjs into ./tools/  (gitignored)
./scripts/build-pkg.sh 1.0.0        # → dist/MMI-Tunes-1.0.0.pkg
```

`build-pkg.sh` runs `wails build -platform darwin/arm64`, copies the
four CLI binaries into `MMI Tunes.app/Contents/Resources/`, ad-hoc
re-signs the app (since modifying Resources/ invalidates the signature),
and wraps the result in a `.pkg` via `pkgbuild`.

### Smoke-test CLI

Useful for iterating on the downloader engine without re-launching the GUI:

```bash
go run ./cmd/cli "https://www.youtube.com/watch?v=…" /tmp/mmi-out
```

## Roadmap

Done:

- [x] MMI-compliant downloader (320 / 48 kHz / stereo / cover ≤ 800×800)
- [x] Concurrent worker pool with cancel + retry
- [x] Settings persistence + history dedup
- [x] React UI with progress bars, settings drawer, dedup prompt
- [x] Per-row download / retry / cancel + Clear all
- [x] yt-dlp stderr categorisation (geo, age, Premium, bot-check, network)
- [x] App icon, full `-race` test suite
- [x] Bundled `yt-dlp` + `ffmpeg` + `qjs` (no `brew install` for end users)
- [x] `.pkg` installer + GitHub Releases

Maybe next:

- [ ] Auto-update yt-dlp from inside the app
- [ ] Playlist URLs (one URL → many tracks)
- [ ] M3U playlist file generation
- [ ] Drag-and-drop URLs from a browser
- [ ] macOS notifications when batches finish
- [ ] Apple Developer signed + notarized release (no Gatekeeper warning)

## Uninstall

macOS apps are self-contained — there is no separate uninstaller:

```bash
# The .pkg installs as root:wheel (standard macOS behaviour for pkgs),
# so removing the app from /Applications needs sudo.
sudo rm -rf "/Applications/MMI Tunes.app"

# Optional: also forget settings, history, and the saved job list
# (these are owned by you, no sudo).
rm -rf "$HOME/Library/Application Support/MMI Tunes"

# Optional: also delete the downloaded MP3s.
rm -rf "$HOME/Music/MMI Tunes"
```

You can also drag `MMI Tunes.app` from Finder to Trash — Finder prompts for the password and uses admin privileges automatically.

## License

MIT — see [LICENSE](LICENSE).
