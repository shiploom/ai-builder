#!/bin/sh
# playground.sh — run the AC demo inside a disposable Docker container.
# Proves the zero-runtime story on a bare image: Python + git only.
# Usage: sh scripts/playground.sh [--python 3.12]
# Exit 0 on green demo, 2 on demo failure, 1 when Docker is unavailable.
set -u

PYVER="3.12"
if [ "${1:-}" = "--python" ] && [ -n "${2:-}" ]; then PYVER="$2"; fi
REPO="$(cd "$(dirname "$0")/.." && pwd)"

if ! command -v docker >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; then
  echo "playground needs a running Docker daemon (not detected)" >&2
  exit 1
fi

docker run --rm -v "$REPO:/work:ro" -w /tmp/demo "python:$PYVER-slim" \
  sh -c "pip install -q -e /work && cp -r /work/examples/stack-python /tmp/demo \
  && cd /tmp/demo && shiploom init --green >/dev/null \
  && SHIP_PY=\$(command -v python) sh /work/scripts/ac-demo.sh"
