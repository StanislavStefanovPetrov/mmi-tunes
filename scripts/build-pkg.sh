#!/usr/bin/env bash
# Build MMI Tunes.app with bundled yt-dlp / ffmpeg / ffprobe and wrap
# the result in a .pkg installer. The .pkg drops the .app under
# /Applications/ on the user's machine — no drag-to-/Applications,
# no `brew install` for dependencies.
#
# Usage:  ./scripts/build-pkg.sh
# Output: dist/MMI-Tunes-<version>.pkg
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

VERSION=${1:-1.0.0}
APP_NAME="MMI Tunes"
BUNDLE_ID="com.stanislavpetrov.mmi-tunes"
APP_PATH="build/bin/${APP_NAME}.app"
DIST_DIR="dist"
PKG_PATH="${DIST_DIR}/MMI-Tunes-${VERSION}.pkg"

# --- Sanity: bundled tools must be present ---
for tool in yt-dlp ffmpeg ffprobe; do
  if [[ ! -x "tools/${tool}" ]]; then
    echo "ERROR: tools/${tool} missing or not executable." >&2
    echo "Run scripts/download-tools.sh first." >&2
    exit 1
  fi
done

echo "==> Building Wails app (darwin/arm64)…"
export PATH="$PATH:$(go env GOPATH)/bin"
wails build -platform darwin/arm64 -clean

echo "==> Bundling tools into ${APP_PATH}/Contents/Resources/…"
RES="${APP_PATH}/Contents/Resources"
cp tools/yt-dlp "${RES}/yt-dlp"
cp tools/ffmpeg "${RES}/ffmpeg"
cp tools/ffprobe "${RES}/ffprobe"
chmod +x "${RES}/yt-dlp" "${RES}/ffmpeg" "${RES}/ffprobe"

# Strip macOS quarantine xattrs so the binaries run without prompting
# (the .pkg installer would fail signature checks otherwise).
xattr -cr "${APP_PATH}" 2>/dev/null || true

echo "==> Re-signing app (ad-hoc, since we modified Resources/)…"
codesign --force --deep --sign - "${APP_PATH}"

echo "==> App size:"
du -sh "${APP_PATH}"

echo "==> Building .pkg…"
mkdir -p "${DIST_DIR}"
pkgbuild \
  --root "build/bin" \
  --identifier "${BUNDLE_ID}" \
  --version "${VERSION}" \
  --install-location "/Applications" \
  "${PKG_PATH}"

echo
echo "✅  Built ${PKG_PATH}"
echo "   $(du -h "${PKG_PATH}" | cut -f1)"
echo
echo "Install with: open '${PKG_PATH}'"
