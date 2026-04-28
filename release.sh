#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-}"
if [[ -z "$VERSION" ]]; then
  echo "Usage: $0 <version>  (e.g. $0 v1.0.0)"
  exit 1
fi

WAILS="${HOME}/go/bin/wails"
OUTDIR="dist"

mkdir -p "$OUTDIR"

echo "==> Building Linux amd64 ($VERSION)..."
"$WAILS" build -platform linux/amd64 -tags webkit2_41

LINUX_ARCHIVE="$OUTDIR/NET-linux-amd64-${VERSION}.tar.gz"
tar -czf "$LINUX_ARCHIVE" -C build/bin NET
echo "    $LINUX_ARCHIVE ($(du -sh "$LINUX_ARCHIVE" | cut -f1))"

echo "==> Building Windows amd64 ($VERSION)..."
"$WAILS" build -platform windows/amd64
WIN_ARCHIVE="$OUTDIR/NET-windows-amd64-${VERSION}.zip"
zip -j "$WIN_ARCHIVE" build/bin/NET.exe
echo "    $WIN_ARCHIVE ($(du -sh "$WIN_ARCHIVE" | cut -f1))"

if [[ "$(uname)" == "Darwin" ]]; then
  echo "==> Building macOS universal ($VERSION)..."
  "$WAILS" build -platform darwin/universal
  MAC_ARCHIVE="$OUTDIR/NET-macos-universal-${VERSION}.zip"
  zip -r "$MAC_ARCHIVE" build/bin/NET.app
  echo "    $MAC_ARCHIVE ($(du -sh "$MAC_ARCHIVE" | cut -f1))"
else
  echo "    Skipping macOS build (cross-compilation not supported by Wails — run on a Mac)"
fi

echo ""
echo "Release artifacts in $OUTDIR/:"
ls -lh "$OUTDIR/"
