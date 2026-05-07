#!/usr/bin/env bash
# Download yt-dlp, ffmpeg, and ffprobe into tools/ so build-pkg.sh
# can bundle them into MMI Tunes.app. Idempotent — re-running upgrades
# to the latest versions.
#
# Sources:
#   yt-dlp       — github.com/yt-dlp/yt-dlp (universal Mach-O)
#   ffmpeg       — evermeet.cx (canonical static macOS build, since 2009)
#   ffprobe      — evermeet.cx
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
TOOLS="${ROOT}/tools"
mkdir -p "${TOOLS}"
cd "${TOOLS}"

echo "==> Downloading yt-dlp (universal binary)…"
curl -sL --max-time 120 -o yt-dlp \
  https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_macos
chmod +x yt-dlp

echo "==> Downloading ffmpeg (evermeet.cx)…"
curl -sL --max-time 180 -o ffmpeg.zip https://evermeet.cx/ffmpeg/getrelease/zip
unzip -o ffmpeg.zip > /dev/null
rm ffmpeg.zip
chmod +x ffmpeg

echo "==> Downloading ffprobe (evermeet.cx)…"
curl -sL --max-time 180 -o ffprobe.zip https://evermeet.cx/ffmpeg/getrelease/ffprobe/zip
unzip -o ffprobe.zip > /dev/null
rm ffprobe.zip
chmod +x ffprobe

echo
echo "==> Versions:"
./yt-dlp --version
./ffmpeg -version | head -1
./ffprobe -version | head -1
echo
echo "==> Total size:"
du -sh "${TOOLS}"
