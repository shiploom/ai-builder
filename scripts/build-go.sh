#!/bin/sh
# build-go.sh — build the Go shiploom binary with the core version stamped in.
# Usage: sh scripts/build-go.sh [output-path]   (default: ./dist/shiploom)
# Requires: go toolchain. No network (stdlib only, no module downloads).
set -u

REPO="$(cd "$(dirname "$0")/.." && pwd)"
OUT="${1:-$REPO/dist/shiploom}"
VERSION="$(cat "$REPO/core/VERSION")"
GO_BIN="${GO_BIN:-go}"

mkdir -p "$(dirname "$OUT")"
cd "$REPO" || exit 2
"$GO_BIN" build -trimpath \
  -ldflags "-X github.com/shiploom/ai-builder/internal/version.CoreVersion=$VERSION" \
  -o "$OUT" ./cmd/shiploom
