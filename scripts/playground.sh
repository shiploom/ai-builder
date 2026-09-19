#!/bin/sh
# playground.sh — run the AC demo inside a disposable Docker container.
# Proves the zero-runtime story on a bare image: Go toolchain + git + jq,
# plus a pytest-bearing python3 purely as operator tooling for the
# auto-detected test gates (like node for the node leg).
# (ac-demo builds the binary itself when SHIPLOOM_GO_BIN is unset).
# Usage: sh scripts/playground.sh [--go 1.24]
# Exit 0 on green demo, 2 on demo failure, 1 when Docker is unavailable.
set -u

GOVER="1.24"
if [ "${1:-}" = "--go" ] && [ -n "${2:-}" ]; then GOVER="$2"; fi
REPO="$(cd "$(dirname "$0")/.." && pwd)"

if ! command -v docker >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; then
  echo "playground needs a running Docker daemon (not detected)" >&2
  exit 1
fi

docker run --rm -v "$REPO:/work:ro" -w /tmp/demo "golang:$GOVER-bookworm" \
  sh -c "apt-get update -qq && apt-get install -y -qq jq python3 python3-pip >/dev/null && pip install -q --break-system-packages pytest && sh /work/scripts/ac-demo.sh"
