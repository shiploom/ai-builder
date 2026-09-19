#!/bin/sh
# ac-demo.sh — acceptance demo (MASTER_SPEC 8.5, BUILD_PLAN PR10).
#
# Exercises AC1-AC6 against the real Go CLI in scratch dirs (/tmp):
#   AC1  idea -> verified MVP on 2 stacks (default + node*) x 2 harnesses
#        (claude + opencode adapter output, validated)
#   AC2  brownfield scoped change with approvals, zero unapproved merges
#   AC3  seeded defects (broken build, redefined acceptance) are caught
#   AC4  kill -9 mid-verify loses nothing; resume never redoes done steps
#   AC5  validate/status/doctor run offline (offline-by-construction:
#        stdlib-only Go build, no network use)
#   AC6  destructive actions denied (exit 3), denied gates halt (exit 2),
#        everything audited
# *node leg skips gracefully when node is unavailable.
#
# File drops stand in for harness LLM output; every gate, lock, approval,
# and ordering constraint enforced here is the real production machinery.
# Requires jq (config patching + JSON checks). SHIPLOOM_GO_BIN may point at
# a prebuilt binary; otherwise it is built into the scratch dir.
# Auto-detected test gates run through the operator PATH, so put a
# pytest-bearing python3 first (e.g. PATH="$REPO/.venv/bin:$PATH").
set -u

REPO="$(cd "$(dirname "$0")/.." && pwd)"
case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*)
    # Git Bash paths (/c/...) are unreadable to the native exe: use the
    # Windows form for everything handed to it via the environment.
    REPO="$(cd "$(dirname "$0")/.." && pwd -W)" ;;
esac
command -v jq >/dev/null 2>&1 || { echo "ac-demo needs jq on PATH" >&2; exit 2; }
T="$REPO/core/artifacts-templates"
PASS=0; FAIL=0; SKIPP=0
WORK="$(mktemp -d "${TMPDIR:-/tmp}/shiploom-ac.XXXXXX")"
trap 'rm -rf "$WORK"; kill %1 2>/dev/null; true' EXIT INT TERM
if [ -z "${SHIPLOOM_GO_BIN:-}" ]; then
  SHIPLOOM_GO_BIN="$WORK/shiploom-go"
  sh "$REPO/scripts/build-go.sh" "$SHIPLOOM_GO_BIN" || exit 2
fi

ship() { SHIPLOOM_SCHEMAS="$REPO/schemas" "$SHIPLOOM_GO_BIN" "$@"; }
step() { printf '\n### %s\n' "$1"; }
ok() { PASS=$((PASS + 1)); printf '  PASS %s\n' "$1"; }
bad() { FAIL=$((FAIL + 1)); printf '  FAIL %s\n' "$1"; }
skip() { SKIPP=$((SKIPP + 1)); printf '  SKIP %s\n' "$1"; }
expect() { # expect <want-exit> <label> -- <cmd...>
  want="$1"; label="$2"; shift 2
  [ "$1" = "--" ] && shift
  out="$WORK/out.txt"
  "$@" >"$out" 2>&1; rc=$?
  if [ "$rc" = "$want" ]; then ok "$label (exit $rc)"; else bad "$label (want $want, got $rc)"; cat "$out"; fi
}
expect_out() { # expect_out <label> <needle> (needle must appear in last output)
  if grep -qF "$2" "$WORK/out.txt"; then ok "$1"; else bad "$1 (missing: $2)"; cat "$WORK/out.txt"; fi
}
patch_test_gate() { # patch_test_gate <project> <command...>
  PROJ="$1"; shift
  jq --arg cmd "$*" '.gates.test = {"command": $cmd, "timeoutS": 120}' \
    "$PROJ/.shiploom/config.json" > "$PROJ/.shiploom/config.json.tmp" \
    && mv "$PROJ/.shiploom/config.json.tmp" "$PROJ/.shiploom/config.json"
}
write_gate_sh() { # write_gate_sh <project> <exit-code>
  printf '#!/bin/sh\nexit %s\n' "$2" > "$1/tests/gate.sh"
  chmod +x "$1/tests/gate.sh"
}
write_twin() { # write_twin <project> <id> <acc...>
  proj="$1"; vid="$2"; shift 2
  results=""
  for acc in "$@"; do
    results="$results{\"acceptanceId\": \"$acc\", \"result\": \"pass\", \"evidence\": \"oracle log\", \"oracleUsed\": true},"
  done
  cat > "$proj/verification/verification-report.json" <<EOF2
{"id": "$vid", "attemptRef": "diff:demo@abc", "verifier": "agent:verifier:fresh-demo",
 "results": [${results%,}], "gatesSummary": {"test": "pass", "secrets": "pass"},
 "verdict": "pass", "timestamp": "2026-09-17T10:00:00Z"}
EOF2
}
drop_lite_files() { # drop_lite_files <project> (harness-simulated artifacts)
  p="$1"
  mkdir -p "$p/research" "$p/product" "$p/architecture" "$p/acceptance" \
           "$p/implementation" "$p/verification" "$p/.shiploom/.oracle/oracle"
  cp "$T/research/market.md" "$T/research/evidence.md" "$p/research/"
  cp "$T/product/requirements.md" "$p/product/"
  cp "$T/architecture/decisions.md" "$T/architecture/architecture.md" "$p/architecture/"
  cp "$T/acceptance/example.json" "$p/acceptance/"
  cp "$T/implementation/attempt-log.md" "$p/implementation/"
  cp "$T/verification/verification-report.md" "$p/verification/"
  printf '#!/bin/sh\nexit 0\n' > "$p/.shiploom/.oracle/oracle/ACC-001.sh"
  printf '#!/bin/sh\nexit 0\n' > "$p/.shiploom/.oracle/oracle/ACC-002.sh"
}

step "AC5 offline commands on the repo itself"
cd "$REPO"
expect 0 "validate --strict offline" -- ship validate --strict .
expect 0 "trace builds offline" -- ship trace REQ-001
expect 0 "doctor offline" -- ship doctor

step "AC1 greenfield default stack: init + adapters (2 harnesses)"
P="$WORK/ac1py"; mkdir -p "$P"; cp -r "$REPO/examples/stack-python/"* "$P/"
cd "$P" && expect 0 "init green" -- ship init --green --harness auto
expect 0 "adapters generate all" -- ship adapters --generate all
[ -f "$P/AGENTS.md" ] && [ -f "$P/CLAUDE.md" ] && ok "AGENTS.md + CLAUDE.md" || bad "harness facts files"
[ "$(find "$P/.claude/skills" "$P/.opencode/skills" -name SKILL.md | wc -l | tr -d ' ')" = "$(( $(ls "$REPO/core/skills" | wc -l | tr -d ' ') * 2 ))" ] \
  && ok "$(ls "$REPO/core/skills" | wc -l | tr -d ' ') skills x 2 harnesses" || bad "skill count"
[ -f "$P/.claude/settings.json" ] && [ -f "$P/opencode.json" ] \
  && [ -x "$P/.claude/hooks/shiploom-guard.py" ] \
  && ok "settings.json + opencode.json + guard hook" || bad "harness config files"
expect 0 "conformance all harnesses" -- ship conformance --harness all
expect 0 "generated tree strict-clean" -- ship validate --strict .
mkdir -p "$P/tests"; write_gate_sh "$P" 0
patch_test_gate "$P" sh tests/gate.sh
expect 0 "verify default stack" -- ship verify

step "AC1 greenfield default stack: full lite run to Done"
drop_lite_files "$P"
expect 0 "run pauses at scope gate" -- ship run greenfield-full-lite
expect_out "scope pause message" "awaiting approval: scope-approval"
expect 0 "approve scope" -- ship approve scope-approval --actor human:nadia
expect 0 "approve arch" -- ship approve arch-approval
expect 0 "run pauses for lock" -- ship run greenfield-full-lite
expect_out "lock pause message" "acceptance not locked yet"
expect 0 "lock acceptance" -- ship lock
expect 0 "run pauses for missing twin" -- ship run greenfield-full-lite
expect_out "twin pause message" "verifier report twin missing"
write_twin "$P" VR-100 ACC-001 ACC-002
expect 0 "run reaches merge gate" -- ship run greenfield-full-lite
expect_out "merge pause message" "awaiting approval: merge-approval"
expect 0 "approve merge" -- ship approve merge-approval
expect 0 "workflow completes" -- ship run greenfield-full-lite
expect_out "complete message" "workflow greenfield-full-lite complete"

step "AC1 node leg (conditional)"
if command -v node >/dev/null 2>&1; then
  N="$WORK/ac1node"; mkdir -p "$N"; cp -r "$REPO/examples/stack-node/"* "$N/"
  cd "$N" && expect 0 "init node project" -- ship init --green --stack node
  expect 0 "adapters claude+opencode" -- ship adapters --generate all
  patch_test_gate "$N" npm test
  expect 0 "verify node stack" -- ship verify --gates test,secrets,compile
else
  skip "node runtime unavailable"
fi

step "AC2 brownfield scoped change with approvals"
B="$WORK/ac2"; mkdir -p "$B"; cp -r "$REPO/examples/brownfield-sample/"* "$B/"
cd "$B" && expect 0 "init existing" -- ship init --existing --harness auto
expect 0 "adapters generate all" -- ship adapters --generate all
mkdir -p "$B/brownfield" "$B/implementation" "$B/verification"
cp "$T/brownfield/repo-map.md" "$B/brownfield/"
expect 0 "run maps then pauses" -- ship run brownfield-fix
expect_out "map pause message" "awaiting approval: map-approval"
expect 0 "approve map" -- ship approve map-approval --actor human:priya
cp "$T/brownfield/impact-plan.md" "$B/brownfield/"
expect 0 "run impacts then pauses" -- ship run brownfield-fix
expect 0 "approve plan" -- ship approve plan-approval
cp "$T/implementation/attempt-log.md" "$B/implementation/"
cp "$T/verification/verification-report.md" "$B/verification/"
write_twin "$B" VR-200 TEST-001
expect 0 "run reaches merge gate" -- ship run brownfield-fix
expect 0 "approve merge" -- ship approve merge-approval --reason "diff reviewed"
expect 0 "brownfield completes" -- ship run brownfield-fix
expect_out "brownfield complete" "workflow brownfield-fix complete"
grep -q "approve.merge-approval" "$B/.shiploom/audit.jsonl" \
  && ok "merge approval audited (no silent merge)" || bad "merge audit entry"

step "AC3 seeded defects are caught"
cd "$P"
cp tests/gate.sh "$WORK/gate.good"
write_gate_sh "$P" 1
expect 2 "broken build fails verify" -- ship verify --gates test
cp "$WORK/gate.good" tests/gate.sh
expect 0 "fixed build passes verify" -- ship verify --gates test
sed -i.bak 's/60 seconds/61 seconds/' acceptance/example.json; rm acceptance/example.json.bak
expect 2 "redefined acceptance fails lock check" -- ship lock --check
expect_out "redefinition message" "redefined after lock"

step "AC4 kill -9 loses nothing; resume never redoes"
K="$WORK/ac4"; mkdir -p "$K"; cp -r "$REPO/examples/stack-python/"* "$K/"
cd "$K" && ship init --green >/dev/null
patch_test_gate "$K" sleep 60
ship verify --gates test >/dev/null 2>&1 & pid=$!
sleep 2; kill -9 "$pid" 2>/dev/null; wait "$pid" 2>/dev/null; true
jq empty .shiploom/manifest.json && jq empty .shiploom/config.json \
  && ok "manifest+config intact after kill -9" || bad "state corrupt after kill -9"
mkdir -p "$K/tests"; write_gate_sh "$K" 0
patch_test_gate "$K" sh tests/gate.sh
expect 0 "verify passes after resume" -- ship verify --gates test
cd "$P"
[ "$(grep -c '"target": "research"' .shiploom/audit.jsonl)" = "1" ] \
  && ok "research completed exactly once (no redo)" || bad "step redo detected"

step "AC6 denies and audit"
D="$WORK/ac6"; mkdir -p "$D"; cd "$D" && ship init --green >/dev/null
mkdir -p .shiploom/workflows
cat > .shiploom/workflows/dep.md <<'EOF3'
---
name: dep
version: 1.0.0
kind: sequential
resume: true
budgets:
  tokens: 100
  spendUSD: 1
  wallClockH: 1
steps:
  - id: nuke
    consumes: [idea.md]
    produces: [idea.md]
    gate: policy
    action: db.destroy
    resource: prod
---

Body.
EOF3
expect 3 "destructive action denied" -- ship run dep
expect_out "deny attribution" "policy deny"
grep -q "run.policy.deny" .shiploom/audit.jsonl && ok "deny audited" || bad "deny audit entry"
G="$WORK/ac6gated"; mkdir -p "$G"; cd "$G" && ship init --green >/dev/null
mkdir -p .shiploom/workflows
cat > .shiploom/workflows/gated.md <<'EOF4'
---
name: gated
version: 1.0.0
kind: sequential
resume: true
budgets:
  tokens: 100
  spendUSD: 1
  wallClockH: 1
steps:
  - id: gate1
    consumes: [idea.md]
    produces: [idea.md]
    gate: human-approval
    onDeny: pause
---

Body.
EOF4
expect 0 "run pauses for approval" -- ship run gated
expect_out "approval pause" "awaiting approval: gate1"
expect 0 "denied gate recorded" -- ship approve gate1 --deny --reason "not yet"
expect 2 "denied gate halts run" -- ship run gated
cd "$D"

printf '\nAC demo: %d pass, %d fail, %d skip\n' "$PASS" "$FAIL" "$SKIPP"
[ "$FAIL" = "0" ]
