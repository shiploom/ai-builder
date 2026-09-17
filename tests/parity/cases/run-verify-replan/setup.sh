#!/bin/sh
set -u
. "$REPO/tests/parity/cases/_seed.sh"
seed_manifest replanflow
mkdir -p seed/.shiploom/workflows
cat > seed/.shiploom/workflows/replanflow.md <<'MDEOF'
---
name: replanflow
version: 1.0.0
kind: sequential
steps:
  - id: get
    produces: [in.txt]
    gate: none
  - id: verify
    consumes: [in.txt]
    produces: [out.txt]
    gate: verification
    retries: 0
---

# replan flow
MDEOF
printf 'input\n' > seed/in.txt
printf 'output\n' > seed/out.txt
