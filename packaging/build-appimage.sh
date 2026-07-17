#!/usr/bin/env bash
# Build a Linux x86-64 AppImage for EveTrace.
#
# Assembles an AppDir (binary + .desktop + icon + AppRun) and runs appimagetool.
# appimagetool is downloaded on first use into dist/tools/ and cached there.
#
# Invoked by `make appimage`, which sets:
#   BIN      path to the built linux-amd64 binary   (default dist/evetrace-linux-amd64)
#   OUT      output directory for the .AppImage      (default dist)
#   VERSION  version string for the filename         (default dev)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="${BIN:-dist/evetrace-linux-amd64}"
OUT="${OUT:-dist}"
VERSION="${VERSION:-dev}"
ARCH="x86_64"

cd "$ROOT"

if [ ! -f "$BIN" ]; then
	echo "error: binary not found at $BIN (run 'make linux-amd64' first)" >&2
	exit 1
fi

APPDIR="$OUT/EveTrace.AppDir"
rm -rf "$APPDIR"
mkdir -p "$APPDIR/usr/bin"

# Binary
install -m 0755 "$BIN" "$APPDIR/usr/bin/evetrace"

# Desktop entry + icon (appimagetool wants both at the AppDir root, with the
# icon filename matching the desktop file's Icon= key).
install -m 0644 packaging/linux/evetrace.desktop "$APPDIR/evetrace.desktop"
install -m 0644 packaging/linux/evetrace.png     "$APPDIR/evetrace.png"
# A .DirIcon is also conventional for file-manager thumbnails.
cp packaging/linux/evetrace.png "$APPDIR/.DirIcon"

# AppRun launcher
install -m 0755 packaging/linux/AppRun "$APPDIR/AppRun"

# Fetch appimagetool once, cache it under dist/tools.
TOOLS="$OUT/tools"
TOOL="$TOOLS/appimagetool-$ARCH.AppImage"
if [ ! -x "$TOOL" ]; then
	mkdir -p "$TOOLS"
	echo "downloading appimagetool..."
	curl -fsSL -o "$TOOL" \
		"https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-$ARCH.AppImage"
	chmod +x "$TOOL"
fi

OUTFILE="$OUT/EveTrace-$VERSION-$ARCH.AppImage"
echo "building $OUTFILE"
# APPIMAGE_EXTRACT_AND_RUN lets appimagetool run without FUSE (e.g. in CI/containers).
ARCH="$ARCH" APPIMAGE_EXTRACT_AND_RUN=1 "$TOOL" "$APPDIR" "$OUTFILE"

rm -rf "$APPDIR"
echo "done: $OUTFILE"
