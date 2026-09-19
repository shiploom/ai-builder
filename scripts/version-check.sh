#!/bin/sh
# version-check.sh — dual-ship version-parity gate (P6 slice 1),
# Go-only since v1.3.0 (Python implementation removed; the pyproject
# assertion went with the Python packaging).
# Usage: sh scripts/version-check.sh [go-binary] [tag]
#   go-binary defaults to a fresh stamped build at $TMPDIR/shiploom-version.
#   tag defaults to v$(cat core/VERSION) (release.yml passes github.ref_name).
# Asserts: tag == core/VERSION == wrappers/npx/package.json
# and the `core X` field of `shiploom --version`. Fail-closed.
set -u

REPO="$(cd "$(dirname "$0")/.." && pwd)"
GO_BIN="${1:-}"
TAG="${2:-v$(cat "$REPO/core/VERSION")}"

if [ -z "$GO_BIN" ]; then
  GO_BIN="${TMPDIR:-/tmp}/shiploom-version"
  sh "$REPO/scripts/build-go.sh" "$GO_BIN" || exit 2
fi

fail() { echo "version-check: FAIL: $1" >&2; exit 2; }

core="$(cat "$REPO/core/VERSION")"
npx="$(sed -n 's/^  "version": "\(.*\)",$/\1/p' "$REPO/wrappers/npx/package.json" | head -n 1)"
[ -n "$core" ] || fail "empty core/VERSION"
[ -n "$npx" ] || fail "no version in wrappers/npx/package.json"
[ "v$core" = "$TAG" ] || fail "tag $TAG != v$core (core/VERSION)"
[ "$npx" = "$core" ] || fail "npx $npx != core $core"

go_core="$(SHIPLOOM_SCHEMAS="$REPO/schemas" "$GO_BIN" --version 2>/dev/null | \
  sed -n 's/.*(core \([^,]*\),.*/\1/p')"
[ -n "$go_core" ] || fail "could not parse go --version"
[ "$go_core" = "$core" ] || fail "go core $go_core != $core"

echo "version-check: OK ($TAG; npx + Go CLI agree on core $core)"
