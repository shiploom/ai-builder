#!/bin/sh
# run.sh — golden parity harness: Python reference vs Go binary.
# Usage: sh tests/parity/run.sh <go-binary> [case...]
# Each tests/parity/cases/<name>/ holds: args.txt (argv, one line),
# exit.txt (expected exit code), stdout.txt + stderr.txt (expected, or
# stdout.contains.txt / stderr.contains.txt for needle matching).
# Both outputs pass through normalize.sed first (versions, runtimes).
# Exit nonzero on any mismatch. No network. Reference needs no install
# (stdlib-only cli/shiploom.py run from the repo root).
set -u

REPO="$(cd "$(dirname "$0")/../.." && pwd)"
GO_BIN="${1:-}"; shift 2>/dev/null || true
if [ -z "$GO_BIN" ]; then echo "usage: sh tests/parity/run.sh <go-binary> [case...]" >&2; exit 2; fi
PYBIN="${PY_SHIPLOOM_BIN:-python3}"

CASES_DIR="$REPO/tests/parity/cases"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/shiploom-parity.XXXXXX")"
trap 'rm -rf "$TMP"' EXIT INT TERM
FAIL=0; COUNT=0

norm() { sed -E -f "$CASES_DIR/normalize.sed" "$1"; }

run_one() {
  name="$1"; dir="$CASES_DIR/$name"
  COUNT=$((COUNT + 1))
  # shellcheck disable=SC2086
  set -- $(cat "$dir/args.txt" 2>/dev/null || true)
  want_exit="$(cat "$dir/exit.txt")"
  (cd "$REPO" && "$PYBIN" cli/shiploom.py "$@" >"$TMP/py.out" 2>"$TMP/py.err"; echo "$?" >"$TMP/py.exit")
  "$GO_BIN" "$@" >"$TMP/go.out" 2>"$TMP/go.err"; echo "$?" >"$TMP/go.exit"
  norm "$TMP/py.out" >"$TMP/py.norm"; norm "$TMP/go.out" >"$TMP/go.norm"
  norm "$TMP/py.err" >"$TMP/pye.norm"; norm "$TMP/go.err" >"$TMP/goe.norm"
  ok=1
  [ "$(cat "$TMP/py.exit")" = "$want_exit" ] || { ok=0; echo "FAIL $name: py exit $(cat "$TMP/py.exit") != $want_exit"; }
  [ "$(cat "$TMP/go.exit")" = "$want_exit" ] || { ok=0; echo "FAIL $name: go exit $(cat "$TMP/go.exit") != $want_exit"; }
  if [ -f "$dir/stdout.contains.txt" ]; then
    grep -qFf "$dir/stdout.contains.txt" "$TMP/go.norm" || { ok=0; echo "FAIL $name: go stdout missing needle"; }
    grep -qFf "$dir/stdout.contains.txt" "$TMP/py.norm" || { ok=0; echo "FAIL $name: py stdout missing needle (case bug)"; }
  else
    cmp -s "$TMP/py.norm" "$TMP/go.norm" || { ok=0; echo "FAIL $name: stdout differs"; diff "$TMP/py.norm" "$TMP/go.norm" | head -n 10; }
  fi
  if [ -f "$dir/stderr.contains.txt" ]; then
    grep -qFf "$dir/stderr.contains.txt" "$TMP/goe.norm" || { ok=0; echo "FAIL $name: go stderr missing needle"; }
    grep -qFf "$dir/stderr.contains.txt" "$TMP/pye.norm" || { ok=0; echo "FAIL $name: py stderr missing needle (case bug)"; }
  else
    cmp -s "$TMP/pye.norm" "$TMP/goe.norm" || { ok=0; echo "FAIL $name: stderr differs"; diff "$TMP/pye.norm" "$TMP/goe.norm" | head -n 10; }
  fi
  if [ "$ok" = "1" ]; then echo "PASS $name"; else FAIL=$((FAIL + 1)); fi
}

if [ "$#" -gt 0 ]; then
  for c in "$@"; do run_one "$c"; done
else
  for d in "$CASES_DIR"/*/; do
    [ -d "$d" ] || continue
    run_one "$(basename "$d")"
  done
fi
echo "parity: $((COUNT - FAIL))/$COUNT pass"
[ "$FAIL" = "0" ]
