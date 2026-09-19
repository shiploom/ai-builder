#!/bin/sh
# collect-dashboard.sh — cost/escape dashboard collector (contract:
# docs/cost-dashboard.md). jq port of scripts/collect-dashboard.py,
# removed with the Python implementation in v1.3.0.
#
# Usage: sh scripts/collect-dashboard.sh [project-dir] [--json]
# Exit 0 always (missing inputs warn, not fail). Requires jq.
set -u

ROOT="."
JSON=0
for arg in "$@"; do
  case "$arg" in
    --json) JSON=1 ;;
    -*) ;;
    *) [ "$ROOT" = "." ] && ROOT="$arg" ;;
  esac
done

warns=""
warn() { warns="$warns$1
"; }

MANIFEST="$ROOT/.shiploom/manifest.json"
if [ ! -f "$MANIFEST" ]; then
  warn "no manifest: [Errno 2] No such file or directory: '$MANIFEST'"
  dashboard='{"workflow":null,"steps":{"done":0,"total":0},"retries":{},"gates":{},"budgets":{},"auditEvents":0,"gateReport":null,"defectEscapes":null,"mergeStats":null,"verifierCatchRate":null,"warnings":[]}'
elif ! jq empty "$MANIFEST" 2>/dev/null; then
  warn "no manifest: $MANIFEST unreadable or invalid JSON"
  dashboard='{"workflow":null,"steps":{"done":0,"total":0},"retries":{},"gates":{},"budgets":{},"auditEvents":0,"gateReport":null,"defectEscapes":null,"mergeStats":null,"verifierCatchRate":null,"warnings":[]}'
else
  dashboard="$(jq -c '{
    workflow: .workflow,
    steps: ((.steps // {}) | {done: ([.[] | select(.state == "done")] | length), total: length}),
    retries: (.retries // {}),
    gates: ((.gates // {}) | with_entries(.value = .value.state)),
    budgets: (.budgets // {}),
    auditEvents: 0,
    gateReport: null,
    defectEscapes: null,
    mergeStats: null,
    verifierCatchRate: null,
    warnings: []
  }' "$MANIFEST")"
  AUDIT="$ROOT/.shiploom/audit.jsonl"
  if [ -f "$AUDIT" ]; then
    n="$(grep -c '[^[:space:]]' "$AUDIT" 2>/dev/null)"
    n="${n:-0}"
    dashboard="$(printf '%s' "$dashboard" | jq -c --argjson n "$n" '.auditEvents = $n')"
  else
    warn "no audit log: $AUDIT unreadable"
  fi
  REPORT="$ROOT/verification/gate-report.json"
  if [ -f "$REPORT" ]; then
    if jq empty "$REPORT" 2>/dev/null; then
      dashboard="$(printf '%s' "$dashboard" | jq -c --slurpfile r "$REPORT" '
        ($r[0]) as $rep
        | .gateReport = {verdict: $rep.verdict, gates: (($rep.gates // {}) | with_entries(.value = .value.status))}')"
    else
      warn "unreadable gate report"
    fi
  else
    warn "no verification/gate-report.json (run shiploom verify --report)"
  fi
  warn "defectEscapes: needs post-merge defect tracking (pilot)"
  warn "mergeStats: needs maintainer-merge records (pilot)"
  warn "verifierCatchRate: needs recorded verifier outcomes (pilot)"
fi

if [ -n "$warns" ]; then
  wjson="$(printf '%s' "$warns" | jq -c -R -s 'split("\n") | map(select(length > 0))')"
  dashboard="$(printf '%s' "$dashboard" | jq -c --argjson w "$wjson" '.warnings = $w')"
fi

if [ "$JSON" = "1" ]; then
  printf '%s' "$dashboard" | jq -S .
else
  line="$(printf '%s' "$dashboard" | jq -r '"dashboard: workflow=\(.workflow // "None") steps=\(.steps.done)/\(.steps.total) audit events=\(.auditEvents)", "gate verdict: \(.gateReport.verdict // "no report")"')"
  printf '%s\n' "$line"
  printf '%s' "$dashboard" | jq -r '.warnings[] | "  note: \(.)"'
fi
exit 0
