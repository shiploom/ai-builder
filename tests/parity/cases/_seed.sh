#!/bin/sh
# Shared stateful-run fixture helpers (sourced by setup.sh files).
# All paths under seed/ (copied pristine per runtime by run.sh).
seed_manifest() {
  # $1 = workflow name; reads optional $2 extra python to merge (unused now)
  mkdir -p seed/.shiploom
  cat > seed/.shiploom/manifest.json <<EOF
{"artifacts": {}, "budgets": {"spendUSD": {"limit": 25.0, "used": 0.0}, "tokens": {"limit": 800000, "used": 0}}, "checkpoints": [], "coreVersion": "1.1.0", "gates": {}, "initializedAt": "2026-01-01T00:00:00Z", "retries": {}, "steps": {}, "workflow": "$1", "workflowVersion": "1.0.0"}
EOF
  printf '# seed idea\n' > seed/idea.md
}

seed_artifact() {
  # $1 = id, $2 = seed-relative path
  mkdir -p "seed/$(dirname "$2")"
  cat > "seed/$2" <<EOF
---
id: $1
kind: report
title: Seed artifact $1
status: researched
provenance:
  - type: human
    ref: "seed"
    confidence: high
    date: 2026-01-01
links:
  requires: []
  decided_by: []
  implemented_by: []
  tested_by: []
  verified_by: []
owner: human
version: 1
---

seed body
EOF
}
