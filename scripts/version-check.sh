#!/bin/sh
# version-check.sh — dual-ship version-parity gate (P6 slice 1).
# Usage: sh scripts/version-check.sh [go-binary] [tag]
#   go-binary defaults to a fresh stamped build at $TMPDIR/shiploom-version.
#   tag defaults to v$(cat core/VERSION) (release.yml passes github.ref_name).
# Asserts: tag == core/VERSION == pyproject.toml == wrappers/npx/package.json
# and the `core X` field of BOTH `shiploom --version` outputs. Fail-closed.
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
pkg="$(sed -n 's/^version = "\(.*\)"$/\1/p' "$REPO/pyproject.toml" | head -n 1)"
npx="$(sed -n 's/^  "version": "\(.*\)",$/\1/p' "$REPO/wrappers/npx/package.json" | head -n 1)"
[ -n "$core" ] || fail "empty core/VERSION"
[ -n "$pkg" ] || fail "no version in pyproject.toml"
[ -n "$npx" ] || fail "no version in wrappers/npx/package.json"
[ "v$core" = "$TAG" ] || fail "tag $TAG != v$core (core/VERSION)"
[ "$pkg" = "$core" ] || fail "pyproject $pkg != core $core"
[ "$npx" = "$core" ] || fail "npx $npx != core $core"

py_core="$(python3 "$REPO/cli/shiploom.py" --version 2>/dev/null | \
  sed -n 's/.*(core \([^,]*\),.*/\1/p')"
go_core="$(SHIPLOOM_SCHEMAS="$REPO/schemas" "$GO_BIN" --version 2>/dev/null | \
  sed -n 's/.*(core \([^,]*\),.*/\1/p')"
[ -n "$py_core" ] || fail "could not parse python --version"
[ -n "$go_core" ] || fail "could not parse go --version"
[ "$py_core" = "$core" ] || fail "python core $py_core != $core"
[ "$go_core" = "$core" ] || fail "go core $go_core != $core"

echo "version-check: OK ($TAG; npx + both CLIs agree on core $core)"
