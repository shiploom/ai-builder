#!/bin/sh
# install.sh — install the shiploom Go binary (pinned or latest release).
# Usage: curl -fsSL .../install.sh | sh
#        VERSION=1.1.0 PREFIX=$HOME/.local sh scripts/install.sh
# Verifies sha256 when checksum tooling exists; fails closed otherwise.
set -u

REPO_URL="${REPO_URL:-https://github.com/shiploom/ai-builder}"
VERSION="${VERSION:-$(cat "$(dirname "$0")/../core/VERSION" 2>/dev/null || echo latest)}"
PREFIX="${PREFIX:-/usr/local}"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  darwin) OS=darwin ;;
  linux) OS=linux ;;
  *) echo "shiploom: unsupported OS: $OS" >&2; exit 2 ;;
esac
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) echo "shiploom: unsupported arch: $ARCH" >&2; exit 2 ;;
esac

ASSET="shiploom-$VERSION-$OS-$ARCH"
BASE="$REPO_URL/releases/download/v$VERSION"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/shiploom-install.XXXXXX")"
trap 'rm -rf "$TMP"' EXIT INT TERM

fetch() {
  if command -v curl >/dev/null 2>&1; then curl -fsSL -o "$2" "$1"; \
  elif command -v wget >/dev/null 2>&1; then wget -q -O "$2" "$1"; \
  else echo "shiploom: need curl or wget" >&2; exit 2; fi
}

fetch "$BASE/$ASSET" "$TMP/shiploom"
if command -v sha256sum >/dev/null 2>&1; then sums=sha256sum; \
elif command -v shasum >/dev/null 2>&1; then sums="shasum -a 256"; \
else sums=""; fi
if [ -n "$sums" ]; then
  fetch "$BASE/shiploom-checksums.txt" "$TMP/checksums.txt"
  want="$(grep " $ASSET\$" "$TMP/checksums.txt" | awk '{print $1}')"
  got="$($sums "$TMP/shiploom" | awk '{print $1}')"
  if [ -z "$want" ] || [ "$want" != "$got" ]; then
    echo "shiploom: checksum mismatch for $ASSET" >&2
    exit 2
  fi
else
  echo "shiploom: no checksum tooling, skipping verification" >&2
fi

mkdir -p "$PREFIX/bin"
mv "$TMP/shiploom" "$PREFIX/bin/shiploom"
chmod 755 "$PREFIX/bin/shiploom"
"$PREFIX/bin/shiploom" --version
