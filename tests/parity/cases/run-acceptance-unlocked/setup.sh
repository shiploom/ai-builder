#!/bin/sh
set -u
. "$REPO/tests/parity/cases/_seed.sh"
seed_manifest unlockflow
mkdir -p seed/.shiploom/workflows
cat > seed/.shiploom/workflows/unlockflow.md <<'MDEOF'
---
name: unlockflow
version: 1.0.0
kind: sequential
steps:
  - id: get
    produces: [in.txt]
    gate: none
  - id: acceptance-lock
    consumes: [in.txt]
    produces: [acceptance/*.json]
    gate: verification
---

# unlock flow
MDEOF
printf 'input\n' > seed/in.txt
