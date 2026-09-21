#!/usr/bin/env bash
# Download yt-dlp, ffmpeg, ffprobe, and qjs into tools/ so build-pkg.sh
# can bundle them into MMI Tunes.app. Idempotent — re-running upgrades
# to the latest versions.
#
# Sources:
#   yt-dlp       — github.com/yt-dlp/yt-dlp (universal Mach-O)
#   ffmpeg       — osxexperts.net (static arm64 macOS build; evermeet.cx,
#                  the previous source, publishes x86_64 only)
#   ffprobe      — osxexperts.net
#   qjs          — github.com/quickjs-ng/quickjs (JS runtime yt-dlp needs to
#                  solve YouTube's n-signature challenge; without it YouTube
#                  returns no audio formats at all)
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
TOOLS="${ROOT}/tools"
mkdir -p "${TOOLS}"
cd "${TOOLS}"

echo "==> Downloading yt-dlp (universal binary)…"
curl -sL --max-time 120 -o yt-dlp \
  https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_macos
chmod +x yt-dlp

# osxexperts.net encodes the FFmpeg major version in the filename rather
# than serving a "latest" alias, so these two URLs need bumping on each
# major release.
echo "==> Downloading ffmpeg (osxexperts.net, arm64)…"
curl -sL --max-time 180 -o ffmpeg.zip https://www.osxexperts.net/ffmpeg9arm.zip
unzip -o ffmpeg.zip > /dev/null
rm ffmpeg.zip
chmod +x ffmpeg

echo "==> Downloading ffprobe (osxexperts.net, arm64)…"
curl -sL --max-time 180 -o ffprobe.zip https://www.osxexperts.net/ffprobe9arm.zip
unzip -o ffprobe.zip > /dev/null
rm ffprobe.zip
chmod +x ffprobe

echo "==> Downloading qjs (quickjs-ng, arm64)…"
curl -sL --max-time 120 -o qjs \
  https://github.com/quickjs-ng/quickjs/releases/latest/download/qjs-darwin-arm64
chmod +x qjs

# The app ships darwin/arm64 only, so an x86_64 tool runs only under
# Rosetta — and on a Mac without it yt-dlp reports the binary as missing
# rather than as unrunnable, which surfaces to the user as a bogus
# "install ffmpeg" error. Refuse to bundle one.
echo
echo "==> Verifying architectures…"
for tool in yt-dlp ffmpeg ffprobe qjs; do
  archs=$(lipo -archs "./${tool}")
  if [[ " ${archs} " != *" arm64 "* ]]; then
    echo "ERROR: ${tool} is ${archs}, not arm64." >&2
    exit 1
  fi
  echo "    ${tool}: ${archs}"
done

echo
echo "==> Versions:"
./yt-dlp --version
./ffmpeg -version | head -1
./ffprobe -version | head -1
echo "qjs      $(./qjs --version)"
echo
echo "==> Total size:"
du -sh "${TOOLS}"
